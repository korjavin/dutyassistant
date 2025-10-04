package handlers

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

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

// HandleAssign handles the /assign command for admins. Format: /assign <username> <date>
func (h *Handlers) HandleAssign(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	isAdmin, err := h.checkAdmin(m.From.ID)
	if err != nil || !isAdmin {
		return tgbotapi.NewMessage(m.Chat.ID, adminOnlyMessage), nil
	}

	args := strings.Fields(m.CommandArguments())
	if len(args) != 2 {
		return tgbotapi.NewMessage(m.Chat.ID, "Invalid command format. Use /assign <username> <YYYY-MM-DD>"), nil
	}

	userName, dateStr := args[0], args[1]
	dutyDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return tgbotapi.NewMessage(m.Chat.ID, invalidDateMessage), nil
	}

	user, err := h.Store.GetUserByName(context.Background(), userName)
	if err != nil || user == nil {
		return tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf(userNotFoundMessage, userName)), nil
	}

	if err := h.Scheduler.AssignDuty(context.Background(), user, dutyDate); err != nil {
		return tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf(assignFailureMessage, userName, dateStr)), nil
	}

	return tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf(assignSuccessMessage, userName, dateStr)), nil
}

// HandleModify handles the /modify command. Format: /modify <date> <new_username>
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

	// Re-using AssignDuty for modification, as it overwrites the existing duty.
	if err := h.Scheduler.AssignDuty(context.Background(), user, dutyDate); err != nil {
		return tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf(modifyFailureMessage, dateStr)), nil
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
	builder.WriteString("<b>User List:</b>\n")
	for _, u := range users {
		status := "Active"
		if !u.IsActive {
			status = "Inactive"
		}
		adminStatus := ""
		if u.IsAdmin {
			adminStatus = " (Admin)"
		}
		builder.WriteString(fmt.Sprintf("- %s%s: %s\n", u.FirstName, adminStatus, status))
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