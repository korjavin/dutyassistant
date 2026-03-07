package handlers

import (
	"context"
	"fmt"
	"html"
	"log"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"
	"os"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/store"
)

// HandleChore handles the /chore command for admins. Format: /chore [description] [/<N>d]
// It assigns a random active user to the described chore.
// If the /<N>d suffix is provided, it sets up a recurring chore.
// If no description is provided, it enters interactive mode.
func (h *Handlers) HandleChore(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	// 1. Admin check
	isAdmin, err := h.checkAdmin(m.From.ID)
	if err != nil || !isAdmin {
		return tgbotapi.NewMessage(m.Chat.ID, adminOnlyMessage), nil
	}

	args := strings.TrimSpace(m.CommandArguments())

	// 2. Check for interactive mode
	if args == "" {
		// Enter interactive mode
		h.SessionManager.StartSession(m.Chat.ID, m.From.ID, SessionTypeChoreCreation)
		msg := tgbotapi.NewMessage(m.Chat.ID, "📝 <b>Interactive Chore Mode</b>\n\nWhat chore do you want to create?\n\nJust send me the description in your next message.")
		msg.ParseMode = tgbotapi.ModeHTML
		return msg, nil
	}

	// 3. Parse for recurring chore suffix /<N>d
	recurringRe := regexp.MustCompile(`(?i)\s+/([1-9][0-9]*)d$`)
	match := recurringRe.FindStringSubmatch(args)

	if len(match) > 1 {
		// It's a recurring chore
		intervalDays, err := strconv.Atoi(match[1])
		if err != nil || intervalDays < 1 {
			return tgbotapi.NewMessage(m.Chat.ID, "❌ Invalid interval for recurring chore. Must be a positive integer, e.g., /3d."), nil
		}

		// Remove the suffix from the description
		description := strings.TrimSpace(args[:len(args)-len(match[0])])
		if description == "" {
			return tgbotapi.NewMessage(m.Chat.ID, "❌ Chore description cannot be empty."), nil
		}

		// Calculate next run at 11:00 AM Europe/Berlin today or future
		berlinLoc, err := time.LoadLocation("Europe/Berlin")
		if err != nil {
			log.Printf("Failed to load Europe/Berlin location: %v", err)
			return tgbotapi.NewMessage(m.Chat.ID, "❌ Internal error determining timezone."), nil
		}

		now := time.Now().In(berlinLoc)
		// Check if it's already past 11:00 AM today
		nextRun := time.Date(now.Year(), now.Month(), now.Day(), 11, 0, 0, 0, berlinLoc)
		if now.After(nextRun) {
			// If it's past 11:00 AM, the first run shouldn't be missed immediately.
			// Let's schedule it for tomorrow + (interval - 1) days
			nextRun = nextRun.AddDate(0, 0, intervalDays)
		} else {
			// Wait until 11 AM today or interval? If they create it before 11, let's run it today,
			// or actually interval from now makes more sense.
			// Task requirement: N days from now.
			nextRun = nextRun.AddDate(0, 0, intervalDays)
		}

		chore := &store.RecurringChore{
			Description: description,
			Interval:    intervalDays,
			NextRunAt:   nextRun,
			CreatedAt:   now,
		}

		if err := h.Store.CreateRecurringChore(context.Background(), chore); err != nil {
			log.Printf("Failed to create recurring chore: %v", err)
			return tgbotapi.NewMessage(m.Chat.ID, "❌ Failed to create recurring chore."), nil
		}

		escapedDesc := html.EscapeString(description)
		nextRunStr := nextRun.Format("2006-01-02 15:04 MST")

		msgText := fmt.Sprintf("✅ <b>Recurring chore created!</b>\n\n"+
			"<b>ID:</b> <code>%d</code>\n"+
			"<b>Description:</b> <i>%s</i>\n"+
			"<b>Interval:</b> every %d days\n"+
			"<b>Next Run:</b> %s",
			chore.ID, escapedDesc, chore.Interval, nextRunStr)

		msg := tgbotapi.NewMessage(m.Chat.ID, msgText)
		msg.ParseMode = tgbotapi.ModeHTML
		return msg, nil
	}

	// 4. One-off chore
	description := args

	// Perform the actual chore assignment
	return h.assignChore(m.Chat.ID, m.From.ID, description)
}

// HandleChoreInteractive handles messages in interactive chore creation mode
// Returns nil if the message should be ignored (not from session creator)
func (h *Handlers) HandleChoreInteractive(m *tgbotapi.Message) (tgbotapi.Chattable, error) {
	session, exists := h.SessionManager.GetSession(m.Chat.ID)
	if !exists || session.Type != SessionTypeChoreCreation {
		// Not in chore creation mode, ignore completely
		return nil, nil
	}

	// CRITICAL: Only accept messages from the SAME user who started the session
	// This prevents other users (in groups) from interfering with the interactive flow
	if session.UserID != m.From.ID {
		// Different user, ignore completely (don't send any response)
		return nil, nil
	}

	// End the session
	h.SessionManager.EndSession(m.Chat.ID)

	// Use the message text as the chore description
	description := strings.TrimSpace(m.Text)
	if description == "" {
		msg := tgbotapi.NewMessage(m.Chat.ID, "❌ Chore description cannot be empty. Please use <code>/chore</code> to try again.")
		msg.ParseMode = tgbotapi.ModeHTML
		return msg, nil
	}

	// Perform the chore assignment
	return h.assignChore(m.Chat.ID, m.From.ID, description)
}

// assignChore performs the actual chore assignment logic
func (h *Handlers) assignChore(chatID int64, fromUserID int64, description string) (tgbotapi.MessageConfig, error) {
	// Note: Description is stored unescaped and will be escaped at display time
	// This prevents double-escaping when showing in multiple places

	// 1. Get candidates
	users, err := h.Store.ListActiveUsers(context.Background())
	if err != nil {
		return tgbotapi.NewMessage(chatID, "Failed to retrieve user list."), nil
	}
	if len(users) == 0 {
		return tgbotapi.NewMessage(chatID, "No active users found to assign the chore to."), nil
	}

	// Filter candidates (exclude those on off-duty)
	var candidates []*store.User
	now := time.Now()
	// Using noon to check off-duty status as it's a daily check usually
	checkDate := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, now.Location())

	offDutyUsers, err := h.Store.GetOffDutyUsers(context.Background(), checkDate)
	if err != nil {
		return tgbotapi.NewMessage(chatID, "Failed to retrieve off-duty users."), nil
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
		return tgbotapi.NewMessage(chatID, "All active users are currently off-duty."), nil
	}

	// 4. Weighted random assignment
	// Weight = 1.0 + (AdminQueueDays * 0.02)
	// Additional +2% chance for every pending admin assigned day

	type weightedUser struct {
		user   *store.User
		weight float64
	}

	var weightedCandidates []weightedUser
	var totalWeight float64

	log.Printf("[CHORE] Starting weighted selection for chore assignment")
	log.Printf("[CHORE] Number of candidates after filtering: %d", len(candidates))

	for _, u := range candidates {
		// Base weight
		weight := 1.0
		// Add 2% for each AdminQueueDay
		if u.AdminQueueDays > 0 {
			weight += float64(u.AdminQueueDays) * 0.02
		}

		weightedCandidates = append(weightedCandidates, weightedUser{user: u, weight: weight})
		totalWeight += weight
		log.Printf("[CHORE] Candidate: %s (ID: %d) - AdminQueueDays: %d, Weight: %.3f", 
			u.FirstName, u.ID, u.AdminQueueDays, weight)
	}

	log.Printf("[CHORE] Total weight: %.3f", totalWeight)

	// Select user
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	target := r.Float64() * totalWeight
	log.Printf("[CHORE] Random target value: %.3f (0 to %.3f)", target, totalWeight)

	var selectedUser *store.User
	currentWeight := 0.0
	for i, wu := range weightedCandidates {
		currentWeight += wu.weight
		log.Printf("[CHORE] Step %d: Checking %s - cumulative weight: %.3f, target: %.3f", 
			i+1, wu.user.FirstName, currentWeight, target)
		if target < currentWeight {
			selectedUser = wu.user
			log.Printf("[CHORE] ✓ Selected: %s (ID: %d)", selectedUser.FirstName, selectedUser.ID)
			break
		}
	}
	// Fallback (should not happen mathematically if totalWeight > 0)
	if selectedUser == nil && len(candidates) > 0 {
		log.Printf("[CHORE] WARNING: Fallback selection triggered (this should not happen)")
		// Just pick randomly
		selectedUser = candidates[r.Intn(len(candidates))]
		log.Printf("[CHORE] Fallback selected: %s (ID: %d)", selectedUser.FirstName, selectedUser.ID)
	}

	if selectedUser == nil {
		return tgbotapi.NewMessage(chatID, "Failed to select a user."), nil
	}

	// 5. Notifications

	// Escape HTML at display time (description is stored unescaped)
	escapedName := html.EscapeString(selectedUser.FirstName)
	escapedDesc := html.EscapeString(description)

	// Message to the admin/caller
	responseMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Assigned chore to <b>%s</b>.", escapedName))
	responseMsg.ParseMode = tgbotapi.ModeHTML

	// Construct group announcement
	groupText := fmt.Sprintf("🎲 <b>Random Chore Assignment</b>\n\n🎯 <b>%s</b> has been assigned a chore:\n\n<i>%s</i>", escapedName, escapedDesc)

	// Handle group announcement
	if h.GroupID != 0 {
		if chatID != h.GroupID {
			// Triggered from DM, need to announce in group
			if h.Bot != nil {
				groupMsg := tgbotapi.NewMessage(h.GroupID, groupText)
				groupMsg.ParseMode = tgbotapi.ModeHTML
				if _, err := h.Bot.Send(groupMsg); err != nil {
					log.Printf("Failed to send chore announcement to group %d: %v", h.GroupID, err)
					responseMsg.Text += "\n\n⚠️ Failed to announce in group."
				} else {
					responseMsg.Text += "\n\n📢 Announced in group."
				}
			} else {
				responseMsg.Text += "\n\n⚠️ Bot API not available for group announcement."
			}
		} else {
			// Triggered in group, just return the announcement as response
			responseMsg.Text = groupText
		}
	} else {
		// No group configured
		responseMsg.Text = fmt.Sprintf("✅ Assigned chore to <b>%s</b>:\n<i>%s</i>", escapedName, escapedDesc)
	}

	// 6. Send DM to assigned user and schedule reminder
	if h.ChoreReminderManager == nil {
		responseMsg.Text += "\n\n⚠️ DM reminders are disabled (bot API is not configured)."
		return responseMsg, nil
	}

	if selectedUser.TelegramUserID == 0 {
		responseMsg.Text += "\n\n⚠️ Couldn't send DM: user is not registered in the bot yet. Ask them to send /start in a DM with the bot."
		return responseMsg, nil
	}

	// Create assignment with unescaped description (will be escaped at display time)
	// Define the deadline as end of the current day in Berlin timezone (or local timezone)
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

	chore := &store.Chore{
		UserID:      selectedUser.ID,
		Description: description,
		AssignedAt:  time.Now(),
		DeadlineAt:  deadline,
		ReminderID:  GenerateReminderID(selectedUser.TelegramUserID, time.Now()),
	}
	if err := h.Store.CreateChore(context.Background(), chore); err != nil {
		log.Printf("Failed to create chore in database: %v", err)
		responseMsg.Text += "\n\n⚠️ Failed to save chore to database."
		return responseMsg, nil
	}
	assignment := &ChoreAssignment{
		UserID:      selectedUser.TelegramUserID,
		UserName:    selectedUser.FirstName,
		Description: description, // Store unescaped
		AssignedAt:  time.Now(),
		GroupID:     h.GroupID,
		ReminderID:  GenerateReminderID(selectedUser.TelegramUserID, time.Now()),
	}

	// SendInitialDM now handles storage internally only on success
	if err := h.ChoreReminderManager.SendInitialDM(assignment); err != nil {
		log.Printf("Failed to send DM to user %s: %v", selectedUser.FirstName, err)
		errText := strings.ToLower(err.Error())
		if strings.Contains(errText, "forbidden") || strings.Contains(errText, "bot can't initiate conversation") {
			responseMsg.Text += "\n\n⚠️ Failed to send DM: user must start a private chat with the bot first (/start)."
		} else {
			responseMsg.Text += "\n\n⚠️ Failed to send DM to user."
		}
	} else {
		responseMsg.Text += "\n\n📨 DM sent to user with reminder scheduled."
	}

	return responseMsg, nil
}
