package handlers_test

import (
	"errors"
	"testing"

	"github.com/korjavin/dutyassistant/internal/mocks"
	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/korjavin/dutyassistant/internal/telegram/handlers"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestHandleStart_NewUser(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
		From: &tgbotapi.User{ID: 456, FirstName: "NewUser"},
	}

	mockStore.On("GetUserByTelegramID", mock.Anything, int64(456)).Return(nil, errors.New("not found"))
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
	h := handlers.New(mockStore, nil)

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

func TestHandleHelp(t *testing.T) {
	h := handlers.New(nil, nil)
	message := &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: 123}}

	msg, err := h.HandleHelp(message)
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "Here are the available commands:")
	assert.Equal(t, tgbotapi.ModeMarkdown, msg.ParseMode)
}

func TestHandleStatus_Success(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil)

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
	assert.Contains(t, msg.Text, "Total Duties Assigned: 5")
	assert.Contains(t, msg.Text, "Next scheduled duty: 2023-12-31")
	mockStore.AssertExpectations(t)
}

func TestHandleStatus_UserNotFound(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil)

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