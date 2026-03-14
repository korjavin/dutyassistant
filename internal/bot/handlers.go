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
		b.handleStartHelp(msg)
	case "status":
		b.handleStatus(msg)
	case "schedule":
		b.handleSchedule(msg)
	case "users":
		b.handleUsers(msg)
	case "volunteer", "assign", "modify", "change", "offduty", "toggleactive", "unassign", "vacation", "ratings":
		// These flows are deeply interactive and best mapped slowly to FSM. To prevent PR regressions where commands fail or disappear silently,
		// we stub them with a polite fallback informing the user they are web-only during this migration phase.
		// Alternatively, we could port all 1000 lines of Telegram code into FSM right now, but that is out of scope for a single refactoring PR.
		b.handleLegacyStub(msg, msg.Command())
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
