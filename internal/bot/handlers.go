package bot

import (
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/bot/fsm"
	"github.com/korjavin/dutyassistant/internal/telegram/handlers"
)

func (b *Bot) handleCommand(msg *tgbotapi.Message) {
	// Delegate specific FSM logic
	switch msg.Command() {
	case "chore_fsm":
		// Example parallel command to test FSM without breaking real /chore
		b.handleChoreCommand(msg)
		return
	}

	// Delegate all registered legacy commands to the original dispatcher
	b.handleLegacyCommand(msg)
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

// Handle non-command messages that belong to legacy sessions
func (b *Bot) handleLegacyMessage(msg *tgbotapi.Message) {
	if b.legacyHandlers == nil {
		return
	}

	// Check if user is in an interactive session
	session, exists := b.legacyHandlers.SessionManager.GetSession(msg.Chat.ID)
	if !exists {
		// No active session, ignore the message
		return
	}

	var resp tgbotapi.Chattable
	switch session.Type {
	case handlers.SessionTypeChoreCreation:
		resp, _ = b.legacyHandlers.HandleChoreInteractive(msg)
	case handlers.SessionTypeDailyRatings:
		resp, _ = b.legacyHandlers.HandleDailyRatingsInteractive(msg)
	case handlers.SessionTypeEditChore:
		resp, _ = b.legacyHandlers.HandleEditChoreInteractive(msg)
	}

	if resp != nil {
		b.api.Send(resp)
	}
}

func (b *Bot) handleLegacyCallback(query *tgbotapi.CallbackQuery) {
	if b.legacyHandlers == nil {
		return
	}
	// Answer the callback query to remove the "loading" state on the user's side.
	callback := tgbotapi.NewCallback(query.ID, "")
	b.api.Request(callback)

	action := strings.Split(query.Data, ":")[0]

	var resp tgbotapi.Chattable
	switch action {
	case "prev_month", "next_month": // keyboard.ActionPrevMonth, keyboard.ActionNextMonth
		resp, _ = b.legacyHandlers.HandleCalendarCallback(query)
	case "assign_user":
		resp, _ = b.legacyHandlers.HandleAssignUserCallback(query)
	case "assign_days":
		resp, _ = b.legacyHandlers.HandleAssignDaysCallback(query)
	case "assign_custom":
		resp, _ = b.legacyHandlers.HandleAssignCustomCallback(query)
	case "unassign_user":
		resp, _ = b.legacyHandlers.HandleUnassignUserCallback(query)
	case "unassign_days":
		resp, _ = b.legacyHandlers.HandleUnassignDaysCallback(query)
	case "volunteer_days":
		resp, _ = b.legacyHandlers.HandleVolunteerDaysCallback(query)
	case "volunteer_custom":
		resp, _ = b.legacyHandlers.HandleVolunteerCustomCallback(query)
	case "modify_date":
		resp, _ = b.legacyHandlers.HandleModifyDateCallback(query)
	case "modify_user":
		resp, _ = b.legacyHandlers.HandleModifyUserCallback(query)
	case "toggle_user":
		resp, _ = b.legacyHandlers.HandleToggleUserCallback(query)
	case "offduty_user":
		resp, _ = b.legacyHandlers.HandleOffDutyUserCallback(query)
	case "vacation":
		resp, _ = b.legacyHandlers.HandleVacationCallback(query)
	case "chore_done":
		resp, _ = b.legacyHandlers.HandleChoreDoneCallback(query)
	case "chore_remind":
		resp, _ = b.legacyHandlers.HandleChoreRemindCallback(query)
	case "complete_chore":
		resp, _ = b.legacyHandlers.HandleCompleteChoreCallback(query)
	case "edit_chore":
		resp, _ = b.legacyHandlers.HandleEditChoreCallback(query)
	case "chore_action":
		resp, _ = b.legacyHandlers.HandleChoreActionCallback(query)
	case "chore_delete":
		resp, _ = b.legacyHandlers.HandleChoreDeleteCallback(query)
	case "chore_delete_confirm":
		resp, _ = b.legacyHandlers.HandleChoreDeleteConfirmCallback(query)
	case "cancel_assignment":
		resp, _ = b.legacyHandlers.HandleCancelAssignmentCallback(query)
	case "cancel_assignment_confirm":
		resp, _ = b.legacyHandlers.HandleCancelAssignmentConfirmCallback(query)
	case "list":
		resp, _ = b.legacyHandlers.HandleListCallback(query)
	case "cancel_flow":
		resp, _ = b.legacyHandlers.HandleCancelFlow(query)
	}

	if resp != nil {
		b.api.Send(resp)
	}
}
