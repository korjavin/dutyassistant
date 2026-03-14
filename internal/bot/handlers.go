package bot

import (
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/bot/fsm"
)

func (b *Bot) handleCommand(msg *tgbotapi.Message) {
	switch msg.Command() {
	case "chore":
		b.handleChoreCommand(msg)
	case "cancel":
		b.handleCancelCommand(msg)
	case "start", "help", "status", "schedule", "volunteer", "assign", "modify", "change", "offduty", "toggleactive", "unassign", "vacation", "users", "ratings":
		// These commands are recognized to avoid unhandled errors but their implementation is migrating
		reply := tgbotapi.NewMessage(msg.Chat.ID, "This command is currently being migrated to the new FSM architecture.")
		b.api.Send(reply)
	default:
		log.Printf("Unknown command: %s", msg.Command())
	}
}

func (b *Bot) processFSMState(msg *tgbotapi.Message, currentState fsm.State) {
	// Central routing for FSM-based interactive sessions
	switch currentState {
	case fsm.StateChoreDesc:
		b.handleChoreDescriptionInput(msg)
	case fsm.StateChoreDuration:
		b.handleChoreDurationInput(msg)
	case fsm.StateRatingPrompt:
		b.handleRatingPromptInput(msg)
	default:
		log.Printf("Unhandled state: %s", currentState)
	}
}
