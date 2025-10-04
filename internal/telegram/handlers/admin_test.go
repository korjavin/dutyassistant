package handlers_test

import (
	"errors"
	"testing"
	"time"

	"github.com/korjavin/dutyassistant/internal/mocks"
	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/korjavin/dutyassistant/internal/telegram/handlers"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// setupAdminTest is a helper to create mocks and an admin user for testing.
func setupAdminTest(t *testing.T) (*mocks.MockStore, *mocks.MockScheduler, *handlers.Handlers) {
	mockStore := new(mocks.MockStore)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.New(mockStore, mockScheduler)

	adminUser := &store.User{ID: 1, TelegramUserID: 123, IsAdmin: true}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(123)).Return(adminUser, nil).Maybe()

	return mockStore, mockScheduler, h
}

func TestAdminCommands_NotAdmin(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil)

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
		Text:     "/assign TestUser 2023-12-25",
		Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 7}},
	}

	targetUser := &store.User{ID: 2, FirstName: "TestUser"}
	dutyDate, _ := time.Parse("2006-01-02", "2023-12-25")
	mockStore.On("GetUserByName", mock.Anything, "TestUser").Return(targetUser, nil)
	mockScheduler.On("AssignDuty", mock.Anything, targetUser, dutyDate).Return(nil)

	msg, err := h.HandleAssign(message)
	assert.NoError(t, err)
	assert.Equal(t, "Successfully assigned TestUser to duty on 2023-12-25.", msg.Text)
	mockStore.AssertExpectations(t)
	mockScheduler.AssertExpectations(t)
}

func TestHandleUsers_Success(t *testing.T) {
	mockStore, _, h := setupAdminTest(t)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 789},
		From: &tgbotapi.User{ID: 123},
	}

	userList := []*store.User{
		{FirstName: "Alice", IsActive: true, IsAdmin: true},
		{FirstName: "Bob", IsActive: false, IsAdmin: false},
	}
	mockStore.On("ListAllUsers", mock.Anything).Return(userList, nil)

	msg, err := h.HandleUsers(message)
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "<b>User List:</b>")
	assert.Contains(t, msg.Text, "- Alice (Admin): Active")
	assert.Contains(t, msg.Text, "- Bob: Inactive")
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
		Text:     "/assign UnknownUser 2023-12-25",
		Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 7}},
	}

	mockStore.On("GetUserByName", mock.Anything, "UnknownUser").Return(nil, errors.New("not found"))

	msg, err := h.HandleAssign(message)
	assert.NoError(t, err)
	assert.Equal(t, "Could not find user: UnknownUser", msg.Text)
	mockStore.AssertExpectations(t)
}

func TestHandleAssign_InvalidDate(t *testing.T) {
	_, _, h := setupAdminTest(t)

	message := &tgbotapi.Message{
		Chat:     &tgbotapi.Chat{ID: 789},
		From:     &tgbotapi.User{ID: 123},
		Text:     "/assign TestUser 2023/12/25", // Invalid date format
		Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 7}},
	}

	msg, err := h.HandleAssign(message)
	assert.NoError(t, err)
	assert.Equal(t, "Invalid date format. Please use YYYY-MM-DD.", msg.Text)
}