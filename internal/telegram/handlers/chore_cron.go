package handlers

import (
	"context"
	"crypto/rand"
	"fmt"
	"html"
	"log/slog"
	"math/big"
	"os"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/store"
)

// processRecurringMu keeps two runs of ProcessRecurringChores from assigning
// the same occurrence twice.
var processRecurringMu sync.Mutex

// ProcessRecurringChores fetches all due recurring and scheduled chores and assigns them.
// It is called every minute by the cron scheduler, so a chore is assigned at its NextRunAt.
func (h *Handlers) ProcessRecurringChores(ctx context.Context) error {
	if !processRecurringMu.TryLock() {
		slog.Warn("[CRON] ProcessRecurringChores is already running; skipping this tick")
		return nil
	}
	defer processRecurringMu.Unlock()

	berlinLoc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		return fmt.Errorf("failed to load Europe/Berlin timezone: %v", err)
	}

	now := TimeNow().In(berlinLoc)
	dueChores, err := h.Store.GetDueRecurringChores(ctx, now)
	if err != nil {
		return fmt.Errorf("failed to get due recurring chores: %v", err)
	}

	if len(dueChores) == 0 {
		slog.Debug("[CRON] No recurring chores are due.")
		return nil
	}

	slog.Info(fmt.Sprintf("[CRON] Found %d due recurring chores.", len(dueChores)))

	for _, chore := range dueChores {
		// Past its end without having run (e.g. the bot was down): just retire it
		if chore.EndsAt != nil && chore.NextRunAt.After(*chore.EndsAt) {
			h.finishRecurringChore(ctx, chore)
			continue
		}

		slog.Info(fmt.Sprintf("[CRON] Processing recurring chore %d: %s", chore.ID, chore.Description))

		err := h.assignRecurringChore(ctx, chore)
		if err != nil {
			slog.Error(fmt.Sprintf("[CRON] ERROR: Failed to assign recurring chore %d: %v", chore.ID, err))
			// Do not update nextRunAt so it retries next time
			continue
		}

		// A scheduled one-off chore runs only once
		if chore.Interval <= 0 {
			h.finishRecurringChore(ctx, chore)
			continue
		}

		// Calculate new nextRunAt (N days from the CURRENT nextRunAt, not from time.Now)
		// Convert NextRunAt to Europe/Berlin so that AddDate respects daylight saving time.
		newNextRun := chore.NextRunAt.In(berlinLoc).AddDate(0, 0, chore.Interval)

		// If newNextRun is still in the past (e.g., bot was down for a long time), fast-forward it
		for newNextRun.Before(now) {
			newNextRun = newNextRun.AddDate(0, 0, chore.Interval)
		}

		if chore.EndsAt != nil && newNextRun.After(*chore.EndsAt) {
			h.finishRecurringChore(ctx, chore)
			continue
		}

		if err := h.Store.UpdateRecurringChoreNextRun(ctx, chore.ID, newNextRun); err != nil {
			slog.Error(fmt.Sprintf("[CRON] ERROR: Failed to update next_run_at for chore %d: %v", chore.ID, err))
		} else {
			slog.Info(fmt.Sprintf("[CRON] Successfully scheduled recurring chore %d for next run at %s", chore.ID, newNextRun.Format("2006-01-02 15:04 MST")))
		}
	}

	return nil
}

// finishRecurringChore deactivates a chore that has no runs left.
func (h *Handlers) finishRecurringChore(ctx context.Context, chore *store.RecurringChore) {
	if err := h.Store.CancelRecurringChore(ctx, chore.ID); err != nil {
		slog.Error(fmt.Sprintf("[CRON] ERROR: Failed to deactivate finished recurring chore %d: %v", chore.ID, err))
		return
	}
	slog.Info(fmt.Sprintf("[CRON] Recurring chore %d has no runs left and was deactivated", chore.ID))
}

// assignRecurringChore is similar to assignChore but doesn't reply to a specific user
// It just sends notifications to the group and the assigned user.
func (h *Handlers) assignRecurringChore(ctx context.Context, chore *store.RecurringChore) error {
	// 1. Get candidates
	users, err := h.Store.ListActiveUsers(ctx)
	if err != nil {
		return fmt.Errorf("failed to retrieve user list: %v", err)
	}
	if len(users) == 0 {
		return fmt.Errorf("no active users found to assign the chore to")
	}

	// A fixed assignee gets every occurrence (an explicit admin choice skips the
	// off-duty filter, like one-off chores); if they're no longer active, fall back to random.
	var selectedUser *store.User
	if chore.AssigneeUserID != 0 {
		for _, u := range users {
			if u.ID == chore.AssigneeUserID {
				selectedUser = u
				break
			}
		}
		if selectedUser == nil {
			slog.Warn(fmt.Sprintf("[CHORE-CRON] Assignee %d of recurring chore %d is not an active user; picking a random user", chore.AssigneeUserID, chore.ID))
		}
	}
	if selectedUser == nil {
		selectedUser, err = h.pickRandomRecurringChoreUser(ctx, users)
		if err != nil {
			return err
		}
	}

	// 5. Save chore to database to be visible in web UI and loaded on restart.
	// This comes before the announcement: a failed save is retried on the next
	// tick, and the group must not get the announcement again each time.
	tz := os.Getenv("CHORE_TIMEZONE")
	if tz == "" {
		tz = "Europe/Berlin"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.Local
	}
	nowLocal := time.Now().In(loc)
	deadline := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(), 23, 59, 59, 0, loc)

	reminderID := GenerateReminderID(selectedUser.TelegramUserID, time.Now())

	dbChore := &store.Chore{
		UserID:      selectedUser.ID,
		Description: chore.Description,
		AssignedAt:  time.Now(),
		DeadlineAt:  deadline,
		ReminderID:  reminderID,
	}
	if err := h.Store.CreateChore(ctx, dbChore); err != nil {
		return fmt.Errorf("failed to save recurring chore to database: %v", err)
	}

	// 6. Notifications
	escapedName := html.EscapeString(selectedUser.FirstName)
	escapedDesc := html.EscapeString(chore.Description)

	// Construct group announcement
	groupText := fmt.Sprintf("🎯 <b>%s</b>: <i>%s</i>", escapedName, escapedDesc)

	// Handle group announcement
	if h.GroupID != 0 && h.Bot != nil {
		groupMsg := tgbotapi.NewMessage(h.GroupID, groupText)
		groupMsg.ParseMode = tgbotapi.ModeHTML
		if _, err := h.Bot.Send(groupMsg); err != nil {
			slog.Error(fmt.Sprintf("Failed to send recurring chore announcement to group %d: %v", h.GroupID, err))
		} else {
			slog.Info("Announced recurring chore in group.")
		}
	} else if h.GroupID == 0 {
		slog.Info("No group configured to announce recurring chore.")
	} else if h.Bot == nil {
		slog.Info("Bot API not available for group announcement.")
	}

	// 7. Send DM to assigned user and schedule reminder (Best Effort)
	if h.ChoreReminderManager == nil {
		slog.Warn(fmt.Sprintf("Warning: DM reminders are disabled (bot API is not configured), skipping DM for %s", selectedUser.FirstName))
		return nil
	}

	if selectedUser.TelegramUserID == 0 {
		slog.Warn(fmt.Sprintf("Warning: couldn't send DM: user %s is not registered in the bot yet", selectedUser.FirstName))
		return nil
	}

	assignment := &ChoreAssignment{
		UserID:      selectedUser.TelegramUserID,
		UserName:    selectedUser.FirstName,
		Description: chore.Description,
		AssignedAt:  time.Now(),
		GroupID:     h.GroupID,
		ReminderID:  reminderID,
	}

	if err := h.ChoreReminderManager.SendInitialDM(assignment); err != nil {
		slog.Warn(fmt.Sprintf("Warning: failed to send initial DM to user %s: %v", selectedUser.FirstName, err))
		// We still consider the assignment successful even if the DM fails
	}

	return nil
}

// pickRandomRecurringChoreUser does a weighted random pick among active users who aren't off-duty.
func (h *Handlers) pickRandomRecurringChoreUser(ctx context.Context, users []*store.User) (*store.User, error) {
	// Filter candidates (exclude those on off-duty)
	var candidates []*store.User
	now := time.Now()
	checkDate := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, now.Location())

	offDutyUsers, err := h.Store.GetOffDutyUsers(ctx, checkDate)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve off-duty users: %v", err)
	}
	offDutyMap := make(map[int64]bool)
	for _, u := range offDutyUsers {
		offDutyMap[u.ID] = true
	}

	for _, u := range users {
		if !offDutyMap[u.ID] {
			candidates = append(candidates, u)
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("all active users are currently off-duty")
	}

	// 4. Weighted random assignment
	type weightedUser struct {
		user   *store.User
		weight float64
	}

	var weightedCandidates []weightedUser
	var totalWeight float64

	for _, u := range candidates {
		weight := 1.0
		if u.AdminQueueDays > 0 {
			weight += float64(u.AdminQueueDays) * 0.02
		}
		weightedCandidates = append(weightedCandidates, weightedUser{user: u, weight: weight})
		totalWeight += weight
	}

	// Select user
	maxWeightInt := int64(totalWeight * 1000)
	if maxWeightInt <= 0 {
		maxWeightInt = 1
	}

	randomBigInt, err := rand.Int(rand.Reader, big.NewInt(maxWeightInt))
	var target float64
	if err != nil {
		slog.Error(fmt.Sprintf("[CHORE-CRON] Error generating random number: %v", err))
		target = 0
	} else {
		target = float64(randomBigInt.Int64()) / 1000.0
	}

	var selectedUser *store.User
	currentWeight := 0.0
	for _, wu := range weightedCandidates {
		currentWeight += wu.weight
		if target < currentWeight {
			selectedUser = wu.user
			break
		}
	}

	if selectedUser == nil && len(candidates) > 0 {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(candidates))))
		if err != nil {
			selectedUser = candidates[0]
		} else {
			selectedUser = candidates[idx.Int64()]
		}
	}

	if selectedUser == nil {
		return nil, fmt.Errorf("failed to select a user")
	}

	return selectedUser, nil
}

// recurrenceLabel describes how often a recurring chore runs.
func recurrenceLabel(rc *store.RecurringChore) string {
	if rc.Interval <= 0 {
		return "once (scheduled)"
	}
	return fmt.Sprintf("every %d days", rc.Interval)
}
