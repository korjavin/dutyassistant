package handlers_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/mocks"
	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/korjavin/dutyassistant/internal/telegram/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	assigneeTestAdminID = int64(123)
	assigneeTestChatID  = int64(789)
	assigneeTestGroupID = int64(-1001234567890)
)

type sentTelegramMessage struct {
	ChatID string
	Text   string
}

// assigneeTestUsers is the active roster used by the specific-assignee tests.
func assigneeTestUsers() []*store.User {
	return []*store.User{
		{ID: 10, TelegramUserID: 1010, FirstName: "Alice", IsActive: true},
		{ID: 11, TelegramUserID: 1111, FirstName: "Bob", IsActive: true},
		{ID: 12, TelegramUserID: 1212, FirstName: "Crusader", IsActive: true},
		{ID: 13, TelegramUserID: 1313, FirstName: "Anna Maria", IsActive: true},
		{ID: 14, TelegramUserID: 1414, FirstName: "Маша", IsActive: true},
	}
}

// setupAssigneeTest builds admin handlers with a mocked Telegram API that records sent messages.
func setupAssigneeTest(t *testing.T, users []*store.User) (*mocks.MockStore, *handlers.Handlers, *[]sentTelegramMessage) {
	t.Helper()

	mockStore := new(mocks.MockStore)
	mockStore.On("GetActiveChores", mock.Anything).Return([]*store.Chore{}, nil)
	mockStore.On("ListActiveUsers", mock.Anything).Return(users, nil)

	h := handlers.NewWithAdminID(mockStore, new(mocks.MockScheduler), assigneeTestGroupID, assigneeTestAdminID, nil)

	sent := &[]sentTelegramMessage{}
	client := NewTestClient(func(req *http.Request) *http.Response {
		if strings.Contains(req.URL.String(), "getMe") {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(`{"ok":true, "result": {"id": 123456, "is_bot": true, "first_name": "TestBot"}}`)),
				Header:     make(http.Header),
			}
		}
		if strings.Contains(req.URL.String(), "sendMessage") {
			bodyBytes, _ := io.ReadAll(req.Body)
			form, _ := url.ParseQuery(string(bodyBytes))
			*sent = append(*sent, sentTelegramMessage{ChatID: form.Get("chat_id"), Text: form.Get("text")})
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(`{"ok":true, "result": {"message_id": 1}}`)),
				Header:     make(http.Header),
			}
		}
		return &http.Response{StatusCode: 404, Body: io.NopCloser(bytes.NewBufferString(`{}`)), Header: make(http.Header)}
	})
	bot, err := tgbotapi.NewBotAPIWithClient("TOKEN", tgbotapi.APIEndpoint, client)
	require.NoError(t, err)
	h.SetBot(bot)

	return mockStore, h, sent
}

func adminChoreCommand(text string) *tgbotapi.Message {
	return &tgbotapi.Message{
		Chat:     &tgbotapi.Chat{ID: assigneeTestChatID},
		From:     &tgbotapi.User{ID: assigneeTestAdminID},
		Text:     text,
		Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: len("/chore")}},
	}
}

func choreForUser(userID int64, description string) interface{} {
	return mock.MatchedBy(func(c *store.Chore) bool {
		return c.UserID == userID && c.Description == description
	})
}

func messagesTo(sent []sentTelegramMessage, chatID string) []string {
	var texts []string
	for _, m := range sent {
		if m.ChatID == chatID {
			texts = append(texts, m.Text)
		}
	}
	return texts
}

func TestHandleChore_SpecificAssignee(t *testing.T) {
	tests := []struct {
		name       string
		command    string
		wantUser   *store.User
		wantDesc   string
		wantDMChat string
	}{
		{
			name:       "exact name",
			command:    "/chore Fix the door knob /Crusader",
			wantUser:   assigneeTestUsers()[2],
			wantDesc:   "Fix the door knob",
			wantDMChat: "1212",
		},
		{
			name:       "case-insensitive name",
			command:    "/chore Take out trash /bob",
			wantUser:   assigneeTestUsers()[1],
			wantDesc:   "Take out trash",
			wantDMChat: "1111",
		},
		{
			name:       "underscore matches space in first name",
			command:    "/chore Vacuum /anna_maria",
			wantUser:   assigneeTestUsers()[3],
			wantDesc:   "Vacuum",
			wantDMChat: "1313",
		},
		{
			name:       "cyrillic name",
			command:    "/chore Feed the cat /маша",
			wantUser:   assigneeTestUsers()[4],
			wantDesc:   "Feed the cat",
			wantDMChat: "1414",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore, h, sent := setupAssigneeTest(t, assigneeTestUsers())
			mockStore.On("CreateChore", mock.Anything, choreForUser(tt.wantUser.ID, tt.wantDesc)).Return(nil).Once()

			msg, err := h.HandleChore(adminChoreCommand(tt.command))
			require.NoError(t, err)

			text := msg.(tgbotapi.MessageConfig).Text
			assert.Contains(t, text, "Assigned chore to <b>"+tt.wantUser.FirstName+"</b>")
			assert.Contains(t, text, "📨 DM sent to user")

			// Chore persisted for the named user with the suffix stripped from the description.
			mockStore.AssertExpectations(t)
			// Explicit assignment bypasses the off-duty filter entirely.
			mockStore.AssertNotCalled(t, "GetOffDutyUsers", mock.Anything, mock.Anything)

			// Group announcement names the assignee and the clean description.
			group := messagesTo(*sent, "-1001234567890")
			require.Len(t, group, 1)
			assert.Equal(t, "🎯 <b>"+tt.wantUser.FirstName+"</b>: <i>"+tt.wantDesc+"</i>", group[0])

			// DM goes to the named user's Telegram account.
			dms := messagesTo(*sent, tt.wantDMChat)
			require.Len(t, dms, 1)
			assert.Contains(t, dms[0], tt.wantDesc)
			assert.NotContains(t, dms[0], "/"+tt.wantUser.FirstName)
		})
	}
}

func TestHandleChore_SpecificAssignee_IsNotRandom(t *testing.T) {
	mockStore, h, _ := setupAssigneeTest(t, assigneeTestUsers())
	mockStore.On("CreateChore", mock.Anything, choreForUser(11, "Clean kitchen")).Return(nil)

	for i := 0; i < 20; i++ {
		msg, err := h.HandleChore(adminChoreCommand("/chore Clean kitchen /Bob"))
		require.NoError(t, err)
		assert.Contains(t, msg.(tgbotapi.MessageConfig).Text, "Assigned chore to <b>Bob</b>")
	}
	mockStore.AssertNumberOfCalls(t, "CreateChore", 20)
}

func TestHandleChore_SpecificAssignee_OffDutyUserStillAssigned(t *testing.T) {
	mockStore, h, _ := setupAssigneeTest(t, assigneeTestUsers())
	// Even if Alice is off duty, an explicit admin choice wins.
	mockStore.On("GetOffDutyUsers", mock.Anything, mock.Anything).Return([]*store.User{assigneeTestUsers()[0]}, nil).Maybe()
	mockStore.On("CreateChore", mock.Anything, choreForUser(10, "Water plants")).Return(nil).Once()

	msg, err := h.HandleChore(adminChoreCommand("/chore Water plants /Alice"))
	require.NoError(t, err)
	assert.Contains(t, msg.(tgbotapi.MessageConfig).Text, "Assigned chore to <b>Alice</b>")
	mockStore.AssertExpectations(t)
}

func TestHandleChore_SpecificAssignee_UnknownUser(t *testing.T) {
	mockStore, h, sent := setupAssigneeTest(t, assigneeTestUsers())

	msg, err := h.HandleChore(adminChoreCommand("/chore Fix the door /Zorro"))
	require.NoError(t, err)

	msgConfig := msg.(tgbotapi.MessageConfig)
	assert.Equal(t, tgbotapi.ModeHTML, msgConfig.ParseMode)
	assert.Contains(t, msgConfig.Text, "No active user named <b>Zorro</b>")
	assert.Contains(t, msgConfig.Text, "Alice, Bob, Crusader, Anna Maria, Маша")

	mockStore.AssertNotCalled(t, "CreateChore", mock.Anything, mock.Anything)
	assert.Empty(t, *sent, "nothing must be announced or DMed for an unknown assignee")
}

func TestHandleChore_SpecificAssignee_UnknownUserIsEscaped(t *testing.T) {
	mockStore, h, _ := setupAssigneeTest(t, assigneeTestUsers())

	msg, err := h.HandleChore(adminChoreCommand("/chore Fix /<script>"))
	require.NoError(t, err)

	text := msg.(tgbotapi.MessageConfig).Text
	assert.Contains(t, text, "No active user named <b>&lt;script&gt;</b>")
	assert.NotContains(t, text, "<script>")
	mockStore.AssertNotCalled(t, "CreateChore", mock.Anything, mock.Anything)
}

func TestHandleChore_SpecificAssignee_AmbiguousName(t *testing.T) {
	users := []*store.User{
		{ID: 10, TelegramUserID: 1010, FirstName: "Alex", IsActive: true},
		{ID: 11, TelegramUserID: 1111, FirstName: "alex", IsActive: true},
	}
	mockStore, h, sent := setupAssigneeTest(t, users)

	msg, err := h.HandleChore(adminChoreCommand("/chore Clean /Alex"))
	require.NoError(t, err)

	assert.Contains(t, msg.(tgbotapi.MessageConfig).Text, "More than one active user is named <b>Alex</b>")
	mockStore.AssertNotCalled(t, "CreateChore", mock.Anything, mock.Anything)
	assert.Empty(t, *sent)
}

func TestHandleChore_SpecificAssignee_InactiveUserNotFound(t *testing.T) {
	// ListActiveUsers only returns active users, so an inactive user can't be targeted.
	mockStore, h, _ := setupAssigneeTest(t, []*store.User{
		{ID: 10, TelegramUserID: 1010, FirstName: "Alice", IsActive: true},
	})

	msg, err := h.HandleChore(adminChoreCommand("/chore Clean /Bob"))
	require.NoError(t, err)

	assert.Contains(t, msg.(tgbotapi.MessageConfig).Text, "No active user named <b>Bob</b>")
	mockStore.AssertNotCalled(t, "CreateChore", mock.Anything, mock.Anything)
}

func TestHandleChore_SpecificAssignee_ListUsersError(t *testing.T) {
	mockStore := new(mocks.MockStore)
	mockStore.On("GetActiveChores", mock.Anything).Return([]*store.Chore{}, nil)
	mockStore.On("ListActiveUsers", mock.Anything).Return(nil, assert.AnError)
	h := handlers.NewWithAdminID(mockStore, new(mocks.MockScheduler), 0, assigneeTestAdminID, nil)

	msg, err := h.HandleChore(adminChoreCommand("/chore Clean /Bob"))
	require.NoError(t, err)

	assert.Equal(t, "Failed to retrieve user list.", msg.(tgbotapi.MessageConfig).Text)
	mockStore.AssertNotCalled(t, "CreateChore", mock.Anything, mock.Anything)
}

func TestHandleChore_SpecificAssignee_EmptyDescription(t *testing.T) {
	mockStore, h, sent := setupAssigneeTest(t, assigneeTestUsers())

	msg, err := h.HandleChore(adminChoreCommand("/chore /Bob"))
	require.NoError(t, err)

	assert.Equal(t, "❌ Chore description cannot be empty.", msg.(tgbotapi.MessageConfig).Text)
	mockStore.AssertNotCalled(t, "CreateChore", mock.Anything, mock.Anything)
	assert.Empty(t, *sent)
}

// fixChoreClock pins handlers.TimeNow to the given Berlin hour on a fixed day.
func fixChoreClock(t *testing.T, hour int) {
	t.Helper()
	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)
	handlers.TimeNow = func() time.Time { return time.Date(2025, time.October, 15, hour, 0, 0, 0, berlin) }
	t.Cleanup(func() { handlers.TimeNow = time.Now })
}

func recurringChoreFor(assigneeID int64, description string, interval int) interface{} {
	return mock.MatchedBy(func(c *store.RecurringChore) bool {
		return c.AssigneeUserID == assigneeID && c.Description == description && c.Interval == interval
	})
}

func TestHandleChore_RecurringWithAssignee_DuringHours(t *testing.T) {
	for _, command := range []string{
		"/chore Water plants /Alice /3d",
		"/chore Water plants /3d /alice",
	} {
		t.Run(command, func(t *testing.T) {
			fixChoreClock(t, 14)
			mockStore, h, sent := setupAssigneeTest(t, assigneeTestUsers())
			mockStore.On("CreateRecurringChore", mock.Anything, recurringChoreFor(10, "Water plants", 3)).
				Return(nil).Run(func(args mock.Arguments) { args.Get(1).(*store.RecurringChore).ID = 7 }).Once()
			mockStore.On("CreateChore", mock.Anything, choreForUser(10, "Water plants")).Return(nil).Once()

			msg, err := h.HandleChore(adminChoreCommand(command))
			require.NoError(t, err)

			text := msg.(tgbotapi.MessageConfig).Text
			assert.Contains(t, text, "Assigned chore to <b>Alice</b>")
			assert.Contains(t, text, "Recurring chore scheduled!")
			assert.Contains(t, text, "<b>Assignee:</b> <b>Alice</b>")
			assert.Contains(t, text, "every 3 days")
			mockStore.AssertExpectations(t)
			mockStore.AssertNotCalled(t, "GetOffDutyUsers", mock.Anything, mock.Anything)
			require.Len(t, messagesTo(*sent, "1010"), 1)
			assert.Equal(t, []string{"🎯 <b>Alice</b>: <i>Water plants</i>"}, messagesTo(*sent, "-1001234567890"))
		})
	}
}

func TestHandleChore_RecurringWithAssignee_OutsideHours(t *testing.T) {
	fixChoreClock(t, 20)
	mockStore, h, sent := setupAssigneeTest(t, assigneeTestUsers())
	mockStore.On("CreateRecurringChore", mock.Anything, recurringChoreFor(11, "Water plants", 2)).Return(nil).Once()

	msg, err := h.HandleChore(adminChoreCommand("/chore Water plants /Bob /2d"))
	require.NoError(t, err)

	text := msg.(tgbotapi.MessageConfig).Text
	assert.Contains(t, text, "Recurring chore scheduled!")
	assert.Contains(t, text, "<b>Assignee:</b> <b>Bob</b>")
	mockStore.AssertExpectations(t)
	mockStore.AssertNotCalled(t, "CreateChore", mock.Anything, mock.Anything)
	assert.Empty(t, *sent)
}

func TestHandleChore_RecurringWithUnknownAssignee(t *testing.T) {
	fixChoreClock(t, 14)
	mockStore, h, sent := setupAssigneeTest(t, assigneeTestUsers())

	msg, err := h.HandleChore(adminChoreCommand("/chore Water plants /Zorro /3d"))
	require.NoError(t, err)

	assert.Contains(t, msg.(tgbotapi.MessageConfig).Text, "No active user named <b>Zorro</b>")
	mockStore.AssertNotCalled(t, "CreateRecurringChore", mock.Anything, mock.Anything)
	mockStore.AssertNotCalled(t, "CreateChore", mock.Anything, mock.Anything)
	assert.Empty(t, *sent)
}

func TestHandleChore_RecurringWithoutAssigneeStillWorks(t *testing.T) {
	fixChoreClock(t, 14)
	mockStore, h, _ := setupAssigneeTest(t, assigneeTestUsers())
	mockStore.On("CreateRecurringChore", mock.Anything, mock.MatchedBy(func(c *store.RecurringChore) bool {
		return c.Description == "Water plants" && c.Interval == 3
	})).Return(nil).Once()
	mockStore.On("GetOffDutyUsers", mock.Anything, mock.Anything).Return([]*store.User{}, nil).Maybe()
	mockStore.On("CreateChore", mock.Anything, mock.Anything).Return(nil).Maybe()

	msg, err := h.HandleChore(adminChoreCommand("/chore Water plants /3d"))
	require.NoError(t, err)

	assert.Contains(t, msg.(tgbotapi.MessageConfig).Text, "Recurring chore scheduled!")
	mockStore.AssertExpectations(t)
}

func TestHandleChoreInteractive_SpecificAssignee(t *testing.T) {
	mockStore, h, sent := setupAssigneeTest(t, assigneeTestUsers())
	mockStore.On("CreateChore", mock.Anything, choreForUser(12, "Clean the coffee machine")).Return(nil).Once()

	h.SessionManager.StartSession(assigneeTestChatID, assigneeTestAdminID, handlers.SessionTypeChoreCreation)

	msg, err := h.HandleChoreInteractive(&tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: assigneeTestChatID},
		From: &tgbotapi.User{ID: assigneeTestAdminID},
		Text: "Clean the coffee machine /crusader",
	})
	require.NoError(t, err)

	assert.Contains(t, msg.(tgbotapi.MessageConfig).Text, "Assigned chore to <b>Crusader</b>")
	mockStore.AssertExpectations(t)
	mockStore.AssertNotCalled(t, "GetOffDutyUsers", mock.Anything, mock.Anything)
	require.Len(t, messagesTo(*sent, "1212"), 1)

	_, exists := h.SessionManager.GetSession(assigneeTestChatID)
	assert.False(t, exists)
}

func TestHandleChoreInteractive_SpecificAssignee_UnknownUser(t *testing.T) {
	mockStore, h, sent := setupAssigneeTest(t, assigneeTestUsers())
	h.SessionManager.StartSession(assigneeTestChatID, assigneeTestAdminID, handlers.SessionTypeChoreCreation)

	msg, err := h.HandleChoreInteractive(&tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: assigneeTestChatID},
		From: &tgbotapi.User{ID: assigneeTestAdminID},
		Text: "Clean the coffee machine /Zorro",
	})
	require.NoError(t, err)

	assert.Contains(t, msg.(tgbotapi.MessageConfig).Text, "No active user named <b>Zorro</b>")
	mockStore.AssertNotCalled(t, "CreateChore", mock.Anything, mock.Anything)
	assert.Empty(t, *sent)
}

func TestHandleChoreInteractive_Recurring(t *testing.T) {
	tests := []struct {
		name         string
		text         string
		wantAssignee int64
		wantText     string
	}{
		{name: "random", text: "Water plants /3d", wantAssignee: 0, wantText: "<b>Assignee:</b> random user"},
		{name: "specific", text: "Water plants /bob /3d", wantAssignee: 11, wantText: "<b>Assignee:</b> <b>Bob</b>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixChoreClock(t, 20)
			mockStore, h, sent := setupAssigneeTest(t, assigneeTestUsers())
			mockStore.On("CreateRecurringChore", mock.Anything, recurringChoreFor(tt.wantAssignee, "Water plants", 3)).Return(nil).Once()
			h.SessionManager.StartSession(assigneeTestChatID, assigneeTestAdminID, handlers.SessionTypeChoreCreation)

			msg, err := h.HandleChoreInteractive(&tgbotapi.Message{
				Chat: &tgbotapi.Chat{ID: assigneeTestChatID},
				From: &tgbotapi.User{ID: assigneeTestAdminID},
				Text: tt.text,
			})
			require.NoError(t, err)

			text := msg.(tgbotapi.MessageConfig).Text
			assert.Contains(t, text, "Recurring chore scheduled!")
			assert.Contains(t, text, tt.wantText)
			mockStore.AssertNumberOfCalls(t, "CreateRecurringChore", 1)
			mockStore.AssertNotCalled(t, "CreateChore", mock.Anything, mock.Anything)
			assert.Empty(t, *sent)

			_, exists := h.SessionManager.GetSession(assigneeTestChatID)
			assert.False(t, exists)
		})
	}
}

func TestProcessRecurringChores_FixedAssignee(t *testing.T) {
	mockStore, h, sent := setupAssigneeTest(t, assigneeTestUsers())
	mockStore.On("GetDueRecurringChores", mock.Anything, mock.Anything).Return([]*store.RecurringChore{
		{ID: 7, Description: "Water plants", Interval: 3, NextRunAt: time.Now().Add(-time.Hour), IsActive: true, AssigneeUserID: 12},
	}, nil)
	mockStore.On("CreateChore", mock.Anything, choreForUser(12, "Water plants")).Return(nil).Once()
	mockStore.On("UpdateRecurringChoreNextRun", mock.Anything, int64(7), mock.Anything).Return(nil).Once()

	require.NoError(t, h.ProcessRecurringChores(context.Background()))

	mockStore.AssertExpectations(t)
	mockStore.AssertNotCalled(t, "GetOffDutyUsers", mock.Anything, mock.Anything)
	assert.Equal(t, []string{"🎯 <b>Crusader</b>: <i>Water plants</i>"}, messagesTo(*sent, "-1001234567890"))
	require.Len(t, messagesTo(*sent, "1212"), 1)
}

func TestProcessRecurringChores_InactiveAssigneeFallsBackToRandom(t *testing.T) {
	mockStore, h, _ := setupAssigneeTest(t, []*store.User{
		{ID: 10, TelegramUserID: 1010, FirstName: "Alice", IsActive: true},
	})
	mockStore.On("GetDueRecurringChores", mock.Anything, mock.Anything).Return([]*store.RecurringChore{
		// User 99 is no longer in the active roster.
		{ID: 7, Description: "Water plants", Interval: 3, NextRunAt: time.Now().Add(-time.Hour), IsActive: true, AssigneeUserID: 99},
	}, nil)
	mockStore.On("GetOffDutyUsers", mock.Anything, mock.Anything).Return([]*store.User{}, nil).Once()
	mockStore.On("CreateChore", mock.Anything, choreForUser(10, "Water plants")).Return(nil).Once()
	mockStore.On("UpdateRecurringChoreNextRun", mock.Anything, int64(7), mock.Anything).Return(nil).Once()

	require.NoError(t, h.ProcessRecurringChores(context.Background()))

	mockStore.AssertExpectations(t)
}

func TestHandleChore_NonAdminCannotAssignToSpecificUser(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	nonAdmin := &store.User{ID: 2, TelegramUserID: 456, IsAdmin: false}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(456)).Return(nonAdmin, nil)
	mockStore.On("GetActiveChoresByUserID", mock.Anything, int64(2)).Return([]*store.Chore{}, nil)

	msg, err := h.HandleChore(&tgbotapi.Message{
		Chat:     &tgbotapi.Chat{ID: assigneeTestChatID},
		From:     &tgbotapi.User{ID: 456},
		Text:     "/chore Clean kitchen /Bob",
		Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: len("/chore")}},
	})
	require.NoError(t, err)

	// Non-admins only ever see their own chores list.
	assert.Contains(t, msg.(tgbotapi.MessageConfig).Text, "You have no active chores right now!")
	mockStore.AssertNotCalled(t, "ListActiveUsers", mock.Anything)
	mockStore.AssertNotCalled(t, "CreateChore", mock.Anything, mock.Anything)
}
