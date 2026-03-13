package handlers

import (
	"context"
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// HandleList handles the /list command for admins. Format: /list chore
func (h *Handlers) HandleList(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	// 1. Admin check
	isAdmin, err := h.checkAdmin(m.From.ID)
	if err != nil || !isAdmin {
		return tgbotapi.NewMessage(m.Chat.ID, adminOnlyMessage), nil
	}

	args := strings.TrimSpace(m.CommandArguments())

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
