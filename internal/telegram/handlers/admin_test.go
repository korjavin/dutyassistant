package handlers_test

import (
	"bytes"
	"errors"
	"io"
	"net/http"
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

// setupAdminTest is a helper to create mocks and an admin user for testing.
func setupAdminTest(t *testing.T) (*mocks.MockStore, *mocks.MockScheduler, *handlers.Handlers) {
	mockStore := new(mocks.MockStore)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.New(mockStore, mockScheduler, 0, nil)

	adminUser := &store.User{ID: 1, TelegramUserID: 123, IsAdmin: true}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(123)).Return(adminUser, nil).Maybe()

	return mockStore, mockScheduler, h
}

func TestAdminCommands_NotAdmin(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	nonAdminUser := &store.User{ID: 2, TelegramUserID: 456, IsAdmin: false}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(456)).Return(nonAdminUser, nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 789},
		From: &tgbotapi.User{ID: 456},
	}

	testCases := []struct {
		name    string
		handler func(*tgbotapi.Message) (tgbotapi.MessageConfig, error)
	}{
		{"Assign", h.HandleAssign},
		{"Modify", h.HandleModify},
		{"Users", h.HandleUsers},
		{"ToggleActive", h.HandleToggleActive},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			msg, err := tc.handler(message)
			assert.NoError(t, err)
			assert.Equal(t, "Sorry, this command is for admins only.", msg.Text)
		})
	}
}

func TestHandleAssign_Success(t *testing.T) {
	mockStore, mockScheduler, h := setupAdminTest(t)

	message := &tgbotapi.Message{
		Chat:     &tgbotapi.Chat{ID: 789},
		From:     &tgbotapi.User{ID: 123},
		Text:     "/assign TestUser 3",
		Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 7}},
	}

	targetUser := &store.User{ID: 2, FirstName: "TestUser"}
	// dutyDate, _ := time.Parse("2006-01-02", "2023-12-25") // Removed
	mockStore.On("GetUserByName", mock.Anything, "TestUser").Return(targetUser, nil)
	mockScheduler.On("AssignDuty", mock.Anything, targetUser, 3).Return(nil)

	msg, err := h.HandleAssign(message)
	assert.NoError(t, err)
	// assert.Equal(t, "Successfully assigned TestUser to duty on 2023-12-25.", msg.Text) // Old message
	assert.Contains(t, msg.Text, "Successfully added 3 day(s) to admin queue")
	mockStore.AssertExpectations(t)
	mockScheduler.AssertExpectations(t)
}

func TestHandleUsers_Success(t *testing.T) {
	mockStore, mockScheduler, h := setupAdminTest(t) // Capture mockScheduler

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 789},
		From: &tgbotapi.User{ID: 123},
	}

	userList := []*store.User{
		{FirstName: "Alice", IsActive: true, IsAdmin: true},
		{FirstName: "Bob", IsActive: false, IsAdmin: false},
	}
	mockStore.On("ListAllUsers", mock.Anything).Return(userList, nil)
	// Mock scheduler for vacation mode check
	mockScheduler.On("IsVacationMode", mock.Anything).Return(false, nil) // Add this

	msg, err := h.HandleUsers(message)
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "📋 User List")
	assert.Contains(t, msg.Text, "Alice</b> 👑: ✅ Active")
	assert.Contains(t, msg.Text, "Bob</b>: ❌ Inactive")
	assert.Equal(t, tgbotapi.ModeHTML, msg.ParseMode)
	mockStore.AssertExpectations(t)
}

func TestHandleToggleActive_Success(t *testing.T) {
	mockStore, _, h := setupAdminTest(t)

	message := &tgbotapi.Message{
		Chat:     &tgbotapi.Chat{ID: 789},
		From:     &tgbotapi.User{ID: 123},
		Text:     "/toggle_active Bob",
		Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 14}},
	}

	bob := &store.User{ID: 2, FirstName: "Bob", IsActive: true}
	mockStore.On("GetUserByName", mock.Anything, "Bob").Return(bob, nil)
	mockStore.On("UpdateUser", mock.Anything, mock.MatchedBy(func(u *store.User) bool {
		return u.ID == 2 && !u.IsActive // Check that IsActive is toggled to false
	})).Return(nil)

	msg, err := h.HandleToggleActive(message)
	assert.NoError(t, err)
	assert.Equal(t, "Successfully set status for Bob to Inactive.", msg.Text)
	mockStore.AssertExpectations(t)
}

func TestHandleAssign_UserNotFound(t *testing.T) {
	mockStore, _, h := setupAdminTest(t)

	message := &tgbotapi.Message{
		Chat:     &tgbotapi.Chat{ID: 789},
		From:     &tgbotapi.User{ID: 123},
		Text:     "/assign UnknownUser 3",
		Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 7}},
	}

	mockStore.On("GetUserByName", mock.Anything, "UnknownUser").Return(nil, errors.New("not found"))
	// Also Mock ListActiveUsers which is called for suggestions
	mockStore.On("ListActiveUsers", mock.Anything).Return([]*store.User{}, nil)

	msg, err := h.HandleAssign(message)
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "User 'UnknownUser' not found")
	mockStore.AssertExpectations(t)
}

func TestHandleAssign_InvalidDays(t *testing.T) {
	_, _, h := setupAdminTest(t)

	message := &tgbotapi.Message{
		Chat:     &tgbotapi.Chat{ID: 789},
		From:     &tgbotapi.User{ID: 123},
		Text:     "/assign TestUser abc", // Invalid days
		Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 7}},
	}

	msg, err := h.HandleAssign(message)
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "is not a valid number of days")
}

func TestHandleComplete_NotAdmin(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	nonAdminUser := &store.User{ID: 2, TelegramUserID: 456, IsAdmin: false}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(456)).Return(nonAdminUser, nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 789},
		From: &tgbotapi.User{ID: 456},
	}

	msg, err := h.HandleComplete(message)
	assert.NoError(t, err)
	assert.Equal(t, "Sorry, this command is for admins only.", msg.Text)
}

func TestHandleComplete_NoActiveChores(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 789},
		From: &tgbotapi.User{ID: 123},
	}

	adminUser := &store.User{ID: 1, TelegramUserID: 123, IsAdmin: true}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(123)).Return(adminUser, nil)
	mockStore.On("GetActiveChores", mock.Anything).Return([]*store.Chore{}, nil)

	msg, err := h.HandleComplete(message)
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "No active chores found")
	mockStore.AssertExpectations(t)
}

func TestHandleComplete_WithActiveChores(t *testing.T) {
	mockStore, _, h := setupAdminTest(t)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 789},
		From: &tgbotapi.User{ID: 123},
	}

	// Create sample chores
	now := time.Now()
	deadline := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, time.UTC)

	alice := &store.User{ID: 2, FirstName: "Alice", TelegramUserID: 222}
	bob := &store.User{ID: 3, FirstName: "Bob", TelegramUserID: 333}

	chores := []*store.Chore{
		{
			ID:          1,
			UserID:      2,
			Description: "Clean the kitchen",
			AssignedAt:  now,
			DeadlineAt:  deadline,
			ReminderID:  "reminder_1",
			User:        alice,
		},
		{
			ID:          2,
			UserID:      3,
			Description: "Take out trash",
			AssignedAt:  now,
			DeadlineAt:  deadline,
			ReminderID:  "reminder_2",
			User:        bob,
		},
	}

	mockStore.On("GetActiveChores", mock.Anything).Return(chores, nil)

	msg, err := h.HandleComplete(message)
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "Mark Chore as Completed")
	assert.Contains(t, msg.Text, "Select a chore")
	assert.Equal(t, tgbotapi.ModeHTML, msg.ParseMode)

	// Check inline keyboard buttons
	assert.NotNil(t, msg.ReplyMarkup)
	inlineKeyboard := msg.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)
	assert.Len(t, inlineKeyboard.InlineKeyboard, 2) // 2 chores = 2 buttons

	// Check first button
	button1 := inlineKeyboard.InlineKeyboard[0][0]
	assert.Equal(t, "Alice - Clean the kitchen @23:59", button1.Text)
	if button1.CallbackData != nil {
		assert.Equal(t, "complete_chore:reminder_1", *button1.CallbackData)
	} else {
		t.Fatal("CallbackData is nil for button1")
	}

	// Check second button
	button2 := inlineKeyboard.InlineKeyboard[1][0]
	assert.Equal(t, "Bob - Take out trash @23:59", button2.Text)
	if button2.CallbackData != nil {
		assert.Equal(t, "complete_chore:reminder_2", *button2.CallbackData)
	} else {
		t.Fatal("CallbackData is nil for button2")
	}

	mockStore.AssertExpectations(t)
}

func TestHandleComplete_StoreError(t *testing.T) {
	mockStore, _, h := setupAdminTest(t)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 789},
		From: &tgbotapi.User{ID: 123},
	}

	mockStore.On("GetActiveChores", mock.Anything).Return(nil, errors.New("database error"))

	msg, err := h.HandleComplete(message)
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "Failed to retrieve active chores")
	mockStore.AssertExpectations(t)
}

func TestHandleComplete_ChoreWithoutUser(t *testing.T) {
	mockStore, _, h := setupAdminTest(t)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 789},
		From: &tgbotapi.User{ID: 123},
	}

	// Create chore without user (should be skipped)
	now := time.Now()
	deadline := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, time.UTC)

	chores := []*store.Chore{
		{
			ID:          1,
			UserID:      2,
			Description: "Chore without user",
			AssignedAt:  now,
			DeadlineAt:  deadline,
			ReminderID:  "reminder_1",
			User:        nil, // No user - should be skipped
		},
	}

	mockStore.On("GetActiveChores", mock.Anything).Return(chores, nil)

	msg, err := h.HandleComplete(message)
	assert.NoError(t, err)
	// All chores had no assigned user, so treated same as no active chores
	assert.Contains(t, msg.Text, "No active chores found")
	assert.Nil(t, msg.ReplyMarkup)

	mockStore.AssertExpectations(t)
}

func TestHandleCompleteChoreCallback_Success(t *testing.T) {
	mockStore := new(mocks.MockStore)
	mockStore.On("GetActiveChores", mock.Anything).Return([]*store.Chore{}, nil)
	mockScheduler := new(mocks.MockScheduler)
	groupID := int64(-1001234567890)
	h := handlers.NewWithAdminID(mockStore, mockScheduler, groupID, 123, nil)

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
	callback := &tgbotapi.CallbackQuery{
		ID:   "callback_123",
		From: &tgbotapi.User{ID: 123},
		Message: &tgbotapi.Message{
			Chat:      &tgbotapi.Chat{ID: 789},
			MessageID: 456,
		},
		Data: "complete_chore:reminder_1",
	}

	// Create sample chore with user
	now := time.Now()
	alice := &store.User{ID: 2, FirstName: "Alice", TelegramUserID: 222}

	chore := &store.Chore{
		ID:          1,
		UserID:      2,
		Description: "Clean the kitchen",
		AssignedAt:  now,
		ReminderID:  "reminder_1",
		User:        alice,
	}

	mockStore.On("GetChoreByReminderID", mock.Anything, "reminder_1").Return(chore, nil)
	mockStore.On("CompleteChoreByReminderID", mock.Anything, "reminder_1").Return(nil)

	edit, err := h.HandleCompleteChoreCallback(callback)
	assert.NoError(t, err)
	assert.Contains(t, edit.Text, "Chore Completed!")
	assert.Contains(t, edit.Text, "Alice")
	assert.Contains(t, edit.Text, "Clean the kitchen")
	assert.Equal(t, tgbotapi.ModeHTML, edit.ParseMode)
	assert.Equal(t, int64(789), edit.ChatID)
	assert.Equal(t, 456, edit.MessageID)

	mockStore.AssertExpectations(t)
}

func TestHandleCompleteChoreCallback_InvalidCallbackData(t *testing.T) {
	mockStore := new(mocks.MockStore)
	mockStore.On("GetActiveChores", mock.Anything).Return([]*store.Chore{}, nil)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.NewWithAdminID(mockStore, mockScheduler, 0, 123, nil)

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

	callback := &tgbotapi.CallbackQuery{
		ID:   "callback_123",
		From: &tgbotapi.User{ID: 123},
		Message: &tgbotapi.Message{
			Chat:      &tgbotapi.Chat{ID: 789},
			MessageID: 456,
		},
		Data: "invalid_data", // Invalid format
	}

	edit, err := h.HandleCompleteChoreCallback(callback)
	assert.NoError(t, err)
	assert.Contains(t, edit.Text, "Error: Invalid callback data.")
	assert.Equal(t, int64(789), edit.ChatID)
	assert.Equal(t, 456, edit.MessageID)
}

func TestHandleCompleteChoreCallback_ChoreNotFound(t *testing.T) {
	mockStore := new(mocks.MockStore)
	mockStore.On("GetActiveChores", mock.Anything).Return([]*store.Chore{}, nil)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.NewWithAdminID(mockStore, mockScheduler, 0, 123, nil)

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

	callback := &tgbotapi.CallbackQuery{
		ID:   "callback_123",
		From: &tgbotapi.User{ID: 123},
		Message: &tgbotapi.Message{
			Chat:      &tgbotapi.Chat{ID: 789},
			MessageID: 456,
		},
		Data: "complete_chore:nonexistent",
	}

	mockStore.On("GetChoreByReminderID", mock.Anything, "nonexistent").Return(nil, errors.New("not found"))

	edit, err := h.HandleCompleteChoreCallback(callback)
	assert.NoError(t, err)
	assert.Contains(t, edit.Text, "Error: Could not find chore.")

	mockStore.AssertExpectations(t)
}

func TestHandleCompleteChoreCallback_ChoreWithoutUser(t *testing.T) {
	mockStore := new(mocks.MockStore)
	mockStore.On("GetActiveChores", mock.Anything).Return([]*store.Chore{}, nil)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.NewWithAdminID(mockStore, mockScheduler, 0, 123, nil)

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

	callback := &tgbotapi.CallbackQuery{
		ID:   "callback_123",
		From: &tgbotapi.User{ID: 123},
		Message: &tgbotapi.Message{
			Chat:      &tgbotapi.Chat{ID: 789},
			MessageID: 456,
		},
		Data: "complete_chore:reminder_no_user",
	}

	// Create chore without user
	now := time.Now()
	chore := &store.Chore{
		ID:          1,
		UserID:      2,
		Description: "Orphaned chore",
		AssignedAt:  now,
		ReminderID:  "reminder_no_user",
		User:        nil,
	}

	mockStore.On("GetChoreByReminderID", mock.Anything, "reminder_no_user").Return(chore, nil)

	edit, err := h.HandleCompleteChoreCallback(callback)
	assert.NoError(t, err)
	assert.Contains(t, edit.Text, "Error: Chore not found or has no assigned user.")

	mockStore.AssertExpectations(t)
}

func TestHandleCompleteChoreCallback_DBFailure(t *testing.T) {
	mockStore := new(mocks.MockStore)
	mockStore.On("GetActiveChores", mock.Anything).Return([]*store.Chore{}, nil)
	mockScheduler := new(mocks.MockScheduler)
	groupID := int64(-1001234567890)
	h := handlers.NewWithAdminID(mockStore, mockScheduler, groupID, 123, nil)
	sendMessageCalls := 0

	client := NewTestClient(func(req *http.Request) *http.Response {
		if strings.Contains(req.URL.String(), "sendMessage") {
			sendMessageCalls++
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

	callback := &tgbotapi.CallbackQuery{
		ID:   "callback_123",
		From: &tgbotapi.User{ID: 123},
		Message: &tgbotapi.Message{
			Chat:      &tgbotapi.Chat{ID: 789},
			MessageID: 456,
		},
		Data: "complete_chore:reminder_1",
	}

	now := time.Now()
	alice := &store.User{ID: 2, FirstName: "Alice", TelegramUserID: 222}
	chore := &store.Chore{
		ID:          1,
		UserID:      2,
		Description: "Clean the kitchen",
		AssignedAt:  now,
		ReminderID:  "reminder_1",
		User:        alice,
	}

	mockStore.On("GetChoreByReminderID", mock.Anything, "reminder_1").Return(chore, nil)
	mockStore.On("CompleteChoreByReminderID", mock.Anything, "reminder_1").Return(errors.New("db error"))

	edit, err := h.HandleCompleteChoreCallback(callback)
	assert.NoError(t, err)
	assert.Contains(t, edit.Text, "Error: Failed to mark chore as completed.")
	assert.Equal(t, int64(789), edit.ChatID)
	assert.Equal(t, 456, edit.MessageID)
	assert.Equal(t, 0, sendMessageCalls)

	mockStore.AssertExpectations(t)
}

func TestHandleCompleteChoreCallback_NilChore(t *testing.T) {
	mockStore := new(mocks.MockStore)
	mockStore.On("GetActiveChores", mock.Anything).Return([]*store.Chore{}, nil)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.NewWithAdminID(mockStore, mockScheduler, 0, 123, nil)

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

	callback := &tgbotapi.CallbackQuery{
		ID:   "callback_123",
		From: &tgbotapi.User{ID: 123},
		Message: &tgbotapi.Message{
			Chat:      &tgbotapi.Chat{ID: 789},
			MessageID: 456,
		},
		Data: "complete_chore:nil_chore",
	}

	mockStore.On("GetChoreByReminderID", mock.Anything, "nil_chore").Return(nil, nil)

	edit, err := h.HandleCompleteChoreCallback(callback)
	assert.NoError(t, err)
	// When chore is nil, we return a generic error message
	assert.Contains(t, edit.Text, "Error: Chore not found or has no assigned user.")

	mockStore.AssertExpectations(t)
}

func TestHandleCompleteChoreCallback_NotAdmin(t *testing.T) {
	mockStore := new(mocks.MockStore)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.NewWithAdminID(mockStore, mockScheduler, 0, 123, nil)

	callback := &tgbotapi.CallbackQuery{
		ID:   "callback_123",
		From: &tgbotapi.User{ID: 456},
		Message: &tgbotapi.Message{
			Chat:      &tgbotapi.Chat{ID: 789},
			MessageID: 456,
		},
		Data: "complete_chore:reminder_1",
	}

	edit, err := h.HandleCompleteChoreCallback(callback)
	assert.NoError(t, err)
	assert.Equal(t, "Sorry, this command is for admins only.", edit.Text)

	mockStore.AssertNotCalled(t, "GetChoreByReminderID", mock.Anything, mock.Anything)
	mockStore.AssertNotCalled(t, "CompleteChoreByReminderID", mock.Anything, mock.Anything)
}
