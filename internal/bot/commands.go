package bot

import (
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// To preserve 100% feature parity, we delegate the commands back to the restored legacy handlers.
// The new FSM is fully available alongside it (via /chore_fsm for testing).
// Slowly, flows will be moved from legacy to FSM.

func (b *Bot) handleLegacyCommand(msg *tgbotapi.Message) {
	if b.legacyHandlers == nil {
		log.Printf("Legacy handlers not initialized. Command %s ignored.", msg.Command())
		return
	}

	var resp tgbotapi.Chattable
	var err error

	switch msg.Command() {
	case "start":
		resp, err = b.legacyHandlers.HandleStart(msg)
	case "help":
		resp, err = b.legacyHandlers.HandleHelp(msg)
	case "status":
		resp, err = b.legacyHandlers.HandleStatus(msg)
	case "schedule":
		resp, err = b.legacyHandlers.HandleSchedule(msg)
	case "volunteer":
		resp, err = b.legacyHandlers.HandleVolunteer(msg)
	case "explain":
		resp, err = b.legacyHandlers.HandleExplain(msg)
	case "chore":
		resp, err = b.legacyHandlers.HandleChore(msg)
	case "overdue":
		resp, err = b.legacyHandlers.HandleOverdue(msg)
	case "chore_stats":
		resp, err = b.legacyHandlers.HandleChoreStats(msg)
	case "assign":
		resp, err = b.legacyHandlers.HandleAssign(msg)
	case "list":
		resp, err = b.legacyHandlers.HandleList(msg)
	case "cancel":
		resp, err = b.legacyHandlers.HandleCancel(msg)
	case "edit":
		resp, err = b.legacyHandlers.HandleEdit(msg)
	case "unassign":
		resp, err = b.legacyHandlers.HandleUnassign(msg)
	case "modify":
		resp, err = b.legacyHandlers.HandleModify(msg)
	case "change":
		resp, err = b.legacyHandlers.HandleChange(msg)
	case "offduty":
		resp, err = b.legacyHandlers.HandleOffDuty(msg)
	case "users":
		resp, err = b.legacyHandlers.HandleUsers(msg)
	case "ratings":
		resp, err = b.legacyHandlers.HandleRatingsCalendar(msg)
	case "vacation":
		resp, err = b.legacyHandlers.HandleVacation(msg)
	case "toggle_active", "toggleactive":
		resp, err = b.legacyHandlers.HandleToggleActive(msg)
	case "complete":
		resp, err = b.legacyHandlers.HandleComplete(msg)
	default:
		resp = tgbotapi.NewMessage(msg.Chat.ID, "Unknown command. Use /help for a list of commands.")
	}

	if err != nil {
		log.Printf("Error in command %s: %v", msg.Command(), err)
	}

	if resp != nil {
		b.api.Send(resp)
	}
}
