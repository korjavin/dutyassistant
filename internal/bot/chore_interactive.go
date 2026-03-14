package bot

import (
	"context"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/bot/fsm"
)

func (b *Bot) handleChoreCommand(msg *tgbotapi.Message) {
	session := b.sessionManager.GetOrCreateSession(msg.From.ID, fsm.StateInit)
	machine := fsm.InitializeChoreFlow()
	session.FSM = machine

	machine.ProcessEvent(context.Background(), fsm.EventStartChore, nil)

	reply := tgbotapi.NewMessage(msg.Chat.ID, "Enter chore description:")
	b.api.Send(reply)
}

func (b *Bot) handleChoreDescriptionInput(msg *tgbotapi.Message) {
	session := b.sessionManager.GetOrCreateSession(msg.From.ID, fsm.StateInit)
	machine := session.FSM

	if machine.CurrentState() != fsm.StateChoreDesc {
		log.Printf("Invalid state for chore description input: %s", machine.CurrentState())
		return
	}

	machine.ProcessEvent(context.Background(), fsm.EventInputDesc, nil)

	reply := tgbotapi.NewMessage(msg.Chat.ID, "Enter chore duration (e.g. 1h):")
	b.api.Send(reply)
}

func (b *Bot) handleChoreDurationInput(msg *tgbotapi.Message) {
	session := b.sessionManager.GetOrCreateSession(msg.From.ID, fsm.StateInit)
	machine := session.FSM

	if machine.CurrentState() != fsm.StateChoreDuration {
		log.Printf("Invalid state for chore duration input: %s", machine.CurrentState())
		return
	}

	machine.ProcessEvent(context.Background(), fsm.EventInputDuration, nil)

	reply := tgbotapi.NewMessage(msg.Chat.ID, "Confirm chore creation? (Y/N)")
	b.api.Send(reply)
}
