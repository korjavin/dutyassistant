package handlers

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// HandleEditChoreCallback handles the callback from the inline keyboard
// when an admin selects a chore to edit interactively.
func (h *Handlers) HandleEditChoreCallback(q *tgbotapi.CallbackQuery) (tgbotapi.Chattable, error) {
	isAdmin, err := h.checkAdmin(q.From.ID)
	if err != nil || !isAdmin {
		return tgbotapi.NewMessage(q.Message.Chat.ID, adminOnlyMessage), nil
	}

	parts := strings.Split(q.Data, ":")
	if len(parts) != 2 {
		return tgbotapi.NewMessage(q.Message.Chat.ID, "❌ Invalid callback data."), nil
	}

	choreIDStr := parts[1]
	choreID, err := strconv.ParseInt(choreIDStr, 10, 64)
	if err != nil {
		return tgbotapi.NewMessage(q.Message.Chat.ID, "❌ Invalid chore ID."), nil
	}

	chore, err := h.Store.GetRecurringChore(context.Background(), choreID)
	if err != nil {
		return tgbotapi.NewMessage(q.Message.Chat.ID, "❌ Failed to retrieve recurring chore."), nil
	}
	if chore == nil || !chore.IsActive {
		return tgbotapi.NewMessage(q.Message.Chat.ID, "❌ Recurring chore not found or is inactive."), nil
	}

	// Start an interactive session
	h.SessionManager.StartSession(q.Message.Chat.ID, q.From.ID, SessionTypeEditChore)
	session, exists := h.SessionManager.GetSession(q.Message.Chat.ID)
	if !exists {
		return tgbotapi.NewMessage(q.Message.Chat.ID, "❌ Failed to start interactive session."), nil
	}

	// Store chore context in the session
	session.SetData("chore_id", chore.ID)
	session.SetData("old_description", chore.Description)

	msgText := fmt.Sprintf(
		"🔄 <b>Editing Chore ID %d</b>\n\n<b>Current Description:</b>\n<i>%s</i>\n\nPlease reply with the new description for this chore (or /cancel to abort):",
		chore.ID,
		html.EscapeString(chore.Description),
	)

	// Send new message
	msg := tgbotapi.NewMessage(q.Message.Chat.ID, msgText)
	msg.ParseMode = tgbotapi.ModeHTML

	// Optionally edit the original message so buttons disappear
	editMsg := tgbotapi.NewEditMessageTextAndMarkup(
		q.Message.Chat.ID,
		q.Message.MessageID,
		"🔄 Session started: Edit periodic chore",
		tgbotapi.InlineKeyboardMarkup{InlineKeyboard: make([][]tgbotapi.InlineKeyboardButton, 0)},
	)
	if h.Bot != nil {
		if _, err := h.Bot.Send(editMsg); err != nil {
			slog.Error(fmt.Sprintf("Failed to clear inline keyboard: %v", err))
		}
	}

	return msg, nil
}

// HandleEditChoreInteractive handles incoming messages for the interactive chore edit session.
func (h *Handlers) HandleEditChoreInteractive(m *tgbotapi.Message) (tgbotapi.Chattable, error) {
	session, exists := h.SessionManager.GetSession(m.Chat.ID)
	if !exists || session.Type != SessionTypeEditChore {
		return nil, nil // Ignored, not in correct session
	}

	// Make sure the same user is responding
	if session.UserID != m.From.ID {
		return tgbotapi.NewMessage(m.Chat.ID, "❌ This session was started by another user. Please wait or start your own."), nil
	}

	h.SessionManager.TouchSession(m.Chat.ID)

	text := strings.TrimSpace(m.Text)

	// Check if the user is trying to cancel
	if strings.ToLower(text) == "/cancel" {
		h.SessionManager.EndSession(m.Chat.ID)
		return tgbotapi.NewMessage(m.Chat.ID, "🛑 Edit chore session cancelled."), nil
	}

	// If text is empty or starts with a slash, it's probably another command
	if len(text) == 0 || strings.HasPrefix(text, "/") {
		return tgbotapi.NewMessage(m.Chat.ID, "⚠️ Please provide a valid description, or send /cancel to abort."), nil
	}

	// Get chore context from session
	val, ok := session.GetData("chore_id")
	if !ok {
		h.SessionManager.EndSession(m.Chat.ID)
		return tgbotapi.NewMessage(m.Chat.ID, "❌ Session error: chore ID not found. Please try again."), nil
	}
	choreID := val.(int64)

	oldDescVal, _ := session.GetData("old_description")
	oldDescription := ""
	if oldDescVal != nil {
		oldDescription = oldDescVal.(string)
	}

	// Proceed with update
	if err := h.Store.UpdateRecurringChoreDescription(context.Background(), choreID, text); err != nil {
		h.SessionManager.EndSession(m.Chat.ID)
		return tgbotapi.NewMessage(m.Chat.ID, "❌ Failed to update recurring chore description in database."), nil
	}

	h.SessionManager.EndSession(m.Chat.ID)

	msg := tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf(
		"✅ <b>Periodic chore description updated interactively:</b>\n\n<b>Old:</b> <i>%s</i>\n<b>New:</b> <i>%s</i>",
		html.EscapeString(oldDescription),
		html.EscapeString(text),
	))
	msg.ParseMode = tgbotapi.ModeHTML
	return msg, nil
}
