package handlers

import (
	"context"
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	adminOnlyMessage      = "Sorry, this command is for admins only."
	invalidCommandMessage = "Invalid command format. Use /help for more information."
	userNotFoundMessage   = "Could not find user: %s"
	assignSuccessMessage  = "Successfully assigned %s to duty on %s."
	assignFailureMessage  = "Failed to assign %s to duty on %s."
)

// checkAdmin is a helper function to verify if a user is an admin.
func (h *Handlers) checkAdmin(telegramUserID int64) (bool, error) {
	user, err := h.Store.GetUserByTelegramID(context.Background(), telegramUserID)
	if err != nil {
		return false, err
	}
	return user.IsAdmin, nil
}

// HandleAssign handles the /assign command for admins.
// Format: /assign <username> <date>
func (h *Handlers) HandleAssign(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	isAdmin, err := h.checkAdmin(m.From.ID)
	if err != nil || !isAdmin {
		return tgbotapi.NewMessage(m.Chat.ID, adminOnlyMessage), nil
	}

	args := strings.Fields(m.CommandArguments())
	if len(args) != 2 {
		return tgbotapi.NewMessage(m.Chat.ID, invalidCommandMessage), nil
	}

	userName := args[0]
	dateStr := args[1]

	user, err := h.Store.GetUserByName(context.Background(), userName)
	if err != nil {
		return tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf(userNotFoundMessage, userName)), nil
	}

	err = h.Scheduler.AssignDuty(context.Background(), user, dateStr)
	if err != nil {
		return tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf(assignFailureMessage, userName, dateStr)), nil
	}

	message := fmt.Sprintf(assignSuccessMessage, userName, dateStr)
	return tgbotapi.NewMessage(m.Chat.ID, message), nil
}

// HandleModify handles the /modify command.
// Format: /modify <date> <new_username>
func (h *Handlers) HandleModify(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	isAdmin, err := h.checkAdmin(m.From.ID)
	if err != nil || !isAdmin {
		return tgbotapi.NewMessage(m.Chat.ID, adminOnlyMessage), nil
	}

	args := strings.Fields(m.CommandArguments())
	if len(args) != 2 {
		return tgbotapi.NewMessage(m.Chat.ID, "Invalid command format. Use /modify <date> <new_username>"), nil
	}

	dateStr := args[0]
	userName := args[1]

	user, err := h.Store.GetUserByName(context.Background(), userName)
	if err != nil {
		return tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf(userNotFoundMessage, userName)), nil
	}

	// Re-using AssignDuty for modification, as it should overwrite the existing duty.
	err = h.Scheduler.AssignDuty(context.Background(), user, dateStr)
	if err != nil {
		return tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("Failed to modify duty for date %s.", dateStr)), nil
	}

	message := fmt.Sprintf("Successfully modified duty for %s to be handled by %s.", dateStr, userName)
	return tgbotapi.NewMessage(m.Chat.ID, message), nil
}

// HandleUsers handles the /users command.
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

// HandleToggleActive handles the /toggle_active command.
// Format: /toggle_active <username>
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
	if err != nil {
		return tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf(userNotFoundMessage, userName)), nil
	}

	// Toggle the active status
	user.IsActive = !user.IsActive

	if err := h.Store.UpdateUser(context.Background(), user); err != nil {
		return tgbotapi.NewMessage(m.Chat.ID, "Failed to update user status."), nil
	}

	newStatus := "Active"
	if !user.IsActive {
		newStatus = "Inactive"
	}
	message := fmt.Sprintf("Successfully set status for %s to %s.", user.FirstName, newStatus)
	return tgbotapi.NewMessage(m.Chat.ID, message), nil
}