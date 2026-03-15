package handlers

import (
	"context"
	"crypto/rand"
	"fmt"
	"html"
	"log/slog"
	"math/big"
	"os"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/store"
)

// ProcessRecurringChores fetches all due recurring chores and assigns them.
// It should be called periodically by the cron scheduler.
func (h *Handlers) ProcessRecurringChores(ctx context.Context) error {
	slog.Info("[CRON] Starting ProcessRecurringChores")

	berlinLoc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		return fmt.Errorf("failed to load Europe/Berlin timezone: %v", err)
	}

	now := time.Now().In(berlinLoc)
	dueChores, err := h.Store.GetDueRecurringChores(ctx, now)
	if err != nil {
		return fmt.Errorf("failed to get due recurring chores: %v", err)
	}

	if len(dueChores) == 0 {
		slog.Info("[CRON] No recurring chores are due.")
		return nil
	}

	slog.Info(fmt.Sprintf("[CRON] Found %d due recurring chores.", len(dueChores)))

	for _, chore := range dueChores {
		slog.Info(fmt.Sprintf("[CRON] Processing recurring chore %d: %s", chore.ID, chore.Description))

		err := h.assignRecurringChore(ctx, chore)
		if err != nil {
			slog.Error(fmt.Sprintf("[CRON] ERROR: Failed to assign recurring chore %d: %v", chore.ID, err))
			// Do not update nextRunAt so it retries next time
			continue
		}

		// Calculate new nextRunAt (N days from the CURRENT nextRunAt, not from time.Now)
		// Convert NextRunAt to Europe/Berlin so that AddDate respects daylight saving time.
		newNextRun := chore.NextRunAt.In(berlinLoc).AddDate(0, 0, chore.Interval)

		// If newNextRun is still in the past (e.g., bot was down for a long time), fast-forward it
		for newNextRun.Before(now) {
			newNextRun = newNextRun.AddDate(0, 0, chore.Interval)
		}

		if err := h.Store.UpdateRecurringChoreNextRun(ctx, chore.ID, newNextRun); err != nil {
			slog.Error(fmt.Sprintf("[CRON] ERROR: Failed to update next_run_at for chore %d: %v", chore.ID, err))
		} else {
			slog.Info(fmt.Sprintf("[CRON] Successfully scheduled recurring chore %d for next run at %s", chore.ID, newNextRun.Format("2006-01-02 15:04 MST")))
		}
	}

	return nil
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

	// Filter candidates (exclude those on off-duty)
	var candidates []*store.User
	now := time.Now()
	checkDate := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, now.Location())

	offDutyUsers, err := h.Store.GetOffDutyUsers(ctx, checkDate)
	if err != nil {
		return fmt.Errorf("failed to retrieve off-duty users: %v", err)
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
		return fmt.Errorf("all active users are currently off-duty")
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
	bg, _ := rand.Int(rand.Reader, big.NewInt(1<<53))
	rFloat := float64(bg.Int64()) / (1 << 53)
	target := rFloat * totalWeight

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
		bg, _ := rand.Int(rand.Reader, big.NewInt(int64(len(candidates))))
		selectedUser = candidates[bg.Int64()]
	}

	if selectedUser == nil {
		return fmt.Errorf("failed to select a user")
	}

	// 5. Notifications
	escapedName := html.EscapeString(selectedUser.FirstName)
	escapedDesc := html.EscapeString(chore.Description)

	// Construct group announcement
	groupText := fmt.Sprintf("🎲 <b>Periodic Chore Assignment</b>\n\n🎯 <b>%s</b> has been assigned a chore:\n\n<i>%s</i>", escapedName, escapedDesc)

	// Handle group announcement
	if h.GroupID != 0 && h.Bot != nil {
		groupMsg := tgbotapi.NewMessage(h.GroupID, groupText)
		groupMsg.ParseMode = tgbotapi.ModeHTML
		if _, err := h.Bot.Send(groupMsg); err != nil {
			slog.Error(fmt.Sprintf("Failed to send recurring chore announcement to group %d: %v", h.GroupID, err))
		} else {
			slog.Info(fmt.Sprintf("Announced recurring chore in group."))
		}
	} else if h.GroupID == 0 {
		slog.Info(fmt.Sprintf("No group configured to announce recurring chore."))
	} else if h.Bot == nil {
		slog.Info(fmt.Sprintf("Bot API not available for group announcement."))
	}

	// 6. Save chore to database to be visible in web UI and loaded on restart
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
