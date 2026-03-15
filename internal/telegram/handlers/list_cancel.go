package handlers

import (
	"context"
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/telegram/keyboard"
)

// HandleList handles the /list command for admins. Format: /list chore
func (h *Handlers) HandleList(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	// 1. Admin check
	isAdmin, err := h.checkAdmin(m.From.ID)
	if err != nil || !isAdmin {
		return tgbotapi.NewMessage(m.Chat.ID, adminOnlyMessage), nil
	}

	args := strings.TrimSpace(m.CommandArguments())

	if args == "" {
		msg := tgbotapi.NewMessage(m.Chat.ID, "Select which type of chores you want to list:")
		msg.ReplyMarkup = keyboard.ListMenu()
		return msg, nil
	}

	if strings.ToLower(args) == "chore" {
		chores, err := h.Store.GetActiveRecurringChores(context.Background())
		if err != nil {
			return tgbotapi.NewMessage(m.Chat.ID, "❌ Failed to retrieve recurring chores."), nil
		}

		if len(chores) == 0 {
			return tgbotapi.NewMessage(m.Chat.ID, "No active recurring chores found."), nil
		}

		var sb strings.Builder
		sb.WriteString("📋 <b>Active Recurring Chores:</b>\n\n")

		berlinLoc, err := time.LoadLocation("Europe/Berlin")
		if err != nil {
			berlinLoc = time.UTC // fallback
		}

		for _, chore := range chores {
			escapedDesc := html.EscapeString(chore.Description)
			nextRunStr := chore.NextRunAt.In(berlinLoc).Format("2006-01-02 15:04 MST")
			sb.WriteString(fmt.Sprintf("<b>ID:</b> <code>%d</code>\n", chore.ID))
			sb.WriteString(fmt.Sprintf("<b>Description:</b> <i>%s</i>\n", escapedDesc))
			sb.WriteString(fmt.Sprintf("<b>Interval:</b> every %d days\n", chore.Interval))
			sb.WriteString(fmt.Sprintf("<b>Next Run:</b> %s\n\n", nextRunStr))
		}

		msg := tgbotapi.NewMessage(m.Chat.ID, sb.String())
		msg.ParseMode = tgbotapi.ModeHTML
		return msg, nil
	}

	if strings.ToLower(args) == "task" {
		chores, err := h.Store.ListActiveChores(context.Background())
		if err != nil {
			return tgbotapi.NewMessage(m.Chat.ID, "❌ Failed to retrieve active regular chores."), nil
		}

		if len(chores) == 0 {
			return tgbotapi.NewMessage(m.Chat.ID, "No active regular chores found."), nil
		}

		var sb strings.Builder
		sb.WriteString("📋 <b>Active Regular Chores:</b>\n\n")

		berlinLoc, err := time.LoadLocation("Europe/Berlin")
		if err != nil {
			berlinLoc = time.UTC // fallback
		}

		for _, chore := range chores {
			escapedDesc := html.EscapeString(chore.Description)
			assignedAtStr := chore.AssignedAt.In(berlinLoc).Format("2006-01-02 15:04 MST")
			assignedUser := "Unknown"
			if chore.User != nil {
				assignedUser = html.EscapeString(chore.User.FirstName)
			}
			sb.WriteString(fmt.Sprintf("<b>ID:</b> <code>%d</code>\n", chore.ID))
			sb.WriteString(fmt.Sprintf("<b>Description:</b> <i>%s</i>\n", escapedDesc))
			sb.WriteString(fmt.Sprintf("<b>Assigned to:</b> %s\n", assignedUser))
			sb.WriteString(fmt.Sprintf("<b>Assigned at:</b> %s\n\n", assignedAtStr))
		}

		msg := tgbotapi.NewMessage(m.Chat.ID, sb.String())
		msg.ParseMode = tgbotapi.ModeHTML
		return msg, nil
	}

	return tgbotapi.NewMessage(m.Chat.ID, "Unknown list command. Use /list chore to list periodic chores or /list task to list regular chores."), nil
}

// HandleListCallback handles interactive /list menu selections
func (h *Handlers) HandleListCallback(q *tgbotapi.CallbackQuery) (tgbotapi.Chattable, error) {
	// 1. Admin check
	isAdmin, err := h.checkAdmin(q.From.ID)
	if err != nil || !isAdmin {
		return nil, nil // Silently ignore for non-admins
	}

	parts := strings.Split(q.Data, ":")
	if len(parts) != 2 {
		return nil, nil
	}

	listType := parts[1]

	var sb strings.Builder
	berlinLoc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		berlinLoc = time.UTC // fallback
	}

	if listType == "chore" {
		chores, err := h.Store.GetActiveRecurringChores(context.Background())
		if err != nil {
			msg := tgbotapi.NewEditMessageTextAndMarkup(q.Message.Chat.ID, q.Message.MessageID, "❌ Failed to retrieve recurring chores.", tgbotapi.InlineKeyboardMarkup{InlineKeyboard: make([][]tgbotapi.InlineKeyboardButton, 0)})
			return msg, nil
		}

		if len(chores) == 0 {
			msg := tgbotapi.NewEditMessageTextAndMarkup(q.Message.Chat.ID, q.Message.MessageID, "No active recurring chores found.", tgbotapi.InlineKeyboardMarkup{InlineKeyboard: make([][]tgbotapi.InlineKeyboardButton, 0)})
			return msg, nil
		}

		sb.WriteString("📋 <b>Active Recurring Chores:</b>\n\n")
		for _, chore := range chores {
			escapedDesc := html.EscapeString(chore.Description)
			nextRunStr := chore.NextRunAt.In(berlinLoc).Format("2006-01-02 15:04 MST")
			sb.WriteString(fmt.Sprintf("<b>ID:</b> <code>%d</code>\n", chore.ID))
			sb.WriteString(fmt.Sprintf("<b>Description:</b> <i>%s</i>\n", escapedDesc))
			sb.WriteString(fmt.Sprintf("<b>Interval:</b> every %d days\n", chore.Interval))
			sb.WriteString(fmt.Sprintf("<b>Next Run:</b> %s\n\n", nextRunStr))
		}
	} else if listType == "task" {
		chores, err := h.Store.ListActiveChores(context.Background())
		if err != nil {
			msg := tgbotapi.NewEditMessageTextAndMarkup(q.Message.Chat.ID, q.Message.MessageID, "❌ Failed to retrieve active regular chores.", tgbotapi.InlineKeyboardMarkup{InlineKeyboard: make([][]tgbotapi.InlineKeyboardButton, 0)})
			return msg, nil
		}

		if len(chores) == 0 {
			msg := tgbotapi.NewEditMessageTextAndMarkup(q.Message.Chat.ID, q.Message.MessageID, "No active regular chores found.", tgbotapi.InlineKeyboardMarkup{InlineKeyboard: make([][]tgbotapi.InlineKeyboardButton, 0)})
			return msg, nil
		}

		sb.WriteString("📋 <b>Active Regular Chores:</b>\n\n")
		for _, chore := range chores {
			escapedDesc := html.EscapeString(chore.Description)
			assignedAtStr := chore.AssignedAt.In(berlinLoc).Format("2006-01-02 15:04 MST")
			assignedUser := "Unknown"
			if chore.User != nil {
				assignedUser = html.EscapeString(chore.User.FirstName)
			}
			sb.WriteString(fmt.Sprintf("<b>ID:</b> <code>%d</code>\n", chore.ID))
			sb.WriteString(fmt.Sprintf("<b>Description:</b> <i>%s</i>\n", escapedDesc))
			sb.WriteString(fmt.Sprintf("<b>Assigned to:</b> %s\n", assignedUser))
			sb.WriteString(fmt.Sprintf("<b>Assigned at:</b> %s\n\n", assignedAtStr))
		}
	} else {
		return nil, nil // Ignore unknown types
	}

	// Update the original message and clear the keyboard
	editMsg := tgbotapi.NewEditMessageTextAndMarkup(
		q.Message.Chat.ID,
		q.Message.MessageID,
		sb.String(),
		tgbotapi.InlineKeyboardMarkup{InlineKeyboard: make([][]tgbotapi.InlineKeyboardButton, 0)},
	)
	editMsg.ParseMode = tgbotapi.ModeHTML
	return editMsg, nil
}

// HandleCancelIDSelection shows an interactive menu for a specific ID
func (h *Handlers) HandleCancelIDSelection(m *tgbotapi.Message, id string) (tgbotapi.MessageConfig, error) {
	var keyboard [][]tgbotapi.InlineKeyboardButton

	keyboard = append(keyboard, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("Cancel periodic chore %s", id), fmt.Sprintf("cancel_assignment:R%s", id)),
	))
	keyboard = append(keyboard, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("Cancel regular task %s", id), fmt.Sprintf("cancel_assignment:A%s", id)),
	))
	keyboard = append(keyboard, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("Show all items", "cancel_interactive"),
	))
	keyboard = append(keyboard, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("❌ Cancel operation", "cancel_flow"),
	))

	markup := tgbotapi.NewInlineKeyboardMarkup(keyboard...)
	msg := tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("What do you want to cancel with ID %s?", id))
	msg.ReplyMarkup = &markup
	return msg, nil
}

// HandleCancel handles the /cancel command for admins. Format: /cancel chore <id>
func (h *Handlers) HandleCancel(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	// 1. Admin check
	isAdmin, err := h.checkAdmin(m.From.ID)
	if err != nil || !isAdmin {
		return tgbotapi.NewMessage(m.Chat.ID, adminOnlyMessage), nil
	}

	args := strings.TrimSpace(m.CommandArguments())
	if args == "" {
		return h.HandleCancelInteractive(m)
	}

	parts := strings.Fields(args)

	// If there's only one argument, check if it's an ID
	if len(parts) == 1 {
		// Check if it's a number
		if _, err := strconv.ParseInt(parts[0], 10, 64); err == nil {
			return h.HandleCancelIDSelection(m, parts[0])
		}
	}

	if len(parts) == 2 && strings.ToLower(parts[0]) == "chore" {
		choreID, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return tgbotapi.NewMessage(m.Chat.ID, "❌ Invalid chore ID format. Use /cancel chore <id>"), nil
		}

		chore, err := h.Store.GetRecurringChore(context.Background(), choreID)
		if err != nil {
			return tgbotapi.NewMessage(m.Chat.ID, "❌ Failed to retrieve recurring chore."), nil
		}

		if chore == nil || !chore.IsActive {
			return tgbotapi.NewMessage(m.Chat.ID, "❌ Recurring chore not found or already cancelled."), nil
		}

		if err := h.Store.CancelRecurringChore(context.Background(), choreID); err != nil {
			return tgbotapi.NewMessage(m.Chat.ID, "❌ Failed to cancel recurring chore."), nil
		}

		msg := tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("✅ <b>Periodic chore cancelled:</b>\n<i>%s</i>", html.EscapeString(chore.Description)))
		msg.ParseMode = tgbotapi.ModeHTML
		return msg, nil
	}

	if len(parts) == 2 && strings.ToLower(parts[0]) == "task" {
		choreID, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return tgbotapi.NewMessage(m.Chat.ID, "❌ Invalid task ID format. Use /cancel task <id>"), nil
		}

		chore, err := h.Store.CancelChore(context.Background(), choreID)
		if err != nil {
			return tgbotapi.NewMessage(m.Chat.ID, "❌ Failed to cancel regular chore (not found, already completed, or cancelled)."), nil
		}

		if h.ChoreReminderManager != nil && chore != nil && chore.ReminderID != "" {
			h.ChoreReminderManager.CancelChore(chore.ReminderID)
		}

		msg := tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("✅ <b>Regular chore %d cancelled successfully.</b>", choreID))
		msg.ParseMode = tgbotapi.ModeHTML
		return msg, nil
	}

	return tgbotapi.NewMessage(m.Chat.ID, "Unknown cancel command. Use /cancel chore <id> or /cancel task <id>"), nil
}

// HandleEdit handles the /edit command for admins.
// It supports direct editing via "/edit chore <id> <new description>"
// or interactive mode by simply sending "/edit" or "/edit chore".
func (h *Handlers) HandleEdit(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	// 1. Admin check
	isAdmin, err := h.checkAdmin(m.From.ID)
	if err != nil || !isAdmin {
		return tgbotapi.NewMessage(m.Chat.ID, adminOnlyMessage), nil
	}

	args := strings.TrimSpace(m.CommandArguments())
	parts := strings.Fields(args)

	// Interactive mode: /edit or /edit chore
	if len(parts) == 0 || (len(parts) == 1 && strings.ToLower(parts[0]) == "chore") {
		chores, err := h.Store.GetActiveRecurringChores(context.Background())
		if err != nil {
			return tgbotapi.NewMessage(m.Chat.ID, "❌ Failed to retrieve recurring chores."), nil
		}

		if len(chores) == 0 {
			return tgbotapi.NewMessage(m.Chat.ID, "No active recurring chores found to edit."), nil
		}

		var buttons [][]tgbotapi.InlineKeyboardButton
		for _, chore := range chores {
			// Ensure description fits on button
			desc := chore.Description
			if len(desc) > 30 {
				desc = desc[:27] + "..."
			}
			row := []tgbotapi.InlineKeyboardButton{
				tgbotapi.NewInlineKeyboardButtonData(
					fmt.Sprintf("ID: %d - %s", chore.ID, desc),
					fmt.Sprintf("edit_chore:%d", chore.ID),
				),
			}
			buttons = append(buttons, row)
		}

		keyboard := tgbotapi.NewInlineKeyboardMarkup(buttons...)
		msg := tgbotapi.NewMessage(m.Chat.ID, "🔄 <b>Edit periodic chore</b>\n\nSelect a chore to edit its description:")
		msg.ParseMode = tgbotapi.ModeHTML
		msg.ReplyMarkup = keyboard
		return msg, nil
	}

	// Direct command mode: /edit chore <id> <new description>
	if len(parts) >= 3 && strings.ToLower(parts[0]) == "chore" {
		choreID, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return tgbotapi.NewMessage(m.Chat.ID, "❌ Invalid chore ID format. Use /edit chore <id> <new description>"), nil
		}

		newDescription := strings.Join(parts[2:], " ")

		chore, err := h.Store.GetRecurringChore(context.Background(), choreID)
		if err != nil {
			return tgbotapi.NewMessage(m.Chat.ID, "❌ Failed to retrieve recurring chore."), nil
		}

		if chore == nil || !chore.IsActive {
			return tgbotapi.NewMessage(m.Chat.ID, "❌ Recurring chore not found or already cancelled."), nil
		}

		oldDescription := chore.Description

		if err := h.Store.UpdateRecurringChoreDescription(context.Background(), choreID, newDescription); err != nil {
			return tgbotapi.NewMessage(m.Chat.ID, "❌ Failed to update recurring chore description."), nil
		}

		msg := tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("✅ <b>Periodic chore description updated:</b>\n<b>Old:</b> <i>%s</i>\n<b>New:</b> <i>%s</i>", html.EscapeString(oldDescription), html.EscapeString(newDescription)))
		msg.ParseMode = tgbotapi.ModeHTML
		return msg, nil
	}

	return tgbotapi.NewMessage(m.Chat.ID, "Use /edit chore <id> <new description> or simply /edit to select interactively."), nil
}
