package telegram

import (
	"context"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

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

const (
	testGroup   = int64(-100500)
	testBotID   = int64(4242)
	testBotName = "DutyBot"
)

func newGroupAssistantTestBot() (*Bot, *mocks.MockStore) {
	bot, mockStore := newAssistantTestBot()
	bot.groupID = testGroup
	bot.self = tgbotapi.User{ID: testBotID, UserName: testBotName, IsBot: true}
	return bot, mockStore
}

// groupMsg builds a group message mentioning @mention at the UTF-16 offset of
// its first occurrence (Telegram entity offsets are in UTF-16 code units).
func groupMsg(chatID, from int64, text, mention string) *tgbotapi.Message {
	m := &tgbotapi.Message{MessageID: 77, Text: text, From: &tgbotapi.User{ID: from, FirstName: "Tg"}, Chat: &tgbotapi.Chat{ID: chatID, Type: "supergroup"}}
	if i := strings.Index(text, mention); mention != "" && i >= 0 {
		m.Entities = []tgbotapi.MessageEntity{{
			Type:   "mention",
			Offset: len(utf16.Encode([]rune(text[:i]))),
			Length: len(utf16.Encode([]rune(mention))),
		}}
	}
	return m
}

func TestGroupMention_Detection(t *testing.T) {
	bot, _ := newGroupAssistantTestBot()

	m := groupMsg(testGroup, 1, "🙂 Привет @dutybot что у меня сегодня?", "@dutybot")
	assert.True(t, bot.isGroupMention(m), "username match is case-insensitive")
	assert.Equal(t, "🙂 Привет что у меня сегодня?", bot.stripMentions(m))

	assert.False(t, bot.isGroupMention(groupMsg(testGroup, 1, "привет @OtherBot", "@OtherBot")))
	assert.False(t, bot.isGroupMention(groupMsg(testGroup, 1, "привет всем", "")))
	assert.False(t, bot.isGroupMention(groupMsg(-1, 1, "@DutyBot hi", "@DutyBot")), "only the main group")

	fromBot := groupMsg(testGroup, 1, "@DutyBot hi", "@DutyBot")
	fromBot.From.IsBot = true
	assert.False(t, bot.isGroupMention(fromBot))

	textMention := groupMsg(testGroup, 1, "Дежурный бот, кто сегодня?", "")
	textMention.Entities = []tgbotapi.MessageEntity{{Type: "text_mention", Offset: 0, Length: 12, User: &tgbotapi.User{ID: testBotID}}}
	assert.True(t, bot.isGroupMention(textMention))
	assert.Equal(t, "Дежурный бот, кто сегодня?", bot.stripMentions(textMention))

	bot.groupID = 0
	assert.False(t, bot.isGroupMention(m), "no group configured")
}

func TestHandleAssistantMessage_GroupMention(t *testing.T) {
	bot, mockStore := newGroupAssistantTestBot()

	// Admin without the Anthropic provider gets the hint as a reply.
	resp, handled := bot.handleAssistantMessage(groupMsg(testGroup, assistantTestAdmin, "@DutyBot назначь Ване посуду", "@DutyBot"))
	require.True(t, handled)
	msg := resp.(tgbotapi.MessageConfig)
	assert.Equal(t, assistantUnavailableMessage, msg.Text)
	assert.Equal(t, testGroup, msg.ChatID)
	assert.Equal(t, 77, msg.ReplyToMessageID)

	// A bare mention or a message without one is not for the assistant.
	_, handled = bot.handleAssistantMessage(groupMsg(testGroup, assistantTestAdmin, "@DutyBot", "@DutyBot"))
	assert.False(t, handled)
	_, handled = bot.handleAssistantMessage(groupMsg(testGroup, assistantTestAdmin, "назначь Ване посуду", ""))
	assert.False(t, handled)

	// Unregistered participants are asked to /start.
	enableTestAssistants(bot, mockStore, 5)
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(999)).Return(nil, nil)
	resp, handled = bot.handleAssistantMessage(groupMsg(testGroup, 999, "@DutyBot что у меня?", "@DutyBot"))
	require.True(t, handled)
	assert.Equal(t, assistantNotRegistered, resp.(tgbotapi.MessageConfig).Text)
}

func TestHandleMessage_GroupMentionBypassesSomeoneElsesSession(t *testing.T) {
	bot, _ := newGroupAssistantTestBot()
	// Another user's session in the group must not swallow the admin's mention.
	bot.handlers.SessionManager.StartSession(testGroup, 999, handlers.SessionType("unknown_type"))

	resp, err := bot.handleMessage(groupMsg(testGroup, assistantTestAdmin, "@DutyBot привет", "@DutyBot"))

	assert.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, assistantUnavailableMessage, resp.(tgbotapi.MessageConfig).Text)
}
