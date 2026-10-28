package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/assistant"
	"github.com/korjavin/dutyassistant/internal/telegram/handlers"
	"github.com/korjavin/dutyassistant/internal/telegram/keyboard"
)

// Bot represents the Telegram bot application.
type Bot struct {
	api      *tgbotapi.BotAPI
	handlers *handlers.Handlers
	groupID  int64         // DISH_GROUP ID for access control
	ownerID  int64         // Owner ID for access control
	self     tgbotapi.User // the bot's own account, to spot mentions in the group
	// migratedTo is the new DISH_GROUP ID already reported to the admin.
	migratedTo int64

	// mediaLocks keeps voice/image messages of one chat in order (chat ID -> *sync.Mutex).
	mediaLocks sync.Map

	// Free-text assistants for the admin and for participants; nil when disabled.
	adminAssistant       *assistant.Agent
	participantAssistant *assistant.Agent
}

// NewBot creates a new Bot instance.
func NewBot(apiToken string, h *handlers.Handlers, groupID, ownerID int64) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(apiToken)
	if err != nil {
		return nil, err
	}
	api.Debug = false // Set to true for verbose logging
	slog.Info(fmt.Sprintf("Authorized on account %s", api.Self.UserName))

	registerCommands(api, ownerID, groupID)

	// Inject bot API into handlers for notifications
	h.SetBot(api)

	return &Bot{
		api:      api,
		handlers: h,
		groupID:  groupID,
		ownerID:  ownerID,
		self:     api.Self,
	}, nil
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
// chatID is where the message or button is; whoever writes in DISH_GROUP
// itself is a member.
func (b *Bot) checkAccess(userID, chatID int64) bool {
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

	if chatID == b.groupID {
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
		var apiErr *tgbotapi.Error
		if errors.As(err, &apiErr) && apiErr.MigrateToChatID != 0 {
			b.reportGroupMigration(apiErr.MigrateToChatID)
		}
		return false
	}

	// Allow if user is a member, administrator, or creator (or a member
	// with restrictions)
	status := chatMember.Status
	allowed := status == "member" || status == "administrator" || status == "creator" ||
		(status == "restricted" && chatMember.IsMember)
	slog.Info(fmt.Sprintf("User %d status in group: %s, access granted: %v", userID, status, allowed), slog.String("component", "access"))
	return allowed
}

// addressedToBot reports whether a message may need the bot's attention:
// anything in a private chat, a command, a mention of the bot, a reply to
// one of the bot's messages, or text for the sender's own interactive
// session in that chat.
func (b *Bot) addressedToBot(m *tgbotapi.Message) bool {
	if m.Chat == nil || m.Chat.IsPrivate() || m.IsCommand() || b.mentionsBot(m) || b.repliesToBot(m) {
		return true
	}
	session, ok := b.handlers.SessionManager.GetSession(m.Chat.ID)
	return ok && m.From != nil && session.UserID == m.From.ID
}

// handleGroupMigration handles the service messages Telegram sends when a
// basic group is upgraded to a supergroup (it gets a new ID; this happens
// e.g. when a bot is made admin). It returns true for such messages.
func (b *Bot) handleGroupMigration(m *tgbotapi.Message) bool {
	var oldID, newID int64
	switch {
	case m.MigrateToChatID != 0:
		oldID, newID = m.Chat.ID, m.MigrateToChatID
	case m.MigrateFromChatID != 0:
		oldID, newID = m.MigrateFromChatID, m.Chat.ID
	default:
		return false
	}
	if b.groupID != 0 && oldID == b.groupID {
		b.reportGroupMigration(newID)
	}
	return true
}

// reportGroupMigration tells the admin (once) that DISH_GROUP is outdated.
func (b *Bot) reportGroupMigration(newID int64) {
	if b.migratedTo == newID {
		return
	}
	b.migratedTo = newID
	text := fmt.Sprintf("⚠️ The group DISH_GROUP=%d was upgraded to a supergroup by Telegram and now has the ID %d. "+
		"Set DISH_GROUP=%d and restart the bot; until then group messages and access checks fail.", b.groupID, newID, newID)
	slog.Error(text, slog.String("component", "access"))
	if b.ownerID != 0 && b.api != nil {
		if _, err := b.api.Send(tgbotapi.NewMessage(b.ownerID, text)); err != nil {
			slog.Error(fmt.Sprintf("Error notifying admin about the group migration: %v", err))
		}
	}
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

	if update.Message != nil {
		update.Message = withCaptionAsText(update.Message)
	}
	if m := update.Message; m != nil {
		if b.handleGroupMigration(m) {
			return
		}
		// Ordinary chatter in a group is not for the bot: no access check,
		// no reply.
		if !b.addressedToBot(m) {
			return
		}
	}

	// Verify user has access
	if userID != 0 && !b.checkAccess(userID, chatID) {
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
		if hasMedia(update.Message) && !update.Message.IsCommand() {
			// Transcribed or described in the background, then handled like typed text.
			b.handleMedia(update.Message)
			return
		}
		response, err = b.routeMessage(update.Message)
	case update.CallbackQuery != nil:
		response, err = b.handleCallbackQuery(update.CallbackQuery)
	}

	b.respond(chatID, response, err)
}

// routeMessage sends a message to its session, command or free-text handler.
func (b *Bot) routeMessage(m *tgbotapi.Message) (tgbotapi.Chattable, error) {
	// Check if the user is in an active interactive session
	session, inSession := b.handlers.SessionManager.GetSession(m.Chat.ID)

	// A bare cancel command has no arguments
	isBareCancel := m.IsCommand() && m.Command() == "cancel" && strings.TrimSpace(m.CommandArguments()) == ""
	// Ensure the session belongs to the user who sent the message
	isSessionOwner := inSession && session.UserID == m.From.ID

	// If in a session, intercept bare /cancel to allow the session to abort gracefully
	if isSessionOwner && isBareCancel {
		return b.handleMessage(m)
	} else if m.IsCommand() {
		return b.handleCommand(m)
	}
	// Handle non-command messages (e.g., for interactive sessions)
	return b.handleMessage(m)
}

// respond sends a handler's response, or a generic error message to chatID.
func (b *Bot) respond(chatID int64, response tgbotapi.Chattable, err error) {
	if err != nil {
		slog.Error(fmt.Sprintf("Error handling update: %v", err))
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
	case "takechore":
		return b.handlers.HandleVolunteerChore(m)
	case "explain":
		return b.handlers.HandleExplain(m)
	case "chore":
		return b.handlers.HandleChore(m)
	case "overdue":
		return b.handlers.HandleOverdue(m)
	case "chore_stats":
		return b.handlers.HandleChoreStats(m)
	case "assign":
		return b.handlers.HandleAssign(m)
	case "list":
		return b.handlers.HandleList(m)
	case "cancel":
		return b.handlers.HandleCancel(m)
	case "edit":
		return b.handlers.HandleEdit(m)
	case "unassign":
		return b.handlers.HandleUnassign(m)
	case "modify":
		return b.handlers.HandleModify(m)
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
	case "toggle_active", "toggleactive":
		return b.handlers.HandleToggleActive(m)
	case "complete":
		return b.handlers.HandleComplete(m)
	case "sviniya":
		return b.handlers.HandleSviniya(m)
	case "set_volunteer":
		return b.handlers.HandleSetVolunteer(m)
	case "set_sviniya_balance":
		return b.handlers.HandleSetSviniyaBalance(m)
	case "spend":
		return b.handlers.HandleSpend(m)
	case "text_candidate_chore":
		return b.handlers.HandleTextCandidateChore(m)
	case "report":
		return b.handlers.HandleReport(m)
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

	if b.adminAssistant.OwnsCallback(q.Data) || b.participantAssistant.OwnsCallback(q.Data) {
		return b.handleAssistantCallback(q)
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
	case "vc_select":
		return b.handlers.HandleVolunteerChoreSelectCallback(q)
	case "vc_confirm":
		return b.handlers.HandleVolunteerChoreConfirmCallback(q)
	case "vc_cancel":
		return b.handlers.HandleVolunteerChoreCancelCallback(q)
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
	case "chore_list_done":
		return b.handlers.HandleChoreListDoneCallback(q)
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
	case "reg_ok", "reg_no":
		return b.handlers.HandleRegistrationCallback(q)
	case "rep_done", "rep_ok", "rep_set", "rep_no":
		return b.handlers.HandleReportCallback(q)
	default:
		slog.Info(fmt.Sprintf("Unknown callback action: %s", action))
		return nil, nil
	}
}

// handleMessage handles non-command messages (for interactive sessions)
func (b *Bot) handleMessage(m *tgbotapi.Message) (tgbotapi.Chattable, error) {
	// Check if user is in an interactive session
	session, exists := b.handlers.SessionManager.GetSession(m.Chat.ID)
	// A mention of the bot in the group is for the assistant unless the
	// sender is in the middle of their own session there.
	if exists && session.UserID != m.From.ID && b.isGroupMention(m) {
		exists = false
	}
	if !exists {
		// No active session: a participant's voice message, or text while
		// their daily report is open, is part of the report.
		if resp, handled := b.handlers.HandleDailyReportMessage(m); handled {
			return resp, nil
		}
		// Otherwise free text in a private chat, or a mention of the bot in
		// the group, goes to the assistant (admin or participant); anything
		// else is ignored.
		if resp, handled := b.handleAssistantMessage(m); handled {
			return resp, nil
		}
		return nil, nil
	}

	// Route to appropriate handler based on session type
	switch session.Type {
	case handlers.SessionTypeChoreCreation:
		return b.handlers.HandleChoreInteractive(m)
	case handlers.SessionTypeEditChore:
		return b.handlers.HandleEditChoreInteractive(m)
	case handlers.SessionTypeSpendSviniya:
		return b.handlers.HandleSpendInteractive(m)
	default:
		slog.Info(fmt.Sprintf("Unknown session type: %s", session.Type))
		return nil, nil
	}
}

// API returns the underlying Telegram Bot API instance.
func (b *Bot) API() *tgbotapi.BotAPI {
	return b.api
}
