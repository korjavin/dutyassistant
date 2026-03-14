package handlers_test

import (
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/mocks"
	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/korjavin/dutyassistant/internal/telegram/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestHandleStart_NewUser(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
		From: &tgbotapi.User{ID: 456, FirstName: "NewUser"},
	}

	mockStore.On("GetUserByTelegramID", mock.Anything, int64(456)).Return(nil, nil)
	mockStore.On("CreateUser", mock.Anything, mock.MatchedBy(func(u *store.User) bool {
		return u.TelegramUserID == 456 && u.FirstName == "NewUser"
	})).Return(nil)

	msg, err := h.HandleStart(message)
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "Welcome to the Roster Bot!")
	mockStore.AssertExpectations(t)
}

func TestHandleStart_ExistingUser(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
		From: &tgbotapi.User{ID: 456, FirstName: "UpdatedName"},
	}

	existingUser := &store.User{ID: 1, TelegramUserID: 456, FirstName: "OldName"}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(456)).Return(existingUser, nil)
	mockStore.On("UpdateUser", mock.Anything, mock.MatchedBy(func(u *store.User) bool {
		return u.ID == 1 && u.FirstName == "UpdatedName"
	})).Return(nil)

	msg, err := h.HandleStart(message)
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "Welcome to the Roster Bot!")
	mockStore.AssertExpectations(t)
}

func TestHandleStart_BackfillsConfiguredAdminFlagsForExistingUser(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.NewWithAdminID(mockStore, nil, 0, 456, nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
		From: &tgbotapi.User{ID: 456, FirstName: "AdminUser"},
	}

	existingUser := &store.User{
		ID:             1,
		TelegramUserID: 456,
		FirstName:      "AdminUser",
		IsAdmin:        false,
		IsActive:       true,
	}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(456)).Return(existingUser, nil)
	mockStore.On("UpdateUser", mock.Anything, mock.MatchedBy(func(u *store.User) bool {
		return u.ID == 1 && u.IsAdmin && !u.IsActive
	})).Return(nil)

	msg, err := h.HandleStart(message)
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "Welcome to the Roster Bot!")
	mockStore.AssertExpectations(t)
}

func TestHandleHelp_RegularUser(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.NewWithAdminID(mockStore, nil, 0, 999, nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
		From: &tgbotapi.User{ID: 123},
	}

	msg, err := h.HandleHelp(message)
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "/start - Show the welcome message")
	assert.NotContains(t, msg.Text, "*Admin Commands:*")
	assert.NotContains(t, msg.Text, "/cancel")
	assert.Equal(t, tgbotapi.ModeMarkdown, msg.ParseMode)
}

func TestHandleHelp_Admin(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.NewWithAdminID(mockStore, nil, 0, 123, nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
		From: &tgbotapi.User{ID: 123},
	}

	msg, err := h.HandleHelp(message)
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "/start - Show the welcome message")
	assert.Contains(t, msg.Text, "*Admin Commands:*")
	assert.Contains(t, msg.Text, "/cancel - Cancel a duty, active chore, or recurring chore.")
	assert.Contains(t, msg.Text, "🗓️ *Duty Management*")
	assert.Contains(t, msg.Text, "🧹 *Chore Management*")
	assert.Contains(t, msg.Text, "👥 *User Management*")
	assert.Contains(t, msg.Text, "/newchore")
	assert.Contains(t, msg.Text, "/editchore")
	assert.NotContains(t, msg.Text, "/toggle_active")
	assert.Contains(t, msg.Text, "/activate")
	assert.Equal(t, tgbotapi.ModeMarkdown, msg.ParseMode)
}

func TestHandleStatus_Success(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
		From: &tgbotapi.User{ID: 456, FirstName: "TestUser"},
	}

	user := &store.User{ID: 1, TelegramUserID: 456}
	stats := &store.UserStats{TotalDuties: 5, DutiesThisMonth: 2, NextDutyDate: "2023-12-31"}

	mockStore.On("GetUserByTelegramID", mock.Anything, int64(456)).Return(user, nil)
	mockStore.On("GetUserStats", mock.Anything, user.ID).Return(stats, nil)

	msg, err := h.HandleStatus(message)
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "Total duties: 5")
	assert.Contains(t, msg.Text, "Next duty: 2023-12-31")
	mockStore.AssertExpectations(t)
}

func TestHandleStatus_UserNotFound(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
		From: &tgbotapi.User{ID: 456},
	}

	mockStore.On("GetUserByTelegramID", mock.Anything, int64(456)).Return(nil, nil) // Return nil user

	msg, err := h.HandleStatus(message)
	assert.NoError(t, err)
	assert.Equal(t, "Could not find your user profile. Please use /start first.", msg.Text)
	mockStore.AssertExpectations(t)
}
