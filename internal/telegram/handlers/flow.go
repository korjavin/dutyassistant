package handlers

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// HandleCancelFlow edits the message to indicate the operation was cancelled and removes any inline keyboards.
func (h *Handlers) HandleCancelFlow(q *tgbotapi.CallbackQuery) (tgbotapi.Chattable, error) {
	isAdmin, err := h.checkAdmin(q.From.ID)
	if err != nil || !isAdmin {
		return nil, nil // Ignore non-admin clicks silently
	}

	editMsg := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, "❌ Operation cancelled")
	editMsg.ReplyMarkup = nil
	return editMsg, nil
}
