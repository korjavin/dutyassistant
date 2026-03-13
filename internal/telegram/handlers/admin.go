package handlers

import (
	"context"
	"fmt"
	"html"
	"log"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/store"
)

const (
	adminOnlyMessage     = "Sorry, this command is for admins only."
	userNotFoundMessage  = "Could not find user: %s"
	assignSuccessMessage = "Successfully assigned %s to duty on %s."
	assignFailureMessage = "Failed to assign %s to duty on %s."
	modifySuccessMessage = "Successfully modified duty for %s to be handled by %s."
	modifyFailureMessage = "Failed to modify duty for date %s."
	toggleSuccessMessage = "Successfully set status for %s to %s."
	toggleFailureMessage = "Failed to update user status."
	invalidDateMessage   = "Invalid date format. Please use YYYY-MM-DD."
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

// HandleUnassign handles the /unassign command for admins. Format: /unassign [username] [days]
func (h *Handlers) HandleUnassign(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	isAdmin, err := h.checkAdmin(m.From.ID)
	if err != nil || !isAdmin {
		return tgbotapi.NewMessage(m.Chat.ID, adminOnlyMessage), nil
	}

	args := strings.Fields(m.CommandArguments())

	// If no arguments provided, show user selection buttons
	if len(args) == 0 {
		// Only list users who have admin queue > 0
		users, err := h.Store.GetUsersWithAdminQueue(context.Background())
		if err != nil || len(users) == 0 {
			msg := tgbotapi.NewMessage(m.Chat.ID, "No users found with assignments in admin queue.")
			return msg, nil
		}

		// Create inline keyboard with user buttons
		var buttons [][]tgbotapi.InlineKeyboardButton
		for _, u := range users {
			row := []tgbotapi.InlineKeyboardButton{
				tgbotapi.NewInlineKeyboardButtonData(
					fmt.Sprintf("👤 %s (%d days)", u.FirstName, u.AdminQueueDays),
					fmt.Sprintf("unassign_user:%d", u.ID),
				),
			}
			buttons = append(buttons, row)
		}

		keyboard := tgbotapi.NewInlineKeyboardMarkup(buttons...)
		msg := tgbotapi.NewMessage(m.Chat.ID, "📋 <b>Remove days from admin queue</b>\n\nSelect a user:")
		msg.ParseMode = tgbotapi.ModeHTML
		msg.ReplyMarkup = keyboard
		return msg, nil
	}

	// If only username provided, prompt for days
	if len(args) == 1 {
		msg := tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("How many days should I remove from %s?\n\nExample: <code>/unassign %s 3</code>", args[0], args[0]))
		msg.ParseMode = tgbotapi.ModeHTML
		return msg, nil
	}

	userName := args[0]
	var days int
	_, err = fmt.Sscanf(args[1], "%d", &days)
	if err != nil || days <= 0 {
		msg := tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("⚠️ '%s' is not a valid number of days.\n\nPlease use a positive number.\n\nExample: <code>/unassign %s 3</code>", args[1], userName))
		msg.ParseMode = tgbotapi.ModeHTML
		return msg, nil
	}

	user, err := h.Store.GetUserByName(context.Background(), userName)
	if err != nil || user == nil {
		return tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("❌ User '%s' not found.", userName)), nil
	}

	if err := h.Scheduler.UnassignDuty(context.Background(), user, days); err != nil {
		return tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("❌ Failed to remove %d days from %s: %v", days, userName, err)), nil
	}

	return tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("✅ Successfully removed %d day(s) from admin queue for %s.", days, userName)), nil
}

// HandleModify handles the /modify command. Format: /modify <date> <new_username>
// This changes the assigned user for today or a future date.
func (h *Handlers) HandleModify(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	isAdmin, err := h.checkAdmin(m.From.ID)
	if err != nil || !isAdmin {
		return tgbotapi.NewMessage(m.Chat.ID, adminOnlyMessage), nil
	}

	args := strings.Fields(m.CommandArguments())

	// No args - show date selection buttons (today + next 7 days)
	if len(args) == 0 {
		now := time.Now()
		var buttons [][]tgbotapi.InlineKeyboardButton

		for i := 0; i < 7; i++ {
			date := now.AddDate(0, 0, i)
			dateStr := date.Format("2006-01-02")
			label := dateStr
			if i == 0 {
				label = "📅 Today (" + dateStr + ")"
			}
			row := []tgbotapi.InlineKeyboardButton{
				tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("modify_date:%s", dateStr)),
			}
			buttons = append(buttons, row)
		}

		keyboard := tgbotapi.NewInlineKeyboardMarkup(buttons...)
		msg := tgbotapi.NewMessage(m.Chat.ID, "🔄 <b>Modify duty assignment</b>\n\nSelect the date:")
		msg.ParseMode = tgbotapi.ModeHTML
		msg.ReplyMarkup = keyboard
		return msg, nil
	}

	// One arg (date) - show user selection buttons
	if len(args) == 1 {
		dateStr := args[0]
		_, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			msg := tgbotapi.NewMessage(m.Chat.ID,
				fmt.Sprintf("⚠️ Invalid date '%s'\n\nPlease use format: YYYY-MM-DD\n\nExample: <code>/modify 2025-10-10 John</code>", dateStr))
			msg.ParseMode = tgbotapi.ModeHTML
			return msg, nil
		}

		users, err := h.Store.ListActiveUsers(context.Background())
		if err != nil || len(users) == 0 {
			return tgbotapi.NewMessage(m.Chat.ID, "No active users found."), nil
		}

		var buttons [][]tgbotapi.InlineKeyboardButton
		for _, u := range users {
			row := []tgbotapi.InlineKeyboardButton{
				tgbotapi.NewInlineKeyboardButtonData(
					fmt.Sprintf("👤 %s", u.FirstName),
					fmt.Sprintf("modify_user:%s:%d", dateStr, u.ID),
				),
			}
			buttons = append(buttons, row)
		}

		keyboard := tgbotapi.NewInlineKeyboardMarkup(buttons...)
		msg := tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("🔄 <b>Modify duty for %s</b>\n\nSelect the new user:", dateStr))
		msg.ParseMode = tgbotapi.ModeHTML
		msg.ReplyMarkup = keyboard
		return msg, nil
	}

	if len(args) != 2 {
		msg := tgbotapi.NewMessage(m.Chat.ID, "⚠️ Invalid format.\n\nUsage: <code>/modify date username</code>\n\nExample: <code>/modify 2025-10-10 John</code>")
		msg.ParseMode = tgbotapi.ModeHTML
		return msg, nil
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

	// Check vacation mode status
	isVacation, err := h.Scheduler.IsVacationMode(context.Background())
	if err != nil {
		log.Printf("Warning: could not check vacation mode: %v", err)
	}

	var builder strings.Builder
	builder.WriteString("<b>📋 User List</b>\n")
	if isVacation {
		builder.WriteString("\n🏖 <b>VACATION MODE ON</b> - No new duties will be assigned\n")
	}
	builder.WriteString("\n")
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
		users, err := h.Store.ListAllUsers(context.Background())
		if err != nil || len(users) == 0 {
			return tgbotapi.NewMessage(m.Chat.ID, "No users found."), nil
		}

		var buttons [][]tgbotapi.InlineKeyboardButton
		for _, u := range users {
			status := "✅"
			if !u.IsActive {
				status = "❌"
			}
			row := []tgbotapi.InlineKeyboardButton{
				tgbotapi.NewInlineKeyboardButtonData(
					fmt.Sprintf("%s %s", status, u.FirstName),
					fmt.Sprintf("toggle_user:%d", u.ID),
				),
			}
			buttons = append(buttons, row)
		}

		keyboard := tgbotapi.NewInlineKeyboardMarkup(buttons...)
		msg := tgbotapi.NewMessage(m.Chat.ID, "🔄 <b>Toggle user active status</b>\n\nSelect a user:")
		msg.ParseMode = tgbotapi.ModeHTML
		msg.ReplyMarkup = keyboard
		return msg, nil
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

	// If no arguments, show user selection with buttons
	if len(args) == 0 {
		users, err := h.Store.ListActiveUsers(context.Background())
		if err != nil || len(users) == 0 {
			msg := tgbotapi.NewMessage(m.Chat.ID, "No active users found.")
			return msg, nil
		}

		var buttons [][]tgbotapi.InlineKeyboardButton
		for _, u := range users {
			row := []tgbotapi.InlineKeyboardButton{
				tgbotapi.NewInlineKeyboardButtonData(
					fmt.Sprintf("👤 %s", u.FirstName),
					fmt.Sprintf("offduty_user:%d:%s", u.ID, u.FirstName),
				),
			}
			buttons = append(buttons, row)
		}

		keyboard := tgbotapi.NewInlineKeyboardMarkup(buttons...)
		msg := tgbotapi.NewMessage(m.Chat.ID, "🏖 <b>Set off-duty period</b>\n\nSelect a user:")
		msg.ParseMode = tgbotapi.ModeHTML
		msg.ReplyMarkup = keyboard
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

// HandleUnassignUserCallback handles the callback when a user is selected for unassignment
func (h *Handlers) HandleUnassignUserCallback(q *tgbotapi.CallbackQuery) (tgbotapi.EditMessageTextConfig, error) {
	parts := strings.Split(q.Data, ":")
	if len(parts) != 2 {
		return tgbotapi.EditMessageTextConfig{}, fmt.Errorf("invalid callback data")
	}

	userID := parts[1]
	var id int64
	fmt.Sscanf(userID, "%d", &id)

	// Get user info (including queue size)
	users, _ := h.Store.GetUsersWithAdminQueue(context.Background())
	var user *store.User
	for _, u := range users {
		if u.ID == id {
			user = u
			break
		}
	}

	if user == nil {
		edit := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, "❌ User not found or has no assigned days.")
		return edit, nil
	}

	// Create number selection keyboard
	var buttons [][]tgbotapi.InlineKeyboardButton
	row := []tgbotapi.InlineKeyboardButton{}

	// Add "1 day" option
	row = append(row, tgbotapi.NewInlineKeyboardButtonData("1", fmt.Sprintf("unassign_days:%d:1", user.ID)))

	// Add other options based on queue size, up to 7 or queue size
	for days := 2; days <= 7 && days < user.AdminQueueDays; days++ {
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("%d", days),
			fmt.Sprintf("unassign_days:%d:%d", user.ID, days),
		))
		if len(row) >= 4 {
			buttons = append(buttons, row)
			row = []tgbotapi.InlineKeyboardButton{}
		}
	}
	if len(row) > 0 {
		buttons = append(buttons, row)
	}

	// Add "All" option
	buttons = append(buttons, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("All (%d)", user.AdminQueueDays),
			fmt.Sprintf("unassign_days:%d:%d", user.ID, user.AdminQueueDays),
		),
	})

	keyboard := tgbotapi.NewInlineKeyboardMarkup(buttons...)
	edit := tgbotapi.NewEditMessageText(
		q.Message.Chat.ID,
		q.Message.MessageID,
		fmt.Sprintf("👤 <b>%s</b> (Queue: %d)\n\nHow many days to remove?", user.FirstName, user.AdminQueueDays),
	)
	edit.ParseMode = tgbotapi.ModeHTML
	edit.ReplyMarkup = &keyboard
	return edit, nil
}

// HandleUnassignDaysCallback handles the final confirmation when days are selected for unassignment
func (h *Handlers) HandleUnassignDaysCallback(q *tgbotapi.CallbackQuery) (tgbotapi.EditMessageTextConfig, error) {
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

	// Unassign the days
	err := h.Scheduler.UnassignDuty(context.Background(), user, int(days))
	if err != nil {
		edit := tgbotapi.NewEditMessageText(
			q.Message.Chat.ID,
			q.Message.MessageID,
			fmt.Sprintf("❌ Failed to unassign: %v", err),
		)
		return edit, nil
	}

	edit := tgbotapi.NewEditMessageText(
		q.Message.Chat.ID,
		q.Message.MessageID,
		fmt.Sprintf("✅ Removed %d day(s) from admin queue for <b>%s</b>", days, user.FirstName),
	)
	edit.ParseMode = tgbotapi.ModeHTML
	return edit, nil
}

// HandleModifyDateCallback handles date selection for modify command
func (h *Handlers) HandleModifyDateCallback(q *tgbotapi.CallbackQuery) (tgbotapi.EditMessageTextConfig, error) {
	parts := strings.Split(q.Data, ":")
	if len(parts) != 2 {
		return tgbotapi.EditMessageTextConfig{}, fmt.Errorf("invalid callback data")
	}

	dateStr := parts[1]

	users, err := h.Store.ListActiveUsers(context.Background())
	if err != nil || len(users) == 0 {
		edit := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, "❌ No active users found.")
		return edit, nil
	}

	var buttons [][]tgbotapi.InlineKeyboardButton
	for _, u := range users {
		row := []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("👤 %s", u.FirstName),
				fmt.Sprintf("modify_user:%s:%d", dateStr, u.ID),
			),
		}
		buttons = append(buttons, row)
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(buttons...)
	edit := tgbotapi.NewEditMessageText(
		q.Message.Chat.ID,
		q.Message.MessageID,
		fmt.Sprintf("🔄 <b>Modify duty for %s</b>\n\nSelect the new user:", dateStr),
	)
	edit.ParseMode = tgbotapi.ModeHTML
	edit.ReplyMarkup = &keyboard
	return edit, nil
}

// HandleModifyUserCallback handles user selection for modify command
func (h *Handlers) HandleModifyUserCallback(q *tgbotapi.CallbackQuery) (tgbotapi.EditMessageTextConfig, error) {
	parts := strings.Split(q.Data, ":")
	if len(parts) != 3 {
		return tgbotapi.EditMessageTextConfig{}, fmt.Errorf("invalid callback data")
	}

	dateStr := parts[1]
	var userID int64
	fmt.Sscanf(parts[2], "%d", &userID)

	dutyDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		edit := tgbotapi.NewEditMessageText(
			q.Message.Chat.ID,
			q.Message.MessageID,
			fmt.Sprintf("❌ Invalid date: %s", dateStr),
		)
		return edit, nil
	}

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

	if _, err := h.Scheduler.ChangeDutyUser(context.Background(), dutyDate, user.ID); err != nil {
		edit := tgbotapi.NewEditMessageText(
			q.Message.Chat.ID,
			q.Message.MessageID,
			fmt.Sprintf("❌ Failed to change duty for %s: %v", dateStr, err),
		)
		return edit, nil
	}

	edit := tgbotapi.NewEditMessageText(
		q.Message.Chat.ID,
		q.Message.MessageID,
		fmt.Sprintf("✅ Successfully modified duty for %s to be handled by <b>%s</b>.", dateStr, user.FirstName),
	)
	edit.ParseMode = tgbotapi.ModeHTML
	return edit, nil
}

// HandleToggleUserCallback handles user selection for toggle_active command
func (h *Handlers) HandleToggleUserCallback(q *tgbotapi.CallbackQuery) (tgbotapi.EditMessageTextConfig, error) {
	parts := strings.Split(q.Data, ":")
	if len(parts) != 2 {
		return tgbotapi.EditMessageTextConfig{}, fmt.Errorf("invalid callback data")
	}

	var userID int64
	fmt.Sscanf(parts[1], "%d", &userID)

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

	user.IsActive = !user.IsActive
	if err := h.Store.UpdateUser(context.Background(), user); err != nil {
		edit := tgbotapi.NewEditMessageText(
			q.Message.Chat.ID,
			q.Message.MessageID,
			fmt.Sprintf("❌ Failed to toggle status for %s: %v", user.FirstName, err),
		)
		return edit, nil
	}

	statusText := "active"
	if !user.IsActive {
		statusText = "inactive"
	}

	edit := tgbotapi.NewEditMessageText(
		q.Message.Chat.ID,
		q.Message.MessageID,
		fmt.Sprintf("✅ Successfully set status for <b>%s</b> to %s.", user.FirstName, statusText),
	)
	edit.ParseMode = tgbotapi.ModeHTML
	return edit, nil
}

// HandleOffDutyUserCallback handles user selection for offduty command
func (h *Handlers) HandleOffDutyUserCallback(q *tgbotapi.CallbackQuery) (tgbotapi.EditMessageTextConfig, error) {
	parts := strings.Split(q.Data, ":")
	if len(parts) != 3 {
		return tgbotapi.EditMessageTextConfig{}, fmt.Errorf("invalid callback data")
	}

	userName := parts[2]

	edit := tgbotapi.NewEditMessageText(
		q.Message.Chat.ID,
		q.Message.MessageID,
		fmt.Sprintf("🏖 <b>Set off-duty period for %s</b>\n\nPlease provide start and end dates:\n\n<code>/offduty %s START_DATE END_DATE</code>\n\nExample: <code>/offduty %s 2025-10-10 2025-10-15</code>", userName, userName, userName),
	)
	edit.ParseMode = tgbotapi.ModeHTML
	return edit, nil
}

// HandleVacation toggles the system vacation mode on/off.
func (h *Handlers) HandleVacation(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	isAdmin, err := h.checkAdmin(m.From.ID)
	if err != nil || !isAdmin {
		return tgbotapi.NewMessage(m.Chat.ID, adminOnlyMessage), nil
	}

	args := strings.Fields(m.CommandArguments())

	// If no arguments, show current status and toggle options
	if len(args) == 0 {
		isVacation, err := h.Scheduler.IsVacationMode(context.Background())
		if err != nil {
			return tgbotapi.NewMessage(m.Chat.ID, "❌ Failed to check vacation mode status."), nil
		}

		statusText := "🔴 <b>OFF</b>"
		if isVacation {
			statusText = "🟢 <b>ON</b>"
		}

		// Create toggle buttons
		var buttons [][]tgbotapi.InlineKeyboardButton
		buttons = append(buttons, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("🟢 Turn ON", "vacation:on"),
			tgbotapi.NewInlineKeyboardButtonData("🔴 Turn OFF", "vacation:off"),
		})

		keyboard := tgbotapi.NewInlineKeyboardMarkup(buttons...)
		msg := tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("🏖 <b>Vacation Mode</b>\n\nCurrent status: %s\n\nWhen vacation mode is ON, no duty assignments will be made.", statusText))
		msg.ParseMode = tgbotapi.ModeHTML
		msg.ReplyMarkup = keyboard
		return msg, nil
	}

	// Handle explicit on/off argument
	arg := strings.ToLower(args[0])
	var enabled bool
	switch arg {
	case "on", "true", "1", "enable":
		enabled = true
	case "off", "false", "0", "disable":
		enabled = false
	default:
		msg := tgbotapi.NewMessage(m.Chat.ID, "⚠️ Invalid argument. Use: <code>/vacation [on|off]</code>")
		msg.ParseMode = tgbotapi.ModeHTML
		return msg, nil
	}

	err = h.Scheduler.SetVacationMode(context.Background(), enabled)
	if err != nil {
		return tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("❌ Failed to set vacation mode: %v", err)), nil
	}

	statusText := "OFF"
	emoji := "🔴"
	if enabled {
		statusText = "ON"
		emoji = "🟢"
	}

	return tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("%s Vacation mode is now <b>%s</b>", emoji, statusText)), nil
}

// HandleVacationCallback handles the vacation mode toggle callback
func (h *Handlers) HandleVacationCallback(q *tgbotapi.CallbackQuery) (tgbotapi.EditMessageTextConfig, error) {
	parts := strings.Split(q.Data, ":")
	if len(parts) != 2 {
		return tgbotapi.EditMessageTextConfig{}, fmt.Errorf("invalid callback data")
	}

	enabled := parts[1] == "on"

	err := h.Scheduler.SetVacationMode(context.Background(), enabled)
	if err != nil {
		edit := tgbotapi.NewEditMessageText(
			q.Message.Chat.ID,
			q.Message.MessageID,
			fmt.Sprintf("❌ Failed to set vacation mode: %v", err),
		)
		return edit, nil
	}

	statusText := "OFF"
	emoji := "🔴"
	if enabled {
		statusText = "ON"
		emoji = "🟢"
	}

	edit := tgbotapi.NewEditMessageText(
		q.Message.Chat.ID,
		q.Message.MessageID,
		fmt.Sprintf("%s Vacation mode is now <b>%s</b>\n\nWhen vacation mode is ON, no duty assignments will be made.", emoji, statusText),
	)
	edit.ParseMode = tgbotapi.ModeHTML
	return edit, nil
}

// HandleComplete handles the /complete command for admins.
// Shows a list of all active chores with inline buttons for selection.
func (h *Handlers) HandleComplete(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	isAdmin, err := h.checkAdmin(m.From.ID)
	if err != nil || !isAdmin {
		return tgbotapi.NewMessage(m.Chat.ID, adminOnlyMessage), nil
	}

	chores, err := h.Store.GetActiveChores(context.Background())
	if err != nil {
		log.Printf("Failed to get active chores: %v", err)
		return tgbotapi.NewMessage(m.Chat.ID, "Failed to retrieve active chores."), nil
	}

	if len(chores) == 0 {
		msg := tgbotapi.NewMessage(m.Chat.ID, "✨ No active chores found! All clear.")
		return msg, nil
	}

	// Create inline keyboard with chore buttons
	var buttons [][]tgbotapi.InlineKeyboardButton
	for _, chore := range chores {
		if chore.User == nil {
			continue
		}

		// Format deadline for display
		deadlineStr := ""
		if !chore.DeadlineAt.IsZero() {
			deadlineStr = chore.DeadlineAt.Format("15:04")
		}

		// Button labels are plain text - use raw values (no HTML escaping)
		buttonLabel := fmt.Sprintf("%s - %s @%s", chore.User.FirstName, chore.Description, deadlineStr)
		row := []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(
				buttonLabel,
				fmt.Sprintf("complete_chore:%s", chore.ReminderID),
			),
		}
		buttons = append(buttons, row)
	}

	if len(buttons) == 0 {
		return tgbotapi.NewMessage(m.Chat.ID, "✨ No active chores found! All clear."), nil
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(buttons...)
	msg := tgbotapi.NewMessage(m.Chat.ID, "✅ <b>Mark Chore as Completed</b>\n\nSelect a chore to mark as completed:")
	msg.ParseMode = tgbotapi.ModeHTML
	msg.ReplyMarkup = keyboard
	return msg, nil
}

// HandleCompleteChoreCallback handles the callback when a chore is selected for completion by admin.
func (h *Handlers) HandleCompleteChoreCallback(q *tgbotapi.CallbackQuery) (tgbotapi.EditMessageTextConfig, error) {
	if q.From == nil {
		edit := tgbotapi.NewEditMessageText(
			q.Message.Chat.ID,
			q.Message.MessageID,
			adminOnlyMessage,
		)
		return edit, nil
	}

	isAdmin, err := h.checkAdmin(q.From.ID)
	if err != nil || !isAdmin {
		edit := tgbotapi.NewEditMessageText(
			q.Message.Chat.ID,
			q.Message.MessageID,
			adminOnlyMessage,
		)
		return edit, nil
	}

	// Extract reminderID from callback data: "complete_chore:reminderID"
	parts := strings.Split(q.Data, ":")
	if len(parts) != 2 {
		log.Printf("Invalid callback data format: %s", q.Data)
		edit := tgbotapi.NewEditMessageText(
			q.Message.Chat.ID,
			q.Message.MessageID,
			"❌ Error: Invalid callback data.",
		)
		return edit, nil
	}

	reminderID := parts[1]

	if h.ChoreReminderManager == nil {
		edit := tgbotapi.NewEditMessageText(
			q.Message.Chat.ID,
			q.Message.MessageID,
			"❌ Error: Reminder manager not configured.",
		)
		return edit, nil
	}

	// Get chore by reminderID from database
	chore, err := h.Store.GetChoreByReminderID(context.Background(), reminderID)
	if err != nil {
		log.Printf("Failed to get chore by reminderID %s: %v", reminderID, err)
		edit := tgbotapi.NewEditMessageText(
			q.Message.Chat.ID,
			q.Message.MessageID,
			"❌ Error: Could not find chore.",
		)
		return edit, nil
	}

	if chore == nil || chore.User == nil {
		edit := tgbotapi.NewEditMessageText(
			q.Message.Chat.ID,
			q.Message.MessageID,
			"❌ Error: Chore not found or has no assigned user.",
		)
		return edit, nil
	}

	// Create assignment from chore
	assignment := &ChoreAssignment{
		UserID:      chore.User.TelegramUserID,
		UserName:    chore.User.FirstName,
		Description: chore.Description,
		AssignedAt:  chore.AssignedAt,
		GroupID:     h.GroupID,
		ReminderID:  reminderID,
	}

	// Mark as completed in database
	if err := h.Store.CompleteChoreByReminderID(context.Background(), reminderID); err != nil {
		log.Printf("Failed to complete chore in database: %v", err)
		edit := tgbotapi.NewEditMessageText(
			q.Message.Chat.ID,
			q.Message.MessageID,
			"❌ Error: Failed to mark chore as completed. Please try again.",
		)
		return edit, nil
	}

	// Remove from active tracking
	h.ChoreReminderManager.CompleteChore(reminderID)

	// Send completion message to group (best-effort), but only after persistence succeeds
	groupNotified := true
	if err := h.ChoreReminderManager.SendCompletionToGroup(assignment); err != nil {
		log.Printf("Failed to send completion message to group: %v", err)
		groupNotified = false
	}

	// Update the message to show completion confirmation
	// Escape HTML at display time (chore.Description and chore.User.FirstName are stored unescaped)
	escapedDesc := html.EscapeString(chore.Description)
	escapedName := html.EscapeString(chore.User.FirstName)
	confirmationText := fmt.Sprintf("✅ <b>Chore Completed!</b>\n\nMarked <b>%s</b>'s chore as completed:\n\n<i>%s</i>", escapedName, escapedDesc)
	if groupNotified {
		confirmationText += "\n\nThe group has been notified!"
	}
	edit := tgbotapi.NewEditMessageText(
		q.Message.Chat.ID,
		q.Message.MessageID,
		confirmationText,
	)
	edit.ParseMode = tgbotapi.ModeHTML

	return edit, nil
}
