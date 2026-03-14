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
