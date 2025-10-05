package handlers

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/korjavin/dutyassistant/internal/store"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	adminOnlyMessage      = "Sorry, this command is for admins only."
	userNotFoundMessage   = "Could not find user: %s"
	assignSuccessMessage  = "Successfully assigned %s to duty on %s."
	assignFailureMessage  = "Failed to assign %s to duty on %s."
	modifySuccessMessage  = "Successfully modified duty for %s to be handled by %s."
	modifyFailureMessage  = "Failed to modify duty for date %s."
	toggleSuccessMessage  = "Successfully set status for %s to %s."
	toggleFailureMessage  = "Failed to update user status."
	invalidDateMessage    = "Invalid date format. Please use YYYY-MM-DD."
)

// checkAdmin is a helper function to verify if a user is an admin.
// Admin is determined by matching the Telegram user ID against the ADMIN_ID env var.
func (h *Handlers) checkAdmin(telegramUserID int64) (bool, error) {
	if h.AdminID == 0 {
		log.Printf("[checkAdmin] AdminID not configured (0), falling back to database flag for user %d", telegramUserID)
		// Fallback to database flag if AdminID is not configured
		user, err := h.Store.GetUserByTelegramID(context.Background(), telegramUserID)
		if err != nil || user == nil {
			log.Printf("[checkAdmin] User %d not found in database or error: %v", telegramUserID, err)
			return false, err
		}
		log.Printf("[checkAdmin] User %d IsAdmin flag from database: %v", telegramUserID, user.IsAdmin)
		return user.IsAdmin, nil
	}
	isAdmin := telegramUserID == h.AdminID
	log.Printf("[checkAdmin] Configured AdminID=%d, User=%d, isAdmin=%v", h.AdminID, telegramUserID, isAdmin)
	return isAdmin, nil
}

// HandleAssign handles the /assign command for admins. Format: /assign [username] [days]
func (h *Handlers) HandleAssign(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	isAdmin, err := h.checkAdmin(m.From.ID)
	if err != nil || !isAdmin {
		return tgbotapi.NewMessage(m.Chat.ID, adminOnlyMessage), nil
	}

	args := strings.Fields(m.CommandArguments())

	// If no arguments provided, show user selection buttons
	if len(args) == 0 {
		users, err := h.Store.ListActiveUsers(context.Background())
		if err != nil || len(users) == 0 {
			msg := tgbotapi.NewMessage(m.Chat.ID, "No active users found.")
			return msg, nil
		}

		// Create inline keyboard with user buttons
		var buttons [][]tgbotapi.InlineKeyboardButton
		for _, u := range users {
			row := []tgbotapi.InlineKeyboardButton{
				tgbotapi.NewInlineKeyboardButtonData(
					fmt.Sprintf("👤 %s", u.FirstName),
					fmt.Sprintf("assign_user:%d", u.ID),
				),
			}
			buttons = append(buttons, row)
		}

		keyboard := tgbotapi.NewInlineKeyboardMarkup(buttons...)
		msg := tgbotapi.NewMessage(m.Chat.ID, "📋 <b>Assign days to admin queue</b>\n\nSelect a user:")
		msg.ParseMode = tgbotapi.ModeHTML
		msg.ReplyMarkup = keyboard
		return msg, nil
	}

	// If only username provided, prompt for days
	if len(args) == 1 {
		msg := tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("How many days should I assign to %s?\n\nExample: <code>/assign %s 3</code>", args[0], args[0]))
		msg.ParseMode = tgbotapi.ModeHTML
		return msg, nil
	}

	userName := args[0]
	var days int
	_, err = fmt.Sscanf(args[1], "%d", &days)
	if err != nil || days <= 0 {
		msg := tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("⚠️ '%s' is not a valid number of days.\n\nPlease use a positive number.\n\nExample: <code>/assign %s 3</code>", args[1], userName))
		msg.ParseMode = tgbotapi.ModeHTML
		return msg, nil
	}

	user, err := h.Store.GetUserByName(context.Background(), userName)
	if err != nil || user == nil {
		// Get list of users for suggestion
		users, _ := h.Store.ListActiveUsers(context.Background())
		suggestions := ""
		if len(users) > 0 {
			suggestions = "\n\nAvailable users:\n"
			for _, u := range users {
				suggestions += fmt.Sprintf("  • %s\n", u.FirstName)
			}
		}
		return tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("❌ User '%s' not found.%s", userName, suggestions)), nil
	}

	if err := h.Scheduler.AssignDuty(context.Background(), user, days); err != nil {
		return tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("❌ Failed to assign %d days to %s: %v", days, userName, err)), nil
	}

	return tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("✅ Successfully added %d day(s) to admin queue for %s.", days, userName)), nil
}

// HandleModify handles the /modify command. Format: /modify <date> <new_username>
// This changes the assigned user for today or a future date.
func (h *Handlers) HandleModify(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	isAdmin, err := h.checkAdmin(m.From.ID)
	if err != nil || !isAdmin {
		return tgbotapi.NewMessage(m.Chat.ID, adminOnlyMessage), nil
	}

	args := strings.Fields(m.CommandArguments())
	if len(args) != 2 {
		return tgbotapi.NewMessage(m.Chat.ID, "Invalid command format. Use /modify <YYYY-MM-DD> <new_username>"), nil
	}

	dateStr, userName := args[0], args[1]
	dutyDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return tgbotapi.NewMessage(m.Chat.ID, invalidDateMessage), nil
	}

	user, err := h.Store.GetUserByName(context.Background(), userName)
	if err != nil || user == nil {
		return tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf(userNotFoundMessage, userName)), nil
	}

	if _, err := h.Scheduler.ChangeDutyUser(context.Background(), dutyDate, user.ID); err != nil {
		return tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("Failed to change duty for %s: %v", dateStr, err)), nil
	}

	return tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf(modifySuccessMessage, dateStr, userName)), nil
}

// HandleUsers lists all users with their status.
func (h *Handlers) HandleUsers(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	isAdmin, err := h.checkAdmin(m.From.ID)
	if err != nil || !isAdmin {
		return tgbotapi.NewMessage(m.Chat.ID, adminOnlyMessage), nil
	}

	users, err := h.Store.ListAllUsers(context.Background())
	if err != nil {
		return tgbotapi.NewMessage(m.Chat.ID, "Failed to retrieve user list."), nil
	}

	if len(users) == 0 {
		return tgbotapi.NewMessage(m.Chat.ID, "No users found in the system."), nil
	}

	var builder strings.Builder
	builder.WriteString("<b>📋 User List</b>\n\n")
	for _, u := range users {
		status := "✅ Active"
		if !u.IsActive {
			status = "❌ Inactive"
		}
		adminStatus := ""
		if u.IsAdmin {
			adminStatus = " 👑"
		}

		builder.WriteString(fmt.Sprintf("<b>%s</b>%s: %s\n", u.FirstName, adminStatus, status))

		// Show queues if any
		if u.VolunteerQueueDays > 0 || u.AdminQueueDays > 0 {
			builder.WriteString(fmt.Sprintf("  Queues: V:%d A:%d\n", u.VolunteerQueueDays, u.AdminQueueDays))
		}

		// Show off-duty if set
		if u.OffDutyStart != nil && u.OffDutyEnd != nil {
			builder.WriteString(fmt.Sprintf("  🏖 Off-duty: %s to %s\n",
				u.OffDutyStart.Format("2006-01-02"),
				u.OffDutyEnd.Format("2006-01-02")))
		}
		builder.WriteString("\n")
	}

	msg := tgbotapi.NewMessage(m.Chat.ID, builder.String())
	msg.ParseMode = tgbotapi.ModeHTML
	return msg, nil
}

// HandleToggleActive toggles a user's participation in the rotation. Format: /toggle_active <username>
func (h *Handlers) HandleToggleActive(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	isAdmin, err := h.checkAdmin(m.From.ID)
	if err != nil || !isAdmin {
		return tgbotapi.NewMessage(m.Chat.ID, adminOnlyMessage), nil
	}

	userName := m.CommandArguments()
	if userName == "" {
		return tgbotapi.NewMessage(m.Chat.ID, "Invalid command format. Use /toggle_active <username>"), nil
	}

	user, err := h.Store.GetUserByName(context.Background(), userName)
	if err != nil || user == nil {
		return tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf(userNotFoundMessage, userName)), nil
	}

	user.IsActive = !user.IsActive
	if err := h.Store.UpdateUser(context.Background(), user); err != nil {
		return tgbotapi.NewMessage(m.Chat.ID, toggleFailureMessage), nil
	}

	newStatus := "Active"
	if !user.IsActive {
		newStatus = "Inactive"
	}
	return tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf(toggleSuccessMessage, user.FirstName, newStatus)), nil
}

// HandleOffDuty sets a user's off-duty period. Format: /offduty [username] [start_date] [end_date]
func (h *Handlers) HandleOffDuty(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	isAdmin, err := h.checkAdmin(m.From.ID)
	if err != nil || !isAdmin {
		return tgbotapi.NewMessage(m.Chat.ID, adminOnlyMessage), nil
	}

	args := strings.Fields(m.CommandArguments())

	// If no arguments, show help with user list
	if len(args) == 0 {
		users, err := h.Store.ListActiveUsers(context.Background())
		if err != nil || len(users) == 0 {
			msg := tgbotapi.NewMessage(m.Chat.ID,
				"🏖 <b>Set off-duty period</b>\n\n"+
				"Usage: <code>/offduty username start end</code>\n\n"+
				"Dates in format: YYYY-MM-DD\n\n"+
				"Example: <code>/offduty John 2025-10-10 2025-10-15</code>")
			msg.ParseMode = tgbotapi.ModeHTML
			return msg, nil
		}

		var builder strings.Builder
		builder.WriteString("🏖 <b>Set off-duty period</b>\n\n")
		builder.WriteString("Usage: <code>/offduty username start end</code>\n\n")
		builder.WriteString("Available users:\n")
		for _, u := range users {
			builder.WriteString(fmt.Sprintf("  • %s\n", u.FirstName))
		}
		builder.WriteString(fmt.Sprintf("\nExample: <code>/offduty %s 2025-10-10 2025-10-15</code>", users[0].FirstName))

		msg := tgbotapi.NewMessage(m.Chat.ID, builder.String())
		msg.ParseMode = tgbotapi.ModeHTML
		return msg, nil
	}

	if len(args) == 1 {
		msg := tgbotapi.NewMessage(m.Chat.ID,
			fmt.Sprintf("📅 When should %s's off-duty period start and end?\n\n"+
			"Usage: <code>/offduty %s start end</code>\n\n"+
			"Example: <code>/offduty %s 2025-10-10 2025-10-15</code>",
			args[0], args[0], args[0]))
		msg.ParseMode = tgbotapi.ModeHTML
		return msg, nil
	}

	if len(args) == 2 {
		msg := tgbotapi.NewMessage(m.Chat.ID,
			fmt.Sprintf("📅 When should %s's off-duty period end?\n\n"+
			"Usage: <code>/offduty %s %s end_date</code>\n\n"+
			"Example: <code>/offduty %s %s 2025-10-15</code>",
			args[0], args[0], args[1], args[0], args[1]))
		msg.ParseMode = tgbotapi.ModeHTML
		return msg, nil
	}

	userName := args[0]
	startDate, err := time.Parse("2006-01-02", args[1])
	if err != nil {
		msg := tgbotapi.NewMessage(m.Chat.ID,
			fmt.Sprintf("⚠️ Invalid start date '%s'\n\n"+
			"Please use format: YYYY-MM-DD\n\n"+
			"Example: <code>/offduty %s 2025-10-10 2025-10-15</code>",
			args[1], userName))
		msg.ParseMode = tgbotapi.ModeHTML
		return msg, nil
	}

	endDate, err := time.Parse("2006-01-02", args[2])
	if err != nil {
		msg := tgbotapi.NewMessage(m.Chat.ID,
			fmt.Sprintf("⚠️ Invalid end date '%s'\n\n"+
			"Please use format: YYYY-MM-DD\n\n"+
			"Example: <code>/offduty %s %s 2025-10-15</code>",
			args[2], userName, args[1]))
		msg.ParseMode = tgbotapi.ModeHTML
		return msg, nil
	}

	user, err := h.Store.GetUserByName(context.Background(), userName)
	if err != nil || user == nil {
		users, _ := h.Store.ListActiveUsers(context.Background())
		suggestions := ""
		if len(users) > 0 {
			suggestions = "\n\nAvailable users:\n"
			for _, u := range users {
				suggestions += fmt.Sprintf("  • %s\n", u.FirstName)
			}
		}
		return tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("❌ User '%s' not found.%s", userName, suggestions)), nil
	}

	if err := h.Scheduler.SetOffDuty(context.Background(), user.ID, startDate, endDate); err != nil {
		return tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("❌ Failed to set off-duty period: %v", err)), nil
	}

	return tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("✅ %s is now off-duty from %s to %s.", userName, args[1], args[2])), nil
}

// HandleChange changes the assigned user for today or a future date. Format: /change <date> <username>
// This is an alias for /modify
func (h *Handlers) HandleChange(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	return h.HandleModify(m)
}

// HandleAssignUserCallback handles the callback when a user is selected from inline keyboard
func (h *Handlers) HandleAssignUserCallback(q *tgbotapi.CallbackQuery) (tgbotapi.EditMessageTextConfig, error) {
	parts := strings.Split(q.Data, ":")
	if len(parts) != 2 {
		return tgbotapi.EditMessageTextConfig{}, fmt.Errorf("invalid callback data")
	}

	userID := parts[1]

	// Get user info
	var id int64
	fmt.Sscanf(userID, "%d", &id)
	user, err := h.Store.GetUserByTelegramID(context.Background(), id)
	if err != nil || user == nil {
		// Try by ID directly
		users, _ := h.Store.ListAllUsers(context.Background())
		for _, u := range users {
			if u.ID == id {
				user = u
				break
			}
		}
	}

	if user == nil {
		edit := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, "❌ User not found")
		return edit, nil
	}

	// Create number selection keyboard (1-7 days)
	var buttons [][]tgbotapi.InlineKeyboardButton
	row := []tgbotapi.InlineKeyboardButton{}
	for days := 1; days <= 7; days++ {
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("%d", days),
			fmt.Sprintf("assign_days:%d:%d", user.ID, days),
		))
		if days%4 == 0 || days == 7 {
			buttons = append(buttons, row)
			row = []tgbotapi.InlineKeyboardButton{}
		}
	}
	// Add custom option
	buttons = append(buttons, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("✏️ Custom", fmt.Sprintf("assign_custom:%d", user.ID)),
	})

	keyboard := tgbotapi.NewInlineKeyboardMarkup(buttons...)
	edit := tgbotapi.NewEditMessageText(
		q.Message.Chat.ID,
		q.Message.MessageID,
		fmt.Sprintf("👤 <b>%s</b>\n\nHow many days to assign?", user.FirstName),
	)
	edit.ParseMode = tgbotapi.ModeHTML
	edit.ReplyMarkup = &keyboard
	return edit, nil
}

// HandleAssignDaysCallback handles the final confirmation when days are selected
func (h *Handlers) HandleAssignDaysCallback(q *tgbotapi.CallbackQuery) (tgbotapi.EditMessageTextConfig, error) {
	parts := strings.Split(q.Data, ":")
	if len(parts) != 3 {
		return tgbotapi.EditMessageTextConfig{}, fmt.Errorf("invalid callback data")
	}

	var userID, days int64
	fmt.Sscanf(parts[1], "%d", &userID)
	fmt.Sscanf(parts[2], "%d", &days)

	// Get user
	users, _ := h.Store.ListAllUsers(context.Background())
	var user *store.User
	for _, u := range users {
		if u.ID == userID {
			user = u
			break
		}
	}

	if user == nil {
		edit := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, "❌ User not found")
		return edit, nil
	}

	// Assign the days
	err := h.Scheduler.AssignDuty(context.Background(), user, int(days))
	if err != nil {
		edit := tgbotapi.NewEditMessageText(
			q.Message.Chat.ID,
			q.Message.MessageID,
			fmt.Sprintf("❌ Failed to assign: %v", err),
		)
		return edit, nil
	}

	edit := tgbotapi.NewEditMessageText(
		q.Message.Chat.ID,
		q.Message.MessageID,
		fmt.Sprintf("✅ Added %d day(s) to admin queue for <b>%s</b>", days, user.FirstName),
	)
	edit.ParseMode = tgbotapi.ModeHTML
	return edit, nil
}

// HandleAssignCustomCallback handles custom day input request
func (h *Handlers) HandleAssignCustomCallback(q *tgbotapi.CallbackQuery) (tgbotapi.EditMessageTextConfig, error) {
	parts := strings.Split(q.Data, ":")
	if len(parts) != 2 {
		return tgbotapi.EditMessageTextConfig{}, fmt.Errorf("invalid callback data")
	}

	var userID int64
	fmt.Sscanf(parts[1], "%d", &userID)

	// Get user
	users, _ := h.Store.ListAllUsers(context.Background())
	var user *store.User
	for _, u := range users {
		if u.ID == userID {
			user = u
			break
		}
	}

	userName := "user"
	if user != nil {
		userName = user.FirstName
	}

	edit := tgbotapi.NewEditMessageText(
		q.Message.Chat.ID,
		q.Message.MessageID,
		fmt.Sprintf("👤 <b>%s</b>\n\nPlease type the number of days:\n\n<code>/assign %s [days]</code>", userName, userName),
	)
	edit.ParseMode = tgbotapi.ModeHTML
	return edit, nil
}