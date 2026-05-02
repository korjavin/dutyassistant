package handlers

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// HandleChoreActionCallback handles chore menu actions (list, create, delete, complete)
func (h *Handlers) HandleChoreActionCallback(q *tgbotapi.CallbackQuery) (tgbotapi.Chattable, error) {
	isAdmin, err := h.checkAdmin(q.From.ID)
	if err != nil || !isAdmin {
		return nil, nil // Ignore non-admin clicks silently
	}

	parts := strings.Split(q.Data, ":")
	if len(parts) < 2 {
		return nil, nil
	}
	action := parts[1]

	switch action {
	case "list":
		// Get all chores (recurring and active)
		return h.handleChoreListInteractive(q)
	case "create":
		h.SessionManager.StartSession(q.Message.Chat.ID, q.From.ID, SessionTypeChoreCreation)
		editMsg := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, "📝 <b>Interactive Chore Mode</b>\n\nWhat chore do you want to create?\n\nJust send me the description in your next message.")
		editMsg.ParseMode = tgbotapi.ModeHTML
		editMsg.ReplyMarkup = nil
		return editMsg, nil
	case "delete":
		return h.handleChoreDeleteInteractive(q)
	case "complete":
		return h.handleChoreCompleteInteractive(q)
	default:
		return nil, nil
	}
}

func (h *Handlers) handleChoreListInteractive(q *tgbotapi.CallbackQuery) (tgbotapi.Chattable, error) {
	chores, err := h.Store.ListActiveChores(context.Background())
	if err != nil {
		slog.Error(fmt.Sprintf("Error fetching chores: %v", err))
		editMsg := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, "❌ Failed to fetch active chores.")
		editMsg.ReplyMarkup = nil
		return editMsg, nil
	}

	rChores, err := h.Store.GetActiveRecurringChores(context.Background())
	if err != nil {
		slog.Error(fmt.Sprintf("Error fetching recurring chores: %v", err))
		editMsg := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, "❌ Failed to fetch recurring chores.")
		editMsg.ReplyMarkup = nil
		return editMsg, nil
	}

	if len(chores) == 0 && len(rChores) == 0 {
		editMsg := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, "✅ There are no active or recurring chores right now!")
		editMsg.ReplyMarkup = nil
		return editMsg, nil
	}

	var sb strings.Builder
	tz := os.Getenv("CHORE_TIMEZONE")
	if tz == "" {
		tz = "Europe/Berlin"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.Local
	}

	if len(chores) > 0 {
		sb.WriteString("📋 <b>Active Chores:</b>\n\n")
		for _, chore := range chores {
			assignedAt := chore.AssignedAt.In(loc).Format("2006-01-02 15:04")
			sb.WriteString(fmt.Sprintf("• <b>%s</b>: <i>%s</i>\n  <span class=\"tg-spoiler\">ID: %d, Assigned: %s</span>\n",
				html.EscapeString(chore.User.FirstName), html.EscapeString(chore.Description), chore.ID, assignedAt))
		}
		sb.WriteString("\n")
	}

	if len(rChores) > 0 {
		sb.WriteString("🔁 <b>Recurring Chores:</b>\n\n")
		for _, rc := range rChores {
			nextRun := rc.NextRunAt.In(loc).Format("2006-01-02 15:04")
			sb.WriteString(fmt.Sprintf("• <b>Every %d days</b>: <i>%s</i>\n  <span class=\"tg-spoiler\">ID: R%d, Next: %s</span>\n",
				rc.Interval, html.EscapeString(rc.Description), rc.ID, nextRun))
		}
	}

	editMsg := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, sb.String())
	editMsg.ParseMode = tgbotapi.ModeHTML
	editMsg.ReplyMarkup = nil
	return editMsg, nil
}

func (h *Handlers) handleChoreDeleteInteractive(q *tgbotapi.CallbackQuery) (tgbotapi.Chattable, error) {
	chores, err := h.Store.ListActiveChores(context.Background())
	if err != nil {
		editMsg := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, "❌ Failed to fetch active chores.")
		editMsg.ReplyMarkup = nil
		return editMsg, nil
	}

	rChores, err := h.Store.GetActiveRecurringChores(context.Background())
	if err != nil {
		editMsg := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, "❌ Failed to fetch recurring chores.")
		editMsg.ReplyMarkup = nil
		return editMsg, nil
	}

	if len(chores) == 0 && len(rChores) == 0 {
		editMsg := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, "✅ There are no active or recurring chores to delete!")
		editMsg.ReplyMarkup = nil
		return editMsg, nil
	}

	var keyboard [][]tgbotapi.InlineKeyboardButton

	for _, c := range chores {
		desc := c.Description
		runes := []rune(desc)
		if len(runes) > 30 {
			desc = string(runes[:27]) + "..."
		}
		btnText := fmt.Sprintf("A%d: %s", c.ID, desc)
		cbData := fmt.Sprintf("chore_delete:A%d", c.ID)
		keyboard = append(keyboard, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(btnText, cbData)))
	}

	for _, r := range rChores {
		desc := r.Description
		runes := []rune(desc)
		if len(runes) > 30 {
			desc = string(runes[:27]) + "..."
		}
		btnText := fmt.Sprintf("R%d: %s (Every %d days)", r.ID, desc, r.Interval)
		cbData := fmt.Sprintf("chore_delete:R%d", r.ID)
		keyboard = append(keyboard, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(btnText, cbData)))
	}

	keyboard = append(keyboard, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("❌ Cancel", "cancel_flow")))
	markup := tgbotapi.NewInlineKeyboardMarkup(keyboard...)

	editMsg := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, "Select a chore to delete:")
	editMsg.ReplyMarkup = &markup
	return editMsg, nil
}

// HandleChoreDeleteCallback handles the confirmation prompt for deleting a chore
func (h *Handlers) HandleChoreDeleteCallback(q *tgbotapi.CallbackQuery) (tgbotapi.Chattable, error) {
	isAdmin, err := h.checkAdmin(q.From.ID)
	if err != nil || !isAdmin {
		return nil, nil // Ignore non-admin clicks silently
	}

	parts := strings.Split(q.Data, ":")
	if len(parts) < 2 {
		return nil, nil
	}
	choreIDStr := parts[1]

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Confirm", fmt.Sprintf("chore_delete_confirm:%s", choreIDStr)),
			tgbotapi.NewInlineKeyboardButtonData("❌ Cancel", "cancel_flow"),
		),
	)

	editMsg := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, fmt.Sprintf("Are you sure you want to delete chore %s?", choreIDStr))
	editMsg.ReplyMarkup = &keyboard
	return editMsg, nil
}

// HandleChoreDeleteConfirmCallback actually deletes the chore
func (h *Handlers) HandleChoreDeleteConfirmCallback(q *tgbotapi.CallbackQuery) (tgbotapi.Chattable, error) {
	isAdmin, err := h.checkAdmin(q.From.ID)
	if err != nil || !isAdmin {
		return nil, nil // Ignore non-admin clicks silently
	}

	parts := strings.Split(q.Data, ":")
	if len(parts) < 2 {
		return nil, nil
	}
	choreIDStr := parts[1]

	var actionErr error
	if strings.HasPrefix(choreIDStr, "R") {
		// Delete recurring chore
		id, parseErr := strconv.Atoi(strings.TrimPrefix(choreIDStr, "R"))
		if parseErr == nil {
			actionErr = h.Store.CancelRecurringChore(context.Background(), int64(id))
		} else {
			actionErr = parseErr
		}
	} else if strings.HasPrefix(choreIDStr, "A") {
		// Cancel active chore
		id, parseErr := strconv.Atoi(strings.TrimPrefix(choreIDStr, "A"))
		if parseErr == nil {
			chore, cancelErr := h.Store.CancelChore(context.Background(), int64(id))
			actionErr = cancelErr
			if actionErr == nil {
				if h.ChoreReminderManager != nil && chore != nil && chore.ReminderID != "" {
					h.ChoreReminderManager.CancelChore(chore.ReminderID)
				}
			}
		} else {
			actionErr = parseErr
		}
	}

	msgText := "✅ Chore deleted successfully."
	if actionErr != nil {
		slog.Error(fmt.Sprintf("Error deleting chore %s: %v", choreIDStr, actionErr))
		msgText = "❌ Failed to delete chore."
	}

	editMsg := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, msgText)
	editMsg.ReplyMarkup = nil
	return editMsg, nil
}

func (h *Handlers) handleChoreCompleteInteractive(q *tgbotapi.CallbackQuery) (tgbotapi.Chattable, error) {
	chores, err := h.Store.GetActiveChores(context.Background())
	if err != nil {
		slog.Error(fmt.Sprintf("Error fetching active chores: %v", err))
		editMsg := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, "❌ Failed to fetch active chores.")
		editMsg.ReplyMarkup = nil
		return editMsg, nil
	}

	if len(chores) == 0 {
		editMsg := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, "✨ No active chores found! All clear.")
		editMsg.ReplyMarkup = nil
		return editMsg, nil
	}

	var keyboard [][]tgbotapi.InlineKeyboardButton
	for _, chore := range chores {
		if chore.User == nil {
			continue
		}
		desc := chore.Description
		runes := []rune(desc)
		if len(runes) > 30 {
			desc = string(runes[:27]) + "..."
		}
		btnText := fmt.Sprintf("%s - %s", chore.User.FirstName, desc)
		cbData := fmt.Sprintf("complete_chore:%s", chore.ReminderID)
		keyboard = append(keyboard, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(btnText, cbData)))
	}

	if len(keyboard) == 0 {
		editMsg := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, "✨ No active chores found! All clear.")
		editMsg.ReplyMarkup = nil
		return editMsg, nil
	}

	keyboard = append(keyboard, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("❌ Cancel", "cancel_flow")))
	markup := tgbotapi.NewInlineKeyboardMarkup(keyboard...)

	editMsg := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, "✅ <b>Mark Chore as Completed</b>\n\nSelect a chore to mark as completed:")
	editMsg.ParseMode = tgbotapi.ModeHTML
	editMsg.ReplyMarkup = &markup
	return editMsg, nil
}
