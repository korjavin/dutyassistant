package bot

import (
	"context"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/bot/fsm"
)

func (b *Bot) handleCancelCommand(msg *tgbotapi.Message) {
	session := b.sessionManager.GetOrCreateSession(msg.From.ID, fsm.StateInit)
	machine := session.FSM

	if machine.CurrentState() != fsm.StateInit {
		machine.ProcessEvent(context.Background(), fsm.EventCancel, nil)
		reply := tgbotapi.NewMessage(msg.Chat.ID, "Operation cancelled.")
		b.api.Send(reply)
	} else {
		reply := tgbotapi.NewMessage(msg.Chat.ID, "Nothing to cancel.")
		b.api.Send(reply)
	}
}

func (b *Bot) handleRatingPromptInput(msg *tgbotapi.Message) {
	// Stub for daily rating FSM handler
}
