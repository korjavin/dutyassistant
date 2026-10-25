package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/anthropics/anthropic-sdk-go"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/assistant"
	"github.com/korjavin/dutyassistant/internal/store"
)

const (
	assistantUnavailableMessage = "💡 Free-text admin requests need the Anthropic LLM provider (set ANTHROPIC_API_KEY). Use /help for the list of commands."
	assistantNotRegistered      = "👋 Please send /start first so I know who you are."
)

// AssistantConfig configures the free-text assistants.
type AssistantConfig struct {
	Client   *anthropic.Client
	Model    string
	Effort   anthropic.OutputConfigEffort
	Store    store.Store
	Location *time.Location
	// ParticipantDailyLimit caps participants' messages per day; 0 disables
	// the participant assistant.
	ParticipantDailyLimit int
}

// EnableAssistants turns on free-text requests handled by Claude: for the
// admin (full household management) and, when ParticipantDailyLimit > 0,
// for participants (their own chores, duties and sviniyas).
func (b *Bot) EnableAssistants(cfg AssistantConfig) {
	base := assistant.Config{
		Client:   cfg.Client,
		Model:    cfg.Model,
		Effort:   cfg.Effort,
		Store:    cfg.Store,
		Executor: assistantExecutor{b: b},
		Sender:   apiSender{api: b.api},
		Location: cfg.Location,
	}
	admin := base
	admin.Profile = assistant.AdminProfile()
	b.adminAssistant = assistant.New(admin)

	if cfg.ParticipantDailyLimit > 0 {
		participant := base
		participant.Profile = assistant.ParticipantProfile()
		participant.DailyLimit = cfg.ParticipantDailyLimit
		b.participantAssistant = assistant.New(participant)
	}
}

// handleAssistantMessage routes free text from a private chat, and messages
// mentioning the bot in the group, to the admin or participant assistant
// depending on who wrote them. It returns false when the message is not for them.
func (b *Bot) handleAssistantMessage(m *tgbotapi.Message) (tgbotapi.Chattable, bool) {
	if m.Chat == nil || m.From == nil {
		return nil, false
	}
	text, group := m.Text, false
	if !m.Chat.IsPrivate() {
		if !b.isGroupMention(m) {
			// The admin mentioning the bot elsewhere most likely means
			// DISH_GROUP points to another chat; tell them which ID this is.
			if b.mentionsBot(m) && b.handlers.IsAdmin(m.From.ID) {
				msg := tgbotapi.NewMessage(m.Chat.ID, b.wrongGroupMessage(m.Chat.ID))
				msg.ReplyToMessageID = m.MessageID
				return msg, true
			}
			return nil, false
		}
		text, group = b.stripMentions(m), true
	}
	if strings.TrimSpace(text) == "" {
		return nil, false
	}
	reply := func(s string) tgbotapi.Chattable {
		msg := tgbotapi.NewMessage(m.Chat.ID, s)
		if group {
			msg.ReplyToMessageID = m.MessageID
		}
		return msg
	}

	agent, role := b.adminAssistant, "admin"
	var user *store.User
	if b.handlers.IsAdmin(m.From.ID) {
		if agent == nil {
			return reply(assistantUnavailableMessage), true
		}
	} else {
		agent, role = b.participantAssistant, "participant"
		if agent == nil {
			return nil, false
		}
		var err error
		user, err = b.handlers.Store.GetUserByTelegramID(context.Background(), m.From.ID)
		if err != nil || user == nil {
			return reply(assistantNotRegistered), true
		}
	}

	if !group {
		slog.Info(fmt.Sprintf("[Assistant] Free-text request from %s %d", role, m.From.ID))
		// The model can take a while; don't block other updates.
		go agent.HandleMessage(context.Background(), m.Chat.ID, m.From.ID, text)
		return nil, true
	}

	name := m.From.FirstName
	if user != nil && user.FirstName != "" {
		name = user.FirstName
	}
	slog.Info(fmt.Sprintf("[Assistant] Group mention from %s %d", role, m.From.ID))
	go agent.HandleGroupMessage(context.Background(), m.Chat.ID, m.From.ID, m.MessageID, name, text)
	return nil, true
}

// isGroupMention reports whether m was written in the main group and
// mentions the bot or replies to one of its messages.
func (b *Bot) isGroupMention(m *tgbotapi.Message) bool {
	return b.groupID != 0 && m.Chat != nil && m.Chat.ID == b.groupID && (b.mentionsBot(m) || b.repliesToBot(m))
}

// repliesToBot reports whether m is a reply to one of the bot's messages.
func (b *Bot) repliesToBot(m *tgbotapi.Message) bool {
	r := m.ReplyToMessage
	return r != nil && r.From != nil && b.self.ID != 0 && r.From.ID == b.self.ID &&
		m.From != nil && !m.From.IsBot
}

// mentionsBot reports whether a person's message mentions the bot
// (@username, or a mention of its account).
func (b *Bot) mentionsBot(m *tgbotapi.Message) bool {
	if m.From == nil || m.From.IsBot || b.self.ID == 0 {
		return false
	}
	return len(b.mentionEntities(m)) > 0
}

// wrongGroupMessage explains to the admin why a mention in chatID is ignored.
func (b *Bot) wrongGroupMessage(chatID int64) string {
	configured := "DISH_GROUP is not set"
	if b.groupID != 0 {
		configured = fmt.Sprintf("DISH_GROUP=%d", b.groupID)
	}
	return fmt.Sprintf("ℹ️ I answer mentions only in the household group (%s). This chat's ID is %d: "+
		"if this is the household group, set DISH_GROUP=%d and restart the bot.", configured, chatID, chatID)
}

// mentionEntities returns the entities in m that mention the bot.
func (b *Bot) mentionEntities(m *tgbotapi.Message) []tgbotapi.MessageEntity {
	text := utf16.Encode([]rune(m.Text))
	var found []tgbotapi.MessageEntity
	for _, e := range m.Entities {
		if e.Offset < 0 || e.Length <= 0 || e.Offset+e.Length > len(text) {
			continue
		}
		switch e.Type {
		case "mention":
			name := string(utf16.Decode(text[e.Offset : e.Offset+e.Length]))
			if b.self.UserName != "" && strings.EqualFold(name, "@"+b.self.UserName) {
				found = append(found, e)
			}
		case "text_mention":
			if e.User != nil && e.User.ID == b.self.ID {
				found = append(found, e)
			}
		}
	}
	return found
}

// stripMentions returns m's text without the @username mentions of the bot.
// Text mentions (a name linked to the bot) are kept as they read as words.
func (b *Bot) stripMentions(m *tgbotapi.Message) string {
	text := utf16.Encode([]rune(m.Text))
	var out []uint16
	pos := 0
	// Entities come sorted by offset.
	for _, e := range b.mentionEntities(m) {
		if e.Type != "mention" || e.Offset < pos {
			continue
		}
		out = append(out, text[pos:e.Offset]...)
		pos = e.Offset + e.Length
	}
	out = append(out, text[pos:]...)
	return strings.Join(strings.Fields(string(utf16.Decode(out))), " ")
}

// handleAssistantCallback handles the Confirm/Cancel buttons of the assistants.
func (b *Bot) handleAssistantCallback(q *tgbotapi.CallbackQuery) (tgbotapi.Chattable, error) {
	if q.Message == nil || q.From == nil {
		return nil, nil
	}
	agent := b.participantAssistant
	if b.adminAssistant.OwnsCallback(q.Data) {
		if !b.handlers.IsAdmin(q.From.ID) {
			return nil, nil
		}
		agent = b.adminAssistant
	}
	if agent == nil {
		return nil, nil
	}
	id, approved, ok := agent.ParseCallback(q.Data)
	if !ok {
		return nil, nil
	}
	// In the group everyone sees the buttons; only the person who asked may
	// use them (and leave them in place for that person).
	if owner, known := agent.PendingOwner(q.Message.Chat.ID, id); known && owner != q.From.ID {
		return nil, nil
	}
	// Pending actions are stored per chat, so a button only acts on its own chat's plan.
	go agent.HandleDecision(context.Background(), q.Message.Chat.ID, q.From.ID, id, approved)

	// Remove the buttons so the decision cannot be made twice.
	return tgbotapi.NewEditMessageReplyMarkup(q.Message.Chat.ID, q.Message.MessageID,
		tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{}}), nil
}

// assistantExecutor runs commands and button callbacks through the bot's own
// handlers, so assistant actions behave exactly like manual ones.
type assistantExecutor struct {
	b *Bot
}

// from builds the Telegram user the bot handlers will see, with the stored
// first name since some handlers greet the user by it.
func (e assistantExecutor) from(ctx context.Context, userID int64) *tgbotapi.User {
	u := &tgbotapi.User{ID: userID}
	if user, err := e.b.handlers.Store.GetUserByTelegramID(ctx, userID); err == nil && user != nil {
		u.FirstName = user.FirstName
	}
	return u
}

func (e assistantExecutor) RunCommand(ctx context.Context, chatID, userID int64, commandLine string) (assistant.Result, error) {
	commandLine = strings.TrimSpace(commandLine)
	if !strings.HasPrefix(commandLine, "/") {
		return assistant.Result{}, fmt.Errorf("not a command: %q", commandLine)
	}
	commandLen := len(strings.Fields(commandLine)[0])
	m := &tgbotapi.Message{
		Text:     commandLine,
		Chat:     &tgbotapi.Chat{ID: chatID, Type: "private"},
		From:     e.from(ctx, userID),
		Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: commandLen}},
	}
	resp, err := e.b.handleCommand(m)
	if err != nil {
		return assistant.Result{}, err
	}
	return resultOf(resp), nil
}

func (e assistantExecutor) RunCallback(ctx context.Context, chatID, userID int64, data string) (assistant.Result, error) {
	q := &tgbotapi.CallbackQuery{
		From:    e.from(ctx, userID),
		Data:    data,
		Message: &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: chatID, Type: "private"}},
	}
	var resp tgbotapi.Chattable
	var err error
	// Only the callbacks the assistants' tools need are allowed. Each handler
	// checks permissions itself (admin-only, or the chore's owner).
	switch action := strings.Split(data, ":")[0]; action {
	case "complete_chore":
		resp, err = e.b.handlers.HandleCompleteChoreCallback(q)
	case "cancel_assignment_confirm":
		resp, err = e.b.handlers.HandleCancelAssignmentConfirmCallback(q)
	case "chore_done":
		resp, err = e.b.handlers.HandleChoreDoneCallback(q)
	case "vc_confirm":
		resp, err = e.b.handlers.HandleVolunteerChoreConfirmCallback(q)
	default:
		return assistant.Result{}, fmt.Errorf("callback %q is not allowed for the assistant", action)
	}
	if err != nil {
		return assistant.Result{}, err
	}
	return resultOf(resp), nil
}

// resultOf extracts the text of a handler response.
func resultOf(c tgbotapi.Chattable) assistant.Result {
	switch v := c.(type) {
	case tgbotapi.MessageConfig:
		return assistant.Result{Text: v.Text, ParseMode: v.ParseMode}
	case tgbotapi.EditMessageTextConfig:
		return assistant.Result{Text: v.Text, ParseMode: v.ParseMode}
	default:
		return assistant.Result{}
	}
}

// apiSender sends Chattables (messages and chat actions) via the Bot API.
type apiSender struct {
	api *tgbotapi.BotAPI
}

func (s apiSender) Send(c tgbotapi.Chattable) error {
	_, err := s.api.Request(c)
	return err
}
