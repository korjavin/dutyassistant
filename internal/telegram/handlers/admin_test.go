package handlers_test

import (
	"errors"
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
	h := handlers.New(mockStore, mockScheduler, 0)

	adminUser := &store.User{ID: 1, TelegramUserID: 123, IsAdmin: true}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(123)).Return(adminUser, nil).Maybe()

	return mockStore, mockScheduler, h
}

func TestAdminCommands_NotAdmin(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0)

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
	h := handlers.New(mockStore, nil, 0)

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
	_, _, h := setupAdminTest(t)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 789},
		From: &tgbotapi.User{ID: 123},
	}

	mockStore := new(mocks.MockStore)
	h = handlers.New(mockStore, nil, 0)

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
	assert.Contains(t, msg.Text, "Mark Chore as Completed")

	// Should have no buttons since the only chore had no user
	inlineKeyboard := msg.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)
	assert.Len(t, inlineKeyboard.InlineKeyboard, 0)

	mockStore.AssertExpectations(t)
}
