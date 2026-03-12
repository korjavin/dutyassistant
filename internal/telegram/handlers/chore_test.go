package handlers_test

import (
	"bytes"
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
)

func TestHandleChore_NonAdmin_NoChores(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0)

	// User is not admin
	nonAdminUser := &store.User{ID: 2, TelegramUserID: 456, IsAdmin: false}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(456)).Return(nonAdminUser, nil)
	mockStore.On("GetActiveChoresByUserID", mock.Anything, int64(2)).Return([]*store.Chore{}, nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 789},
		From: &tgbotapi.User{ID: 456},
		Text: "/chore",
	}

	msg, err := h.HandleChore(message)
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "You have no active chores right now!")
}

func TestHandleChore_NonAdmin_WithChores(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0)

	// User is not admin
	nonAdminUser := &store.User{ID: 2, TelegramUserID: 456, IsAdmin: false}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(456)).Return(nonAdminUser, nil)

	// Has one chore
	chores := []*store.Chore{
		{
			ID:          1,
			UserID:      2,
			Description: "Take out the trash <script>",
			AssignedAt:  time.Now(),
		},
	}
	mockStore.On("GetActiveChoresByUserID", mock.Anything, int64(2)).Return(chores, nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 789},
		From: &tgbotapi.User{ID: 456},
		Text: "/chore",
	}

	msg, err := h.HandleChore(message)
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "Your Active Chores")
	assert.Contains(t, msg.Text, "Take out the trash &lt;script&gt;")
}

func TestHandleChore_NonAdmin_UserNotRegistered(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0)

	// User is not admin and not in DB
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(456)).Return((*store.User)(nil), nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 789},
		From: &tgbotapi.User{ID: 456},
		Text: "/chore",
	}

	msg, err := h.HandleChore(message)
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "Could not find your user profile")
}

func TestHandleChore_NoArgs_EntersInteractiveMode(t *testing.T) {
	_, _, h := setupAdminTest(t)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 789},
		From: &tgbotapi.User{ID: 123},
		Text: "/chore", // No description
	}

	msg, err := h.HandleChore(message)
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "Interactive Chore Mode")
	assert.Contains(t, msg.Text, "What chore do you want to create?")

	// Verify session was created
	session, exists := h.SessionManager.GetSession(789)
	assert.True(t, exists)
	assert.Equal(t, handlers.SessionTypeChoreCreation, session.Type)
	assert.Equal(t, int64(123), session.UserID)
}

func TestHandleChore_Success(t *testing.T) {
	mockStore := new(mocks.MockStore)
	mockStore.On("GetActiveChores", mock.Anything).Return([]*store.Chore{}, nil)
	mockScheduler := new(mocks.MockScheduler)
	// Create with admin ID 123 and group ID 0
	h := handlers.NewWithAdminID(mockStore, mockScheduler, 0, 123)

	// Admin user setup not needed if we use NewWithAdminID and match ID

	// Active users for selection
	activeUsers := []*store.User{
		{ID: 10, FirstName: "Alice", IsActive: true},
		{ID: 11, FirstName: "Bob", IsActive: true},
	}
	mockStore.On("ListActiveUsers", mock.Anything).Return(activeUsers, nil)

	// Mock off-duty check - everyone on duty
	mockStore.On("GetOffDutyUsers", mock.Anything, mock.Anything).Return([]*store.User{}, nil)
	mockStore.On("CreateChore", mock.Anything, mock.Anything).Return(nil)

	message := &tgbotapi.Message{
		Chat:     &tgbotapi.Chat{ID: 789},
		From:     &tgbotapi.User{ID: 123},
		Text:     "/chore Clean kitchen",
		Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 6}},
	}

	msg, err := h.HandleChore(message)
	assert.NoError(t, err)

	// Message should state success and mention one of the users
	assert.Contains(t, msg.Text, "Assigned chore to")
	// Should mention Alice or Bob
	// isAlice := assert.Contains(t, msg.Text, "Alice")
	// isBob := assert.Contains(t, msg.Text, "Bob")
	// We check if either is present
	assert.True(t, strings.Contains(msg.Text, "Alice") || strings.Contains(msg.Text, "Bob"))
}

func TestHandleChore_NoActiveUsers(t *testing.T) {
	mockStore, _, h := setupAdminTest(t)

	mockStore.On("ListActiveUsers", mock.Anything).Return([]*store.User{}, nil)

	message := &tgbotapi.Message{
		Chat:     &tgbotapi.Chat{ID: 789},
		From:     &tgbotapi.User{ID: 123},
		Text:     "/chore Clean kitchen",
		Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 6}},
	}

	msg, err := h.HandleChore(message)
	assert.NoError(t, err)
	assert.Equal(t, "No active users found to assign the chore to.", msg.Text)
}

func TestHandleChore_HTMLInjection(t *testing.T) {
	mockStore := new(mocks.MockStore)
	mockStore.On("GetActiveChores", mock.Anything).Return([]*store.Chore{}, nil)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.NewWithAdminID(mockStore, mockScheduler, 0, 123)

	activeUsers := []*store.User{
		{ID: 10, FirstName: "<b>EvilUser</b>", IsActive: true},
	}
	mockStore.On("ListActiveUsers", mock.Anything).Return(activeUsers, nil)
	mockStore.On("GetOffDutyUsers", mock.Anything, mock.Anything).Return([]*store.User{}, nil)
	mockStore.On("CreateChore", mock.Anything, mock.Anything).Return(nil)

	message := &tgbotapi.Message{
		Chat:     &tgbotapi.Chat{ID: 789},
		From:     &tgbotapi.User{ID: 123},
		Text:     "/chore <i>malicious description</i>",
		Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 6}},
	}

	msg, err := h.HandleChore(message)
	assert.NoError(t, err)

	// Check that HTML tags are escaped in the output
	assert.Contains(t, msg.Text, "&lt;b&gt;EvilUser&lt;/b&gt;")
	assert.Contains(t, msg.Text, "&lt;i&gt;malicious description&lt;/i&gt;")
}

type RoundTripFunc func(req *http.Request) *http.Response

func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

func NewTestClient(fn RoundTripFunc) *http.Client {
	return &http.Client{
		Transport: fn,
	}
}

func TestHandleChore_GroupAnnouncement(t *testing.T) {
	mockStore := new(mocks.MockStore)
	mockStore.On("GetActiveChores", mock.Anything).Return([]*store.Chore{}, nil)
	mockScheduler := new(mocks.MockScheduler)
	groupID := int64(-1001234567890)
	h := handlers.NewWithAdminID(mockStore, mockScheduler, groupID, 123)

	activeUsers := []*store.User{
		{ID: 10, TelegramUserID: 111, FirstName: "Alice", IsActive: true},
	}
	mockStore.On("ListActiveUsers", mock.Anything).Return(activeUsers, nil)
	mockStore.On("GetOffDutyUsers", mock.Anything, mock.Anything).Return([]*store.User{}, nil)
	mockStore.On("CreateChore", mock.Anything, mock.Anything).Return(nil)

	// Mock Bot API client
	client := NewTestClient(func(req *http.Request) *http.Response {
		// Verify expected URL and payload if needed
		// For now simple check that it returns success for sendMessage
		if req.URL.String() == "https://api.telegram.org/botTOKEN/sendMessage" {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(`{"ok":true, "result": {"message_id": 1, "chat": {"id": -1001234567890, "type": "supergroup"}}}`)),
				Header:     make(http.Header),
			}
		}
		if req.URL.String() == "https://api.telegram.org/botTOKEN/getMe" {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(`{"ok":true, "result": {"id": 123456, "is_bot": true, "first_name": "TestBot", "username": "test_bot"}}`)),
				Header:     make(http.Header),
			}
		}
		return &http.Response{
			StatusCode: 404,
			Body:       io.NopCloser(bytes.NewBufferString(`{}`)),
			Header:     make(http.Header),
		}
	})

	bot, err := tgbotapi.NewBotAPIWithClient("TOKEN", tgbotapi.APIEndpoint, client)
	assert.NoError(t, err)
	bot.Debug = true
	h.SetBot(bot)

	message := &tgbotapi.Message{
		Chat:     &tgbotapi.Chat{ID: 789}, // DM
		From:     &tgbotapi.User{ID: 123},
		Text:     "/chore Clean kitchen",
		Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 6}},
	}

	msg, err := h.HandleChore(message)
	assert.NoError(t, err)

	// Check response to user
	assert.Contains(t, msg.Text, "Assigned chore to")
	assert.Contains(t, msg.Text, "Announced in group")
	assert.Contains(t, msg.Text, "DM sent to user")
}

func TestHandleChore_MissingTelegramID_ShowsRegistrationHint(t *testing.T) {
	mockStore := new(mocks.MockStore)
	mockStore.On("GetActiveChores", mock.Anything).Return([]*store.Chore{}, nil)
	mockScheduler := new(mocks.MockScheduler)
	groupID := int64(-1001234567890)
	h := handlers.NewWithAdminID(mockStore, mockScheduler, groupID, 123)

	activeUsers := []*store.User{
		{ID: 10, TelegramUserID: 0, FirstName: "Alice", IsActive: true},
	}
	mockStore.On("ListActiveUsers", mock.Anything).Return(activeUsers, nil)
	mockStore.On("GetOffDutyUsers", mock.Anything, mock.Anything).Return([]*store.User{}, nil)
	mockStore.On("CreateChore", mock.Anything, mock.Anything).Return(nil)

	client := NewTestClient(func(req *http.Request) *http.Response {
		if req.URL.String() == "https://api.telegram.org/botTOKEN/getMe" {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(`{"ok":true, "result": {"id": 123456, "is_bot": true, "first_name": "TestBot", "username": "test_bot"}}`)),
				Header:     make(http.Header),
			}
		}
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewBufferString(`{"ok":true, "result": {"message_id": 1}}`)),
			Header:     make(http.Header),
		}
	})
	bot, err := tgbotapi.NewBotAPIWithClient("TOKEN", tgbotapi.APIEndpoint, client)
	assert.NoError(t, err)
	h.SetBot(bot)

	message := &tgbotapi.Message{
		Chat:     &tgbotapi.Chat{ID: groupID},
		From:     &tgbotapi.User{ID: 123},
		Text:     "/chore Clean kitchen",
		Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 6}},
	}

	msg, err := h.HandleChore(message)
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "Couldn't send DM")
	assert.Contains(t, msg.Text, "/start")
}

func TestHandleChoreInteractive_Success(t *testing.T) {
	mockStore := new(mocks.MockStore)
	mockStore.On("GetActiveChores", mock.Anything).Return([]*store.Chore{}, nil)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.NewWithAdminID(mockStore, mockScheduler, 0, 123)

	// First, enter interactive mode
	h.SessionManager.StartSession(789, 123, handlers.SessionTypeChoreCreation)

	// Setup mock data for chore assignment
	activeUsers := []*store.User{
		{ID: 10, TelegramUserID: 111, FirstName: "Alice", IsActive: true},
		{ID: 11, TelegramUserID: 222, FirstName: "Bob", IsActive: true},
	}
	mockStore.On("ListActiveUsers", mock.Anything).Return(activeUsers, nil)
	mockStore.On("GetOffDutyUsers", mock.Anything, mock.Anything).Return([]*store.User{}, nil)
	mockStore.On("CreateChore", mock.Anything, mock.Anything).Return(nil)

	// Send a message with chore description
	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 789},
		From: &tgbotapi.User{ID: 123},
		Text: "Clean the coffee machine",
	}

	msg, err := h.HandleChoreInteractive(message)
	assert.NoError(t, err)
	assert.NotNil(t, msg)

	// Type assert to MessageConfig to access Text field
	msgConfig, ok := msg.(tgbotapi.MessageConfig)
	assert.True(t, ok)
	assert.Contains(t, msgConfig.Text, "Assigned chore to")

	// Verify session was ended
	_, exists := h.SessionManager.GetSession(789)
	assert.False(t, exists)
}

func TestHandleChoreInteractive_EmptyDescription(t *testing.T) {
	mockStore := new(mocks.MockStore)
	mockStore.On("GetActiveChores", mock.Anything).Return([]*store.Chore{}, nil)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.NewWithAdminID(mockStore, mockScheduler, 0, 123)

	// Enter interactive mode
	h.SessionManager.StartSession(789, 123, handlers.SessionTypeChoreCreation)

	// Send empty message
	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 789},
		From: &tgbotapi.User{ID: 123},
		Text: "   ",
	}

	msg, err := h.HandleChoreInteractive(message)
	assert.NoError(t, err)
	assert.NotNil(t, msg)

	// Type assert to MessageConfig to access Text field
	msgConfig, ok := msg.(tgbotapi.MessageConfig)
	assert.True(t, ok)
	assert.Contains(t, msgConfig.Text, "cannot be empty")

	// Session should be ended even on error
	_, exists := h.SessionManager.GetSession(789)
	assert.False(t, exists)
}

func TestHandleChoreInteractive_NoSession(t *testing.T) {
	mockStore := new(mocks.MockStore)
	mockStore.On("GetActiveChores", mock.Anything).Return([]*store.Chore{}, nil)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.NewWithAdminID(mockStore, mockScheduler, 0, 123)

	// No session started
	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 789},
		From: &tgbotapi.User{ID: 123},
		Text: "Some random text",
	}

	msg, err := h.HandleChoreInteractive(message)
	assert.NoError(t, err)
	// Should return nil when no session (completely ignored)
	assert.Nil(t, msg)
}

func TestHandleChoreInteractive_WrongUser(t *testing.T) {
	mockStore := new(mocks.MockStore)
	mockStore.On("GetActiveChores", mock.Anything).Return([]*store.Chore{}, nil)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.NewWithAdminID(mockStore, mockScheduler, 0, 123)

	// Start session for user 123
	h.SessionManager.StartSession(789, 123, handlers.SessionTypeChoreCreation)

	// Different user sends message
	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 789},
		From: &tgbotapi.User{ID: 456}, // Different user
		Text: "Clean kitchen",
	}

	msg, err := h.HandleChoreInteractive(message)
	assert.NoError(t, err)
	// Should return nil when wrong user (completely ignored)
	assert.Nil(t, msg)

	// Session should still exist (only the original user can interact)
	_, exists := h.SessionManager.GetSession(789)
	assert.True(t, exists)
}

func TestSessionManager_SessionLifecycle(t *testing.T) {
	sm := handlers.NewSessionManager()

	// Start a session
	sm.StartSession(123, 456, handlers.SessionTypeChoreCreation)

	// Get session
	session, exists := sm.GetSession(123)
	assert.True(t, exists)
	assert.Equal(t, int64(123), session.ChatID)
	assert.Equal(t, int64(456), session.UserID)
	assert.Equal(t, handlers.SessionTypeChoreCreation, session.Type)

	// Set and get data
	session.SetData("key1", "value1")
	val, ok := session.GetData("key1")
	assert.True(t, ok)
	assert.Equal(t, "value1", val)

	// End session
	sm.EndSession(123)
	_, exists = sm.GetSession(123)
	assert.False(t, exists)
}

func TestHandleChoreDoneCallback_Success(t *testing.T) {
	mockStore := new(mocks.MockStore)
	mockStore.On("GetActiveChores", mock.Anything).Return([]*store.Chore{}, nil)
	mockScheduler := new(mocks.MockScheduler)
	groupID := int64(-1001234567890)
	h := handlers.NewWithAdminID(mockStore, mockScheduler, groupID, 123)

	// Mock Bot API client
	client := NewTestClient(func(req *http.Request) *http.Response {
		if strings.Contains(req.URL.String(), "sendMessage") {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(`{"ok":true, "result": {"message_id": 1, "chat": {"id": -1001234567890}}}`)),
				Header:     make(http.Header),
			}
		}
		if strings.Contains(req.URL.String(), "getMe") {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(`{"ok":true, "result": {"id": 123456, "is_bot": true, "first_name": "TestBot"}}`)),
				Header:     make(http.Header),
			}
		}
		return &http.Response{StatusCode: 404, Body: io.NopCloser(bytes.NewBufferString(`{}`)), Header: make(http.Header)}
	})

	bot, err := tgbotapi.NewBotAPIWithClient("TOKEN", tgbotapi.APIEndpoint, client)
	assert.NoError(t, err)
	h.SetBot(bot)

	// The ChoreReminderManager is initialized when SetBot is called
	// For this test, we test the case where the assignment doesn't exist

	callbackQuery := &tgbotapi.CallbackQuery{
		ID:   "callback1",
		Data: "chore_done:test_123",
		From: &tgbotapi.User{ID: 111},
		Message: &tgbotapi.Message{
			Chat:      &tgbotapi.Chat{ID: 111},
			MessageID: 999,
		},
	}

	edit, err := h.HandleChoreDoneCallback(callbackQuery)
	// Will fail because assignment doesn't exist, but we can check the error handling
	assert.NoError(t, err)
	assert.Contains(t, edit.Text, "not found")
}

func TestHandleChoreRemindCallback_Success(t *testing.T) {
	mockStore := new(mocks.MockStore)
	mockStore.On("GetActiveChores", mock.Anything).Return([]*store.Chore{}, nil)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.NewWithAdminID(mockStore, mockScheduler, 0, 123)

	// Mock Bot API client
	client := NewTestClient(func(req *http.Request) *http.Response {
		if strings.Contains(req.URL.String(), "getMe") {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(`{"ok":true, "result": {"id": 123456, "is_bot": true, "first_name": "TestBot"}}`)),
				Header:     make(http.Header),
			}
		}
		return &http.Response{StatusCode: 404, Body: io.NopCloser(bytes.NewBufferString(`{}`)), Header: make(http.Header)}
	})

	bot, err := tgbotapi.NewBotAPIWithClient("TOKEN", tgbotapi.APIEndpoint, client)
	assert.NoError(t, err)
	h.SetBot(bot)

	callbackQuery := &tgbotapi.CallbackQuery{
		ID:   "callback2",
		Data: "chore_remind:test_456",
		From: &tgbotapi.User{ID: 111},
		Message: &tgbotapi.Message{
			Chat:      &tgbotapi.Chat{ID: 111},
			MessageID: 999,
		},
	}

	edit, err := h.HandleChoreRemindCallback(callbackQuery)
	assert.NoError(t, err)
	// Will show "not found" since we didn't create the assignment
	assert.Contains(t, edit.Text, "not found")
}

func TestHandleChoreDoneCallback_InvalidData(t *testing.T) {
	mockStore := new(mocks.MockStore)
	mockStore.On("GetActiveChores", mock.Anything).Return([]*store.Chore{}, nil)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.NewWithAdminID(mockStore, mockScheduler, 0, 123)

	callbackQuery := &tgbotapi.CallbackQuery{
		ID:   "callback3",
		Data: "chore_done", // Missing reminderID
		From: &tgbotapi.User{ID: 111},
		Message: &tgbotapi.Message{
			Chat:      &tgbotapi.Chat{ID: 111},
			MessageID: 999,
		},
	}

	edit, err := h.HandleChoreDoneCallback(callbackQuery)
	assert.NoError(t, err)
	assert.Contains(t, edit.Text, "Invalid callback data")
}

func TestHandleChoreRemindCallback_InvalidData(t *testing.T) {
	mockStore := new(mocks.MockStore)
	mockStore.On("GetActiveChores", mock.Anything).Return([]*store.Chore{}, nil)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.NewWithAdminID(mockStore, mockScheduler, 0, 123)

	callbackQuery := &tgbotapi.CallbackQuery{
		ID:   "callback4",
		Data: "chore_remind", // Missing reminderID
		From: &tgbotapi.User{ID: 111},
		Message: &tgbotapi.Message{
			Chat:      &tgbotapi.Chat{ID: 111},
			MessageID: 999,
		},
	}

	edit, err := h.HandleChoreRemindCallback(callbackQuery)
	assert.NoError(t, err)
	assert.Contains(t, edit.Text, "Invalid callback data")
}

func TestHandleChoreInteractive_NoDoubleEscaping(t *testing.T) {
	mockStore := new(mocks.MockStore)
	mockStore.On("GetActiveChores", mock.Anything).Return([]*store.Chore{}, nil)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.NewWithAdminID(mockStore, mockScheduler, 0, 123)

	// Enter interactive mode
	h.SessionManager.StartSession(789, 123, handlers.SessionTypeChoreCreation)

	// Setup mock data
	activeUsers := []*store.User{
		{ID: 10, TelegramUserID: 111, FirstName: "Alice", IsActive: true},
	}
	mockStore.On("ListActiveUsers", mock.Anything).Return(activeUsers, nil)
	mockStore.On("GetOffDutyUsers", mock.Anything, mock.Anything).Return([]*store.User{}, nil)
	mockStore.On("CreateChore", mock.Anything, mock.Anything).Return(nil)

	// Send a message with HTML special characters
	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 789},
		From: &tgbotapi.User{ID: 123},
		Text: "<script>alert('test')</script>",
	}

	msg, err := h.HandleChoreInteractive(message)
	assert.NoError(t, err)

	// The response should contain the escaped version ONCE (not double-escaped)
	// Single escape: &lt;script&gt;
	// Double escape would be: &amp;lt;script&amp;gt;
	assert.NotContains(t, msg.(tgbotapi.MessageConfig).Text, "&amp;lt;")
	assert.NotContains(t, msg.(tgbotapi.MessageConfig).Text, "&amp;gt;")
}

func TestGenerateReminderID_NoCollisions(t *testing.T) {
	// Generate multiple IDs for the same user in rapid succession
	userID := int64(123)
	ids := make(map[string]bool)

	for i := 0; i < 1000; i++ {
		id := handlers.GenerateReminderID(userID, time.Now())
		// Check for duplicates
		assert.False(t, ids[id], "Found duplicate reminder ID: %s", id)
		ids[id] = true
	}

	// All IDs should be unique
	assert.Equal(t, 1000, len(ids))
}

func TestSendInitialDM_IncludesDescriptionAndButtons(t *testing.T) {
	var sendMessageForm url.Values

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
			sendMessageForm = form
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(`{"ok":true, "result": {"message_id": 1}}`)),
				Header:     make(http.Header),
			}
		}
		return &http.Response{StatusCode: 404, Body: io.NopCloser(bytes.NewBufferString(`{}`)), Header: make(http.Header)}
	})

	bot, err := tgbotapi.NewBotAPIWithClient("TOKEN", tgbotapi.APIEndpoint, client)
	assert.NoError(t, err)

	crm := handlers.NewChoreReminderManager(bot, nil, 0)
	assignment := &handlers.ChoreAssignment{
		UserID:      777,
		UserName:    "Vasiliy",
		Description: "Clean coffee machine",
		AssignedAt:  time.Now(),
		GroupID:     -1001234567890,
		ReminderID:  "reminder_123",
	}

	err = crm.SendInitialDM(assignment)
	assert.NoError(t, err)

	assert.Equal(t, "777", sendMessageForm.Get("chat_id"))
	assert.Contains(t, sendMessageForm.Get("text"), "Clean coffee machine")
	assert.Contains(t, sendMessageForm.Get("reply_markup"), "chore_done:reminder_123")
	assert.Contains(t, sendMessageForm.Get("reply_markup"), "chore_remind:reminder_123")
}

func TestHandleChore_WeightedSelection(t *testing.T) {
	// This test verifies that the weighted selection works correctly
	// by running multiple iterations and checking distribution
	mockStore := new(mocks.MockStore)
	mockStore.On("GetActiveChores", mock.Anything).Return([]*store.Chore{}, nil)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.NewWithAdminID(mockStore, mockScheduler, 0, 123)

	// Create users with different AdminQueueDays
	// Alice: 0 days (weight = 1.0)
	// Bob: 50 days (weight = 1.0 + 50*0.02 = 2.0)
	// Charlie: 100 days (weight = 1.0 + 100*0.02 = 3.0)
	// Total weight = 6.0
	// Expected probabilities: Alice ~16.7%, Bob ~33.3%, Charlie ~50%
	activeUsers := []*store.User{
		{ID: 10, FirstName: "Alice", IsActive: true, AdminQueueDays: 0},
		{ID: 11, FirstName: "Bob", IsActive: true, AdminQueueDays: 50},
		{ID: 12, FirstName: "Charlie", IsActive: true, AdminQueueDays: 100},
	}

	mockStore.On("ListActiveUsers", mock.Anything).Return(activeUsers, nil)
	mockStore.On("GetOffDutyUsers", mock.Anything, mock.Anything).Return([]*store.User{}, nil)
	mockStore.On("CreateChore", mock.Anything, mock.Anything).Return(nil)

	// Run 1000 iterations to check distribution
	counts := make(map[string]int)
	iterations := 1000

	for i := 0; i < iterations; i++ {
		message := &tgbotapi.Message{
			Chat:     &tgbotapi.Chat{ID: 789},
			From:     &tgbotapi.User{ID: 123},
			Text:     "/chore Clean kitchen",
			Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 6}},
		}

		msg, err := h.HandleChore(message)
		assert.NoError(t, err)

		// Count which user was selected
		if strings.Contains(msg.Text, "Alice") {
			counts["Alice"]++
		} else if strings.Contains(msg.Text, "Bob") {
			counts["Bob"]++
		} else if strings.Contains(msg.Text, "Charlie") {
			counts["Charlie"]++
		}
	}

	t.Logf("Distribution over %d iterations:", iterations)
	t.Logf("Alice (0 days, weight 1.0): %d (%.1f%%, expected ~16.7%%)", counts["Alice"], float64(counts["Alice"])/float64(iterations)*100)
	t.Logf("Bob (50 days, weight 2.0): %d (%.1f%%, expected ~33.3%%)", counts["Bob"], float64(counts["Bob"])/float64(iterations)*100)
	t.Logf("Charlie (100 days, weight 3.0): %d (%.1f%%, expected ~50%%)", counts["Charlie"], float64(counts["Charlie"])/float64(iterations)*100)

	// Verify that:
	// 1. All users were occasionally selected (non-zero counts)
	assert.Greater(t, counts["Alice"], 0, "Alice should be selected at least once")
	assert.Greater(t, counts["Bob"], 0, "Bob should be selected at least once")
	assert.Greater(t, counts["Charlie"], 0, "Charlie should be selected at least once")

	// 2. Charlie (highest weight) should be selected most often
	assert.Greater(t, counts["Charlie"], counts["Bob"], "Charlie should be selected more than Bob")
	assert.Greater(t, counts["Bob"], counts["Alice"], "Bob should be selected more than Alice")

	// 3. Rough distribution check (allowing for statistical variance)
	// Charlie should get roughly 50% (+/- 10% with 1000 iterations is reasonable)
	charliePercent := float64(counts["Charlie"]) / float64(iterations) * 100
	assert.Greater(t, charliePercent, 40.0, "Charlie should get at least 40%")
	assert.Less(t, charliePercent, 60.0, "Charlie should get at most 60%")

	// Bob should get roughly 33% (+/- 10%)
	bobPercent := float64(counts["Bob"]) / float64(iterations) * 100
	assert.Greater(t, bobPercent, 23.0, "Bob should get at least 23%")
	assert.Less(t, bobPercent, 43.0, "Bob should get at most 43%")

	// Ensure total is correct
	total := counts["Alice"] + counts["Bob"] + counts["Charlie"]
	assert.Equal(t, iterations, total, "Total selections should equal iterations")
}
