package handlers

import (
	"context"
	"crypto/rand"
	"fmt"
	"html"
	"log/slog"
	"math/big"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/korjavin/dutyassistant/internal/telegram/keyboard"
)

// HandleChore handles the /chore command.
// For non-admins: Shows their active chores.
// For admins: Format: /chore [description] [/<N>d]
// It assigns a random active user to the described chore.
// If the /<N>d suffix is provided, it sets up a recurring chore.
// If no description is provided, it enters interactive mode.
func (h *Handlers) HandleChore(m *tgbotapi.Message) (tgbotapi.Chattable, error) {
	// 1. Admin check
	isAdmin, err := h.checkAdmin(m.From.ID)

	if err != nil || !isAdmin {
		// Non-admin path: list personal active chores
		user, err := h.Store.GetUserByTelegramID(context.Background(), m.From.ID)
		if err != nil || user == nil {
			msg := tgbotapi.NewMessage(m.Chat.ID, "Could not find your user profile. Please use /start first.")
			return msg, nil
		}

		chores, err := h.Store.GetActiveChoresByUserID(context.Background(), user.ID)
		if err != nil {
			slog.Error(fmt.Sprintf("Failed to get active chores for user %d: %v", user.ID, err))
			return tgbotapi.NewMessage(m.Chat.ID, "Sorry, something went wrong while fetching your chores."), nil
		}

		if len(chores) == 0 {
			return tgbotapi.NewMessage(m.Chat.ID, "🎉 You have no active chores right now!"), nil
		}

		tz := os.Getenv("CHORE_TIMEZONE")
		if tz == "" {
			tz = "Europe/Berlin"
		}
		loc, err := time.LoadLocation(tz)
		if err != nil {
			loc = time.Local
		}

		for _, chore := range chores {
			assignedAt := chore.AssignedAt.In(loc).Format("2006-01-02 15:04")
			text := fmt.Sprintf("📋 <b>Active Chore</b>\n\n<i>%s</i>\n\nAssigned: %s", html.EscapeString(chore.Description), assignedAt)

			msg := tgbotapi.NewMessage(m.Chat.ID, text)
			msg.ParseMode = tgbotapi.ModeHTML

			// Add inline keyboard with a single "Mark as Done" button
			keyboard := tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("✅ Mark as Done", fmt.Sprintf("chore_list_done:%d", chore.ID)),
				),
			)
			msg.ReplyMarkup = keyboard

			if h.Bot != nil {
				if _, err := h.Bot.Send(msg); err != nil {
					slog.Error(fmt.Sprintf("Failed to send chore message for chore %d: %v", chore.ID, err))
				}
			}
		}

		return nil, nil
	}

	args := strings.TrimSpace(m.CommandArguments())

	// 2. Check for interactive mode
	if args == "" {
		msg := tgbotapi.NewMessage(m.Chat.ID, "📝 <b>Chore Management</b>\n\nWhat would you like to do?")
		msg.ParseMode = tgbotapi.ModeHTML
		msg.ReplyMarkup = keyboard.ChoreMenu()
		return msg, nil
	}

	// 3. Check for translate subcommand: /chore translate <id>
	// Be specific to avoid blocking chore descriptions starting with "translate"
	lowerArgs := strings.ToLower(args)
	if strings.HasPrefix(lowerArgs, "translate ") {
		// Try to parse the ID after "translate "
		parts := strings.SplitN(args, " ", 2)
		if len(parts) == 2 {
			trimmedID := strings.TrimSpace(parts[1])
			_, err := strconv.ParseInt(trimmedID, 10, 64)
			if err == nil {
				// Valid ID found - route to translate handler
				return h.HandleChoreTranslate(m)
			}
		}
		// If we get here, it's "translate" but not followed by a valid ID
		// Could be "translate the document" - treat as regular chore creation
	}

	// 4. Parse for recurring chore suffix /<N>d
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

		description = h.translateIfNonLatin(context.Background(), description)

		// Load configured timezone
		tz := os.Getenv("CHORE_TIMEZONE")
		if tz == "" {
			tz = "Europe/Berlin"
		}
		loc, err := time.LoadLocation(tz)
		if err != nil {
			slog.Error("Failed to load location", "tz", tz, "error", err) //nolint:gosec // G706 - structured logging is safe, tz is from env var
			loc = time.Local
		}

		now := TimeNow().In(loc)
		hour := now.Hour()

		var nextRun time.Time
		executeImmediately := false

		// Allowed assignment interval is 10:00 to 18:00
		if hour >= 10 && hour < 18 {
			// Within interval: execute immediately, schedule next run exactly N days from now at 10:00 AM
			executeImmediately = true
			targetDate := now.AddDate(0, 0, intervalDays)
			nextRun = time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), 10, 0, 0, 0, loc)
		} else if hour < 10 {
			// Before 10:00: do not execute immediately, schedule first run at 10:00 today
			executeImmediately = false
			nextRun = time.Date(now.Year(), now.Month(), now.Day(), 10, 0, 0, 0, loc)
		} else {
			// After 18:00: do not execute immediately, schedule first run at 10:00 tomorrow
			executeImmediately = false
			tomorrow := now.AddDate(0, 0, 1)
			nextRun = time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 10, 0, 0, 0, loc)
		}

		chore := &store.RecurringChore{
			Description: description,
			Interval:    intervalDays,
			NextRunAt:   nextRun,
			CreatedAt:   now,
		}

		if err := h.Store.CreateRecurringChore(context.Background(), chore); err != nil {
			slog.Error(fmt.Sprintf("Failed to create recurring chore: %v", err))
			return tgbotapi.NewMessage(m.Chat.ID, "❌ Failed to create recurring chore."), nil
		}

		var msgConfig tgbotapi.MessageConfig
		if executeImmediately {
			// Execute the first assignment immediately
			var err error
			msgConfig, err = h.assignChore(m.Chat.ID, m.From.ID, description)
			if err != nil {
				slog.Error(fmt.Sprintf("Failed to assign initial recurring chore: %v", err))
				return tgbotapi.NewMessage(m.Chat.ID, "❌ Recurring chore created, but failed to assign immediately."), nil
			}
		} else {
			// No immediate execution
			msgConfig = tgbotapi.NewMessage(m.Chat.ID, "")
			msgConfig.ParseMode = tgbotapi.ModeHTML
		}

		nextRunStr := nextRun.Format("2006-01-02 15:04 MST")

		recurringInfo := fmt.Sprintf("✅ <b>Recurring chore scheduled!</b>\n\n"+
			"<b>ID:</b> <code>%d</code>\n"+
			"<b>Description:</b> <i>%s</i>\n"+
			"<b>Interval:</b> every %d days\n"+
			"<b>Next Run:</b> %s",
			chore.ID, html.EscapeString(description), chore.Interval, nextRunStr)

		if executeImmediately {
			msgConfig.Text += "\n\n" + recurringInfo
		} else {
			msgConfig.Text = recurringInfo
		}

		return msgConfig, nil
	}

	// 4. One-off chore
	description := args

	description = h.translateIfNonLatin(context.Background(), description)

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

	description = h.translateIfNonLatin(context.Background(), description)

	// Perform the chore assignment
	return h.assignChore(m.Chat.ID, m.From.ID, description)
}

// assignChore performs the actual chore assignment logic
func (h *Handlers) assignChore(chatID int64, fromUserID int64, description string) (tgbotapi.MessageConfig, error) {
	// Note: Description is stored unescaped and will be escaped at display time
	// This prevents double-escaping when showing in multiple places

	candidates, msgConfig := h.getCandidatesForChore(chatID)
	if msgConfig != nil {
		return *msgConfig, nil
	}

	selectedUser, msgConfig := h.selectUserForChore(chatID, candidates)
	if msgConfig != nil {
		return *msgConfig, nil
	}

	return h.notifyAndScheduleChore(chatID, selectedUser, description), nil
}

// getCandidatesForChore fetches active users and filters out those on off-duty.
func (h *Handlers) getCandidatesForChore(chatID int64) ([]*store.User, *tgbotapi.MessageConfig) {
	// 1. Get candidates
	users, err := h.Store.ListActiveUsers(context.Background())
	if err != nil {
		msg := tgbotapi.NewMessage(chatID, "Failed to retrieve user list.")
		return nil, &msg
	}
	if len(users) == 0 {
		msg := tgbotapi.NewMessage(chatID, "No active users found to assign the chore to.")
		return nil, &msg
	}

	// Filter candidates (exclude those on off-duty)
	var candidates []*store.User
	now := time.Now()
	// Using noon to check off-duty status as it's a daily check usually
	checkDate := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, now.Location())

	offDutyUsers, err := h.Store.GetOffDutyUsers(context.Background(), checkDate)
	if err != nil {
		msg := tgbotapi.NewMessage(chatID, "Failed to retrieve off-duty users.")
		return nil, &msg
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
		msg := tgbotapi.NewMessage(chatID, "All active users are currently off-duty.")
		return nil, &msg
	}

	return candidates, nil
}

// selectUserForChore performs weighted random selection from the given candidates.
func (h *Handlers) selectUserForChore(chatID int64, candidates []*store.User) (*store.User, *tgbotapi.MessageConfig) {
	// 4. Weighted random assignment
	// Weight = 1.0 + (AdminQueueDays * 0.02)
	// Additional +2% chance for every pending admin assigned day

	type weightedUser struct {
		user   *store.User
		weight float64
	}

	var weightedCandidates []weightedUser
	var totalWeight float64

	slog.Info("[CHORE] Starting weighted selection for chore assignment")
	slog.Info(fmt.Sprintf("[CHORE] Number of candidates after filtering: %d", len(candidates)))

	for _, u := range candidates {
		// Base weight
		weight := 1.0
		// Add 2% for each AdminQueueDay
		if u.AdminQueueDays > 0 {
			weight += float64(u.AdminQueueDays) * 0.02
		}

		weightedCandidates = append(weightedCandidates, weightedUser{user: u, weight: weight})
		totalWeight += weight
		slog.Info(fmt.Sprintf("[CHORE] Candidate: %s (ID: %d) - AdminQueueDays: %d, Weight: %.3f",
			u.FirstName, u.ID, u.AdminQueueDays, weight))
	}

	slog.Info(fmt.Sprintf("[CHORE] Total weight: %.3f", totalWeight))

	// Select user using crypto/rand
	// Convert total weight to int for big.Int (multiply by 1000 for precision)
	maxWeightInt := int64(totalWeight * 1000)
	if maxWeightInt <= 0 {
		maxWeightInt = 1
	}

	randomBigInt, err := rand.Int(rand.Reader, big.NewInt(maxWeightInt))
	var target float64
	if err != nil {
		slog.Error(fmt.Sprintf("[CHORE] Error generating random number: %v", err))
		target = 0
	} else {
		target = float64(randomBigInt.Int64()) / 1000.0
	}

	slog.Info(fmt.Sprintf("[CHORE] Random target value: %.3f (0 to %.3f)", target, totalWeight))

	var selectedUser *store.User
	currentWeight := 0.0
	for i, wu := range weightedCandidates {
		currentWeight += wu.weight
		slog.Info(fmt.Sprintf("[CHORE] Step %d: Checking %s - cumulative weight: %.3f, target: %.3f",
			i+1, wu.user.FirstName, currentWeight, target))
		if target < currentWeight {
			selectedUser = wu.user
			slog.Info(fmt.Sprintf("[CHORE] ✓ Selected: %s (ID: %d)", selectedUser.FirstName, selectedUser.ID))
			break
		}
	}
	// Fallback (should not happen mathematically if totalWeight > 0)
	if selectedUser == nil && len(candidates) > 0 {
		slog.Warn("[CHORE] WARNING: Fallback selection triggered (this should not happen)")
		// Just pick randomly
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(candidates))))
		if err != nil {
			selectedUser = candidates[0]
		} else {
			selectedUser = candidates[idx.Int64()]
		}
		slog.Info(fmt.Sprintf("[CHORE] Fallback selected: %s (ID: %d)", selectedUser.FirstName, selectedUser.ID))
	}

	if selectedUser == nil {
		msg := tgbotapi.NewMessage(chatID, "Failed to select a user.")
		return nil, &msg
	}

	return selectedUser, nil
}

// notifyAndScheduleChore creates the chore, announces it, and sends DMs.
func (h *Handlers) notifyAndScheduleChore(chatID int64, selectedUser *store.User, description string) tgbotapi.MessageConfig {
	// 5. Notifications

	// Escape HTML at display time (description is stored unescaped)
	escapedName := html.EscapeString(selectedUser.FirstName)
	escapedDesc := html.EscapeString(description)

	// Message to the admin/caller
	responseMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Assigned chore to <b>%s</b>.", escapedName))
	responseMsg.ParseMode = tgbotapi.ModeHTML

	// Construct group announcement
	groupText := fmt.Sprintf("🎯 <b>%s</b>: <i>%s</i>", escapedName, escapedDesc)

	// Handle group announcement
	if h.GroupID != 0 {
		if chatID != h.GroupID {
			// Triggered from DM, need to announce in group
			if h.Bot != nil {
				groupMsg := tgbotapi.NewMessage(h.GroupID, groupText)
				groupMsg.ParseMode = tgbotapi.ModeHTML
				if _, err := h.Bot.Send(groupMsg); err != nil {
					slog.Error(fmt.Sprintf("Failed to send chore announcement to group %d: %v", h.GroupID, err))
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
		return responseMsg
	}

	if selectedUser.TelegramUserID == 0 {
		responseMsg.Text += "\n\n⚠️ Couldn't send DM: user is not registered in the bot yet. Ask them to send /start in a DM with the bot."
		return responseMsg
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

	reminderID := GenerateReminderID(selectedUser.TelegramUserID, time.Now())

	chore := &store.Chore{
		UserID:      selectedUser.ID,
		Description: description,
		AssignedAt:  time.Now(),
		DeadlineAt:  deadline,
		ReminderID:  reminderID,
	}
	if err := h.Store.CreateChore(context.Background(), chore); err != nil {
		slog.Error(fmt.Sprintf("Failed to create chore in database: %v", err))
		responseMsg.Text += "\n\n⚠️ Failed to save chore to database."
		return responseMsg
	}
	assignment := &ChoreAssignment{
		UserID:      selectedUser.TelegramUserID,
		UserName:    selectedUser.FirstName,
		Description: description, // Store unescaped
		AssignedAt:  time.Now(),
		GroupID:     h.GroupID,
		ReminderID:  reminderID,
	}

	// SendInitialDM now handles storage internally only on success
	if err := h.ChoreReminderManager.SendInitialDM(assignment); err != nil {
		slog.Error(fmt.Sprintf("Failed to send DM to user %s: %v", selectedUser.FirstName, err))
		errText := strings.ToLower(err.Error())
		if strings.Contains(errText, "forbidden") || strings.Contains(errText, "bot can't initiate conversation") {
			responseMsg.Text += "\n\n⚠️ Failed to send DM: user must start a private chat with the bot first (/start)."
		} else {
			responseMsg.Text += "\n\n⚠️ Failed to send DM to user."
		}
	} else {
		responseMsg.Text += "\n\n📨 DM sent to user with reminder scheduled."
	}

	return responseMsg
}

// HandleChoreTranslate handles the "/chore translate <id>" command.
// It translates an existing recurring chore's description if it contains non-Latin characters.
// Only admins can use this command.
func (h *Handlers) HandleChoreTranslate(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	// 1. Admin check
	isAdmin, err := h.checkAdmin(m.From.ID)
	if err != nil || !isAdmin {
		msg := tgbotapi.NewMessage(m.Chat.ID, "❌ Only admins can translate chore descriptions.")
		return msg, nil
	}

	// 2. Parse the chore ID from arguments (format: "/chore translate <id>")
	args := strings.TrimSpace(m.CommandArguments())
	// Convert to lowercase for case-insensitive parsing
	lowerArgs := strings.ToLower(args)
	// Remove "translate " prefix
	prefix := "translate "
	if !strings.HasPrefix(lowerArgs, prefix) {
		msg := tgbotapi.NewMessage(m.Chat.ID, "❌ Invalid translate command format. Usage: /chore translate <id>")
		return msg, nil
	}

	idStr := strings.TrimSpace(args[len(prefix):])
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		msg := tgbotapi.NewMessage(m.Chat.ID, "❌ Invalid chore ID. Usage: /chore translate <id>")
		return msg, nil
	}

	// 3. Fetch recurring chore
	ctx := context.Background()
	chore, err := h.Store.GetRecurringChore(ctx, id)
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to get recurring chore %d: %v", id, err))
		msg := tgbotapi.NewMessage(m.Chat.ID, "❌ Chore not found.")
		return msg, nil
	}
	if chore == nil {
		msg := tgbotapi.NewMessage(m.Chat.ID, "❌ Chore not found.")
		return msg, nil
	}
	if !chore.IsActive {
		msg := tgbotapi.NewMessage(m.Chat.ID, "❌ Recurring chore not found or is inactive.")
		return msg, nil
	}

	// 4. Translate if non-Latin
	translatedDesc := h.translateIfNonLatin(ctx, chore.Description)

	if translatedDesc == chore.Description {
		// Check if description has non-Latin characters to distinguish between
		// "already in English" and "translation failed"
		if hasNonLatinCharacters(chore.Description) && h.LLMClient != nil {
			// Translation failed (LLM client exists but error occurred)
			msg := tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("❌ Translation failed. Please check logs and try again.\n\nCurrent: <i>%s</i>",
				html.EscapeString(chore.Description)))
			msg.ParseMode = tgbotapi.ModeHTML
			return msg, nil
		} else {
			// Already in English or LLM client not configured
			msg := tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("ℹ️ Chore <b>%d</b> description is already in English or translation is disabled.\n\nCurrent: <i>%s</i>",
				chore.ID, html.EscapeString(chore.Description)))
			msg.ParseMode = tgbotapi.ModeHTML
			return msg, nil
		}
	}

	// 5. Update description
	if err := h.Store.UpdateRecurringChoreDescription(ctx, id, translatedDesc); err != nil {
		slog.Error(fmt.Sprintf("Failed to update recurring chore %d description: %v", id, err))
		msg := tgbotapi.NewMessage(m.Chat.ID, "❌ Failed to update chore description.")
		return msg, nil
	}

	// 6. Return success message
	msg := tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("✅ Chore <b>%d</b> description translated!\n\n<b>Old:</b> <i>%s</i>\n<b>New:</b> <i>%s</i>",
		chore.ID, html.EscapeString(chore.Description), html.EscapeString(translatedDesc)))
	msg.ParseMode = tgbotapi.ModeHTML
	return msg, nil
}
