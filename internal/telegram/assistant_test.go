package telegram

import (
	"context"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/mocks"
	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/korjavin/dutyassistant/internal/telegram/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const assistantTestAdmin = int64(123)

func newAssistantTestBot() (*Bot, *mocks.MockStore) {
	mockStore := new(mocks.MockStore)
	h := handlers.NewWithAdminID(mockStore, new(mocks.MockScheduler), 0, assistantTestAdmin, nil)
	return &Bot{handlers: h, ownerID: assistantTestAdmin}, mockStore
}

// enableTestAssistants turns on both assistants with a client that is never
// called by these tests.
func enableTestAssistants(bot *Bot, mockStore *mocks.MockStore, participantLimit int) {
	client := anthropic.NewClient()
	bot.EnableAssistants(AssistantConfig{
		Client: &client, Model: "claude-opus-5-5", Store: mockStore,
		Location: time.UTC, ParticipantDailyLimit: participantLimit,
	})
}

func TestEnableAssistants_ParticipantLimitZeroDisablesParticipants(t *testing.T) {
	bot, mockStore := newAssistantTestBot()
	enableTestAssistants(bot, mockStore, 0)
	assert.NotNil(t, bot.adminAssistant)
	assert.Nil(t, bot.participantAssistant)

	enableTestAssistants(bot, mockStore, 5)
	assert.NotNil(t, bot.participantAssistant)
}

func TestHandleAssistantMessage_UnregisteredParticipant(t *testing.T) {
	bot, mockStore := newAssistantTestBot()
	enableTestAssistants(bot, mockStore, 5)
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(999)).Return(nil, nil)

	resp, handled := bot.handleAssistantMessage(&tgbotapi.Message{
		Text: "привет", From: &tgbotapi.User{ID: 999}, Chat: &tgbotapi.Chat{ID: 999, Type: "private"},
	})

	assert.True(t, handled)
	assert.Equal(t, assistantNotRegistered, resp.(tgbotapi.MessageConfig).Text)
}

func TestHandleAssistantCallback_AdminButtonsNeedAdmin(t *testing.T) {
	bot, mockStore := newAssistantTestBot()
	enableTestAssistants(bot, mockStore, 5)
	q := &tgbotapi.CallbackQuery{
		Data:    "aa:y:abc",
		From:    &tgbotapi.User{ID: 999},
		Message: &tgbotapi.Message{MessageID: 5, Chat: &tgbotapi.Chat{ID: 999}},
	}
	resp, err := bot.handleAssistantCallback(q)
	assert.NoError(t, err)
	assert.Nil(t, resp, "a participant must not trigger admin confirmations")
}

func TestAssistantExecutor_RunCommandUsesBotHandlers(t *testing.T) {
	bot, mockStore := newAssistantTestBot()
	mockStore.On("GetUserByTelegramID", mock.Anything, assistantTestAdmin).Return(&store.User{ID: 1, FirstName: "Admin"}, nil)
	mockStore.On("GetUserByName", mock.Anything, "Ivan").Return(&store.User{ID: 7, FirstName: "Ivan"}, nil)
	mockStore.On("SetSviniyaBalance", mock.Anything, int64(7), 3).Return(nil)

	res, err := assistantExecutor{b: bot}.RunCommand(context.Background(), 555, assistantTestAdmin, "/set_sviniya_balance Ivan 3")

	require.NoError(t, err)
	assert.Equal(t, "Set sviniya balance for Ivan to 3.", res.Text)
	mockStore.AssertExpectations(t)
}

func TestAssistantExecutor_RunCommandRejectsNonCommands(t *testing.T) {
	bot, _ := newAssistantTestBot()
	_, err := assistantExecutor{b: bot}.RunCommand(context.Background(), 1, assistantTestAdmin, "hello")
	assert.Error(t, err)
}

func TestAssistantExecutor_RunCallbackAllowList(t *testing.T) {
	bot, mockStore := newAssistantTestBot()
	mockStore.On("GetUserByTelegramID", mock.Anything, mock.Anything).Return(nil, nil)
	_, err := assistantExecutor{b: bot}.RunCallback(context.Background(), 1, assistantTestAdmin, "vacation:on")
	assert.ErrorContains(t, err, "not allowed")
}

func TestResultOf(t *testing.T) {
	msg := tgbotapi.NewMessage(1, "<b>hi</b>")
	msg.ParseMode = tgbotapi.ModeHTML
	assert.Equal(t, "<b>hi</b>", resultOf(msg).Text)
	assert.Equal(t, tgbotapi.ModeHTML, resultOf(msg).ParseMode)
	assert.Equal(t, "edited", resultOf(tgbotapi.NewEditMessageText(1, 2, "edited")).Text)
	assert.Empty(t, resultOf(nil).Text)
}

func TestHandleAssistantMessage_Routing(t *testing.T) {
	bot, _ := newAssistantTestBot()
	privateMsg := func(from int64, text string) *tgbotapi.Message {
		return &tgbotapi.Message{Text: text, From: &tgbotapi.User{ID: from}, Chat: &tgbotapi.Chat{ID: from, Type: "private"}}
	}

	// Non-admins and group chats keep the old behavior: ignored.
	_, handled := bot.handleAssistantMessage(privateMsg(999, "привет"))
	assert.False(t, handled)
	group := privateMsg(assistantTestAdmin, "привет")
	group.Chat.Type = "group"
	_, handled = bot.handleAssistantMessage(group)
	assert.False(t, handled)
	_, handled = bot.handleAssistantMessage(privateMsg(assistantTestAdmin, "   "))
	assert.False(t, handled)

	// Admin free text without the Anthropic provider gets a hint.
	resp, handled := bot.handleAssistantMessage(privateMsg(assistantTestAdmin, "назначь Ване посуду"))
	assert.True(t, handled)
	msg, ok := resp.(tgbotapi.MessageConfig)
	require.True(t, ok)
	assert.Equal(t, assistantUnavailableMessage, msg.Text)
}

func TestHandleMessage_SessionTakesPriorityOverAssistant(t *testing.T) {
	bot, _ := newAssistantTestBot()
	m := &tgbotapi.Message{Text: "5 4 3", From: &tgbotapi.User{ID: assistantTestAdmin}, Chat: &tgbotapi.Chat{ID: assistantTestAdmin, Type: "private"}}
	bot.handlers.SessionManager.StartSession(assistantTestAdmin, assistantTestAdmin, handlers.SessionType("unknown_type"))

	resp, err := bot.handleMessage(m)

	assert.NoError(t, err)
	assert.Nil(t, resp, "an active session must handle the text, not the assistant")
}

func TestHandleAssistantCallback_IgnoredWhenDisabledOrNotAdmin(t *testing.T) {
	bot, _ := newAssistantTestBot()
	q := &tgbotapi.CallbackQuery{
		Data:    "aa:y:abc",
		From:    &tgbotapi.User{ID: assistantTestAdmin},
		Message: &tgbotapi.Message{MessageID: 5, Chat: &tgbotapi.Chat{ID: assistantTestAdmin}},
	}
	resp, err := bot.handleAssistantCallback(q)
	assert.NoError(t, err)
	assert.Nil(t, resp)
}
