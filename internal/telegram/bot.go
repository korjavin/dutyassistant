package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/telegram/handlers"
	"github.com/korjavin/dutyassistant/internal/telegram/keyboard"
)

// Bot represents the Telegram bot application.
type Bot struct {
	api      *tgbotapi.BotAPI
	handlers *handlers.Handlers
	groupID  int64 // DISH_GROUP ID for access control
	ownerID  int64 // Owner ID for access control
}

// NewBot creates a new Bot instance.
func NewBot(apiToken string, h *handlers.Handlers, groupID, ownerID int64) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(apiToken)
	if err != nil {
		return nil, err
	}
	api.Debug = false // Set to true for verbose logging
	slog.Info(fmt.Sprintf("Authorized on account %s", api.Self.UserName))

	// Inject bot API into handlers for notifications
	h.SetBot(api)

	b := &Bot{
		api:      api,
		handlers: h,
		groupID:  groupID,
		ownerID:  ownerID,
	}

	b.registerCommands()

	return b, nil
}

// registerCommands registers bot commands with Telegram for autocomplete.
func (b *Bot) registerCommands() {
	if b.api == nil {
		return
	}

	// Base user commands
	userCommands := []tgbotapi.BotCommand{
		{Command: "start", Description: "Register with the bot"},
		{Command: "help", Description: "Show available commands"},
		{Command: "status", Description: "View your duty statistics and queue status"},
		{Command: "schedule", Description: "View the current month's duty schedule"},
		{Command: "volunteer", Description: "Volunteer for duty"},
		{Command: "explain", Description: "Explain how the most recent dish hero duty was assigned"},
		{Command: "chores", Description: "View your active chores"},
	}

	userConfig := tgbotapi.NewSetMyCommands(userCommands...)
	if _, err := b.api.Request(userConfig); err != nil {
		slog.Error(fmt.Sprintf("Failed to set default bot commands: %v", err))
	} else {
		slog.Info("Successfully registered default bot commands")
	}

	// Admin commands (scoped to owner if configured)
	if b.ownerID != 0 {
		adminCommands := []tgbotapi.BotCommand{
			{Command: "chores", Description: "Chore management menu"},
			{Command: "newchore", Description: "Create a new chore interactively"},
			{Command: "editchore", Description: "Edit a chore description"},
			{Command: "translate", Description: "Translate a chore description to English"},
			{Command: "stats", Description: "Show chore statistics"},
			{Command: "activate", Description: "Toggle user active/inactive status"},
			{Command: "assign", Description: "Assign days to a user's admin queue"},
			{Command: "unassign", Description: "Remove days from a user's admin queue"},
			{Command: "cancel", Description: "Cancel a duty, active chore, or recurring chore"},
			{Command: "change", Description: "Change duty assignment for a date"},
			{Command: "offduty", Description: "Set off-duty period for a user"},
			{Command: "vacation", Description: "Toggle vacation mode to pause all duty assignments"},
			{Command: "users", Description: "List all users with their queues and status"},
			{Command: "ratings", Description: "Show the current month's participant rating calendar"},
			{Command: "complete", Description: "Mark an active chore as completed"},
			{Command: "overdue", Description: "Check for overdue chores"},
		}

		// Prepend base commands so admins get both
		fullAdminCommands := append(userCommands, adminCommands...)

		adminScope := tgbotapi.NewBotCommandScopeChat(b.ownerID)
		adminConfig := tgbotapi.NewSetMyCommandsWithScopeAndLanguage(adminScope, "", fullAdminCommands...)
		if _, err := b.api.Request(adminConfig); err != nil {
			slog.Error(fmt.Sprintf("Failed to set admin bot commands for user %d: %v", b.ownerID, err))
		} else {
			slog.Info(fmt.Sprintf("Successfully registered admin bot commands for user %d", b.ownerID))
		}
	}
}

// SendMessage sends a text message to a specific chat ID.
func (b *Bot) SendMessage(chatID int64, text string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	_, err := b.api.Send(msg)
	return err
}

// SendMessageHTML sends a text message with HTML formatting to a specific chat ID.
func (b *Bot) SendMessageHTML(chatID int64, text string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	_, err := b.api.Send(msg)
	return err
}

// SendMessageMarkdown sends a text message with MarkdownV2 formatting to a specific chat ID.
func (b *Bot) SendMessageMarkdown(chatID int64, text string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeMarkdownV2
	_, err := b.api.Send(msg)
	return err
}

// checkAccess verifies if a user has access to the bot.
// Returns true if the user is the owner or a member of the DISH_GROUP.
func (b *Bot) checkAccess(userID int64) bool {
	// Owner always has access
	if b.ownerID != 0 && userID == b.ownerID {
		slog.Info(fmt.Sprintf("User %d granted access as owner", userID), slog.String("component", "access"))
		return true
	}

	// If no group is configured, allow access
	if b.groupID == 0 {
		slog.Info(fmt.Sprintf("User %d granted access (no group restriction)", userID), slog.String("component", "access"))
		return true
	}

	// Check if user is a member of the group
	slog.Info(fmt.Sprintf("Checking group membership for user %d in group %d", userID, b.groupID), slog.String("component", "access"))
	chatMember, err := b.api.GetChatMember(tgbotapi.GetChatMemberConfig{
		ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
			ChatID: b.groupID,
			UserID: userID,
		},
	})
	if err != nil {
		slog.Info(fmt.Sprintf("Error checking group membership for user %d: %v", userID, err), slog.String("component", "access"))
		return false
	}

	// Allow if user is a member, administrator, or creator
	status := chatMember.Status
	allowed := status == "member" || status == "administrator" || status == "creator"
	slog.Info(fmt.Sprintf("User %d status in group: %s, access granted: %v", userID, status, allowed), slog.String("component", "access"))
	return allowed
}

// Start begins listening for and processing updates from Telegram.
func (b *Bot) Start(ctx context.Context) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	for {
		select {
		case update := <-updates:
			b.handleUpdate(update)
		case <-ctx.Done():
			return
		}
	}
}

// handleUpdate is the central dispatcher for all incoming updates.
func (b *Bot) handleUpdate(update tgbotapi.Update) {
	var err error
	var response tgbotapi.Chattable

	// Check access control for messages and callbacks
	var userID int64
	var chatID int64
	if update.Message != nil {
		userID = update.Message.From.ID
		chatID = update.Message.Chat.ID
	} else if update.CallbackQuery != nil {
		userID = update.CallbackQuery.From.ID
		chatID = update.CallbackQuery.Message.Chat.ID
	}

	// Verify user has access
	if userID != 0 && !b.checkAccess(userID) {
		slog.Info(fmt.Sprintf("Access denied for user %d", userID))
		ownerMention := ""
		if b.ownerID != 0 {
			ownerMention = fmt.Sprintf(" Please contact the bot owner (ID: %d) for access.", b.ownerID)
		}
		response = tgbotapi.NewMessage(chatID, fmt.Sprintf("🚫 Access denied. You must be a member of the authorized group to use this bot.%s", ownerMention))
		if _, err := b.api.Send(response); err != nil {
			slog.Error(fmt.Sprintf("Error sending access denied message: %v", err))
		}
		return
	}

	switch {
	case update.Message != nil:
		// Check if the user is in an active interactive session
		session, inSession := b.handlers.SessionManager.GetSession(chatID)

		// A bare cancel command has no arguments
		isBareCancel := update.Message.IsCommand() && update.Message.Command() == "cancel" && strings.TrimSpace(update.Message.CommandArguments()) == ""
		// Ensure the session belongs to the user who sent the message
		isSessionOwner := inSession && session.UserID == userID

		// If in a session, intercept bare /cancel to allow the session to abort gracefully
		if isSessionOwner && isBareCancel {
			response, err = b.handleMessage(update.Message)
		} else if update.Message.IsCommand() {
			response, err = b.handleCommand(update.Message)
		} else {
			// Handle non-command messages (e.g., for interactive sessions)
			response, err = b.handleMessage(update.Message)
		}
	case update.CallbackQuery != nil:
		response, err = b.handleCallbackQuery(update.CallbackQuery)
	}

	if err != nil {
		slog.Error(fmt.Sprintf("Error handling update: %v", err))
		var chatID int64
		if update.Message != nil {
			chatID = update.Message.Chat.ID
		} else if update.CallbackQuery != nil {
			chatID = update.CallbackQuery.Message.Chat.ID
		}
		if chatID != 0 {
			response = tgbotapi.NewMessage(chatID, "An unexpected error occurred. Please try again.")
		} else {
			response = nil
		}
	}

	if response != nil {
		if _, err := b.api.Send(response); err != nil {
			slog.Error(fmt.Sprintf("Error sending response: %v", err))
		}
	}
}

// handleCommand routes a command to the appropriate handler.
func (b *Bot) handleCommand(m *tgbotapi.Message) (tgbotapi.Chattable, error) {
	switch m.Command() {
	case "start":
		return b.handlers.HandleStart(m)
	case "help":
		return b.handlers.HandleHelp(m)
	case "status":
		return b.handlers.HandleStatus(m)
	case "schedule":
		return b.handlers.HandleSchedule(m)
	case "volunteer":
		return b.handlers.HandleVolunteer(m)
	case "explain":
		return b.handlers.HandleExplain(m)
	case "chores":
		return b.handlers.HandleChore(m)
	case "newchore":
		return b.handlers.HandleChore(m)
	case "translate":
		return b.handlers.HandleChoreTranslate(m)
	case "overdue":
		return b.handlers.HandleOverdue(m)
	case "stats":
		return b.handlers.HandleChoreStats(m)
	case "assign":
		return b.handlers.HandleAssign(m)
	case "cancel":
		return b.handlers.HandleCancel(m)
	case "editchore":
		return b.handlers.HandleEdit(m)
	case "unassign":
		return b.handlers.HandleUnassign(m)
	case "change":
		return b.handlers.HandleChange(m)
	case "offduty":
		return b.handlers.HandleOffDuty(m)
	case "users":
		return b.handlers.HandleUsers(m)
	case "ratings":
		return b.handlers.HandleRatingsCalendar(m)
	case "vacation":
		return b.handlers.HandleVacation(m)
	case "activate":
		return b.handlers.HandleToggleActive(m)
	case "complete":
		return b.handlers.HandleComplete(m)
	default:
		msg := tgbotapi.NewMessage(m.Chat.ID, "Unknown command. Use /help for a list of commands.")
		return msg, nil
	}
}

// handleCallbackQuery routes a callback query to the appropriate handler.
func (b *Bot) handleCallbackQuery(q *tgbotapi.CallbackQuery) (tgbotapi.Chattable, error) {
	// Answer the callback query to remove the "loading" state on the user's side.
	callback := tgbotapi.NewCallback(q.ID, "")
	if _, err := b.api.Request(callback); err != nil {
		slog.Error(fmt.Sprintf("failed to answer callback query: %v", err))
	}

	action := strings.Split(q.Data, ":")[0]

	switch action {
	case keyboard.ActionPrevMonth, keyboard.ActionNextMonth:
		// Calendar navigation for /schedule command
		return b.handlers.HandleCalendarCallback(q)
	case keyboard.ActionSelectDay:
		// /schedule is read-only, do nothing on day selection
		return nil, nil
	case keyboard.ActionIgnore:
		return nil, nil // Do nothing for ignore actions
	case "assign_user":
		return b.handlers.HandleAssignUserCallback(q)
	case "assign_days":
		return b.handlers.HandleAssignDaysCallback(q)
	case "assign_custom":
		return b.handlers.HandleAssignCustomCallback(q)
	case "unassign_user":
		return b.handlers.HandleUnassignUserCallback(q)
	case "unassign_days":
		return b.handlers.HandleUnassignDaysCallback(q)
	case "volunteer_days":
		return b.handlers.HandleVolunteerDaysCallback(q)
	case "volunteer_custom":
		return b.handlers.HandleVolunteerCustomCallback(q)
	case "modify_date":
		return b.handlers.HandleModifyDateCallback(q)
	case "modify_user":
		return b.handlers.HandleModifyUserCallback(q)
	case "toggle_user":
		return b.handlers.HandleToggleUserCallback(q)
	case "offduty_user":
		return b.handlers.HandleOffDutyUserCallback(q)
	case "vacation":
		return b.handlers.HandleVacationCallback(q)
	case "chore_done":
		return b.handlers.HandleChoreDoneCallback(q)
	case "chore_remind":
		return b.handlers.HandleChoreRemindCallback(q)
	case "complete_chore":
		return b.handlers.HandleCompleteChoreCallback(q)
	case "edit_chore":
		return b.handlers.HandleEditChoreCallback(q)
	case "chore_action":
		return b.handlers.HandleChoreActionCallback(q)
	case "chore_delete":
		return b.handlers.HandleChoreDeleteCallback(q)
	case "chore_delete_confirm":
		return b.handlers.HandleChoreDeleteConfirmCallback(q)
	case "cancel_assignment":
		return b.handlers.HandleCancelAssignmentCallback(q)
	case "cancel_assignment_confirm":
		return b.handlers.HandleCancelAssignmentConfirmCallback(q)
	case "list":
		return b.handlers.HandleListCallback(q)
	case "cancel_flow":
		return b.handlers.HandleCancelFlow(q)
	default:
		slog.Info(fmt.Sprintf("Unknown callback action: %s", action))
		return nil, nil
	}
}

// handleMessage handles non-command messages (for interactive sessions)
func (b *Bot) handleMessage(m *tgbotapi.Message) (tgbotapi.Chattable, error) {
	// Check if user is in an interactive session
	session, exists := b.handlers.SessionManager.GetSession(m.Chat.ID)
	if !exists {
		// No active session, ignore the message
		return nil, nil
	}

	// Route to appropriate handler based on session type
	switch session.Type {
	case handlers.SessionTypeChoreCreation:
		return b.handlers.HandleChoreInteractive(m)
	case handlers.SessionTypeDailyRatings:
		return b.handlers.HandleDailyRatingsInteractive(m)
	case handlers.SessionTypeEditChore:
		return b.handlers.HandleEditChoreInteractive(m)
	default:
		slog.Info(fmt.Sprintf("Unknown session type: %s", session.Type))
		return nil, nil
	}
}

// API returns the underlying Telegram Bot API instance.
func (b *Bot) API() *tgbotapi.BotAPI {
	return b.api
}
