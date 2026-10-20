package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/assistant"
	"github.com/korjavin/dutyassistant/internal/store"
)

const assistantUnavailableMessage = "💡 Free-text admin requests need the Anthropic LLM provider (set ANTHROPIC_API_KEY). Use /help for the list of commands."

// EnableAdminAssistant turns on free-text admin requests handled by Claude.
func (b *Bot) EnableAdminAssistant(client *anthropic.Client, model string, effort anthropic.OutputConfigEffort, s store.Store, loc *time.Location) {
	b.assistant = assistant.New(assistant.Config{
		Client:   client,
		Model:    model,
		Effort:   effort,
		Store:    s,
		Executor: assistantExecutor{b: b},
		Sender:   apiSender{api: b.api},
		Location: loc,
	})
}

// handleAssistantMessage routes a free-text message from the admin's private
// chat to the assistant. It returns false when the message is not for it.
func (b *Bot) handleAssistantMessage(m *tgbotapi.Message) (tgbotapi.Chattable, bool) {
	if m.Chat == nil || !m.Chat.IsPrivate() || m.From == nil || strings.TrimSpace(m.Text) == "" {
		return nil, false
	}
	if !b.handlers.IsAdmin(m.From.ID) {
		return nil, false
	}
	if b.assistant == nil {
		return tgbotapi.NewMessage(m.Chat.ID, assistantUnavailableMessage), true
	}
	slog.Info(fmt.Sprintf("[Assistant] Free-text request from admin %d", m.From.ID))
	// The model can take a while; don't block other updates.
	go b.assistant.HandleMessage(context.Background(), m.Chat.ID, m.From.ID, m.Text)
	return nil, true
}

// handleAssistantCallback handles the Confirm/Cancel buttons of the assistant.
func (b *Bot) handleAssistantCallback(q *tgbotapi.CallbackQuery) (tgbotapi.Chattable, error) {
	if q.Message == nil || q.From == nil || b.assistant == nil || !b.handlers.IsAdmin(q.From.ID) {
		return nil, nil
	}
	id, approved, ok := assistant.ParseCallback(q.Data)
	if !ok {
		return nil, nil
	}
	go b.assistant.HandleDecision(context.Background(), q.Message.Chat.ID, q.From.ID, id, approved)

	// Remove the buttons so the decision cannot be made twice.
	return tgbotapi.NewEditMessageReplyMarkup(q.Message.Chat.ID, q.Message.MessageID,
		tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{}}), nil
}

// assistantExecutor runs commands and button callbacks through the bot's own
// handlers, so assistant actions behave exactly like the admin's manual ones.
type assistantExecutor struct {
	b *Bot
}

func (e assistantExecutor) RunCommand(_ context.Context, chatID, adminID int64, commandLine string) (assistant.Result, error) {
	commandLine = strings.TrimSpace(commandLine)
	if !strings.HasPrefix(commandLine, "/") {
		return assistant.Result{}, fmt.Errorf("not a command: %q", commandLine)
	}
	commandLen := len(strings.Fields(commandLine)[0])
	m := &tgbotapi.Message{
		Text:     commandLine,
		Chat:     &tgbotapi.Chat{ID: chatID, Type: "private"},
		From:     &tgbotapi.User{ID: adminID},
		Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: commandLen}},
	}
	resp, err := e.b.handleCommand(m)
	if err != nil {
		return assistant.Result{}, err
	}
	return resultOf(resp), nil
}

func (e assistantExecutor) RunCallback(_ context.Context, chatID, adminID int64, data string) (assistant.Result, error) {
	q := &tgbotapi.CallbackQuery{
		From:    &tgbotapi.User{ID: adminID},
		Data:    data,
		Message: &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: chatID, Type: "private"}},
	}
	var resp tgbotapi.Chattable
	var err error
	// Only the callbacks the assistant's tools need are allowed.
	switch action := strings.Split(data, ":")[0]; action {
	case "complete_chore":
		resp, err = e.b.handlers.HandleCompleteChoreCallback(q)
	case "cancel_assignment_confirm":
		resp, err = e.b.handlers.HandleCancelAssignmentConfirmCallback(q)
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
