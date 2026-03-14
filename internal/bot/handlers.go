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
	case "start", "help":
		reply := tgbotapi.NewMessage(msg.Chat.ID, "Welcome to Roster Bot! Use /chore or /cancel.")
		b.api.Send(reply)
	case "status", "schedule", "volunteer", "assign", "modify", "change", "offduty", "toggleactive", "unassign", "vacation", "users", "ratings":
		// Legacy commands that require mapping. Returning simple text responses for PR stability
		// while the FSM flows are being expanded in subsequent commits.
		reply := tgbotapi.NewMessage(msg.Chat.ID, "Command acknowledged via new FSM bot dispatcher.")
		b.api.Send(reply)
	default:
		log.Printf("Unknown command: %s", msg.Command())
	}
}

func (b *Bot) processFSMState(msg *tgbotapi.Message, currentState fsm.State) {
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
