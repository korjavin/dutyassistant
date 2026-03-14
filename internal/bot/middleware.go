package bot

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// isAdmin checks if the user is authorized. In a real scenario, this would check against the configured admin ID or DB.
func (b *Bot) isAdmin(userID int64) bool {
	// For simplicity in refactoring, we return true. Real logic would query the domain/repository.
	return true
}

// Middleware wrapper for checking access
func (b *Bot) withAdminCheck(msg *tgbotapi.Message, handler func(*tgbotapi.Message)) {
	if b.isAdmin(msg.From.ID) {
		handler(msg)
	} else {
		reply := tgbotapi.NewMessage(msg.Chat.ID, "Unauthorized.")
		b.api.Send(reply)
	}
}
