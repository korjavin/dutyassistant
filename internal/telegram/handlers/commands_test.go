package handlers_test

import (
	"errors"
	"testing"

	"github.com/korjavin/dutyassistant/internal/mocks"
	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/korjavin/dutyassistant/internal/telegram/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func TestHandleStart(t *testing.T) {
	// Setup
	mockStore := new(mocks.MockStore)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.New(mockStore, mockScheduler)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
	}

	// Execute
	msg, err := h.HandleStart(message)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, int64(123), msg.ChatID)
	assert.Contains(t, msg.Text, "Welcome to the Roster Bot!")
}

func TestHandleHelp(t *testing.T) {
	// Setup
	mockStore := new(mocks.MockStore)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.New(mockStore, mockScheduler)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
	}

	// Execute
	msg, err := h.HandleHelp(message)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, int64(123), msg.ChatID)
	assert.Contains(t, msg.Text, "Here are the available commands:")
	assert.Equal(t, tgbotapi.ModeMarkdown, msg.ParseMode)
}


func TestHandleStatus_Success(t *testing.T) {
	// Setup
	mockStore := new(mocks.MockStore)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.New(mockStore, mockScheduler)

	fromUser := &tgbotapi.User{ID: 456, FirstName: "TestUser"}
	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
		From: fromUser,
	}

	// Mock expectations
	expectedUser := &store.User{ID: 1, TelegramUserID: 456}
	expectedStats := &store.UserStats{TotalDuties: 5, DutiesThisMonth: 2, NextDutyDate: "2023-12-31"}
	mockStore.On("GetUserByTelegramID", mock.Anything, fromUser.ID).Return(expectedUser, nil)
	mockStore.On("GetUserStats", mock.Anything, expectedUser.ID).Return(expectedStats, nil)

	// Execute
	msg, err := h.HandleStatus(message)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, int64(123), msg.ChatID)
	assert.Contains(t, msg.Text, "Duty Status for TestUser")
	assert.Contains(t, msg.Text, "Total Duties Assigned: 5")
	assert.Contains(t, msg.Text, "Duties this month: 2")
	assert.Contains(t, msg.Text, "Next scheduled duty: 2023-12-31")
	mockStore.AssertExpectations(t)
}

func TestHandleStatus_UserNotFound(t *testing.T) {
	// Setup
	mockStore := new(mocks.MockStore)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.New(mockStore, mockScheduler)

	fromUser := &tgbotapi.User{ID: 456, FirstName: "TestUser"}
	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
		From: fromUser,
	}

	// Mock expectations
	mockStore.On("GetUserByTelegramID", mock.Anything, fromUser.ID).Return(nil, errors.New("not found"))

	// Execute
	msg, err := h.HandleStatus(message)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "Could not find your user profile. Are you registered?", msg.Text)
	mockStore.AssertExpectations(t)
}

func TestHandleStatus_StatsFailure(t *testing.T) {
	// Setup
	mockStore := new(mocks.MockStore)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.New(mockStore, mockScheduler)

	fromUser := &tgbotapi.User{ID: 456, FirstName: "TestUser"}
	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
		From: fromUser,
	}

	// Mock expectations
	expectedUser := &store.User{ID: 1, TelegramUserID: 456}
	mockStore.On("GetUserByTelegramID", mock.Anything, fromUser.ID).Return(expectedUser, nil)
	mockStore.On("GetUserStats", mock.Anything, expectedUser.ID).Return(nil, errors.New("db error"))

	// Execute
	msg, err := h.HandleStatus(message)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "Sorry, I couldn't retrieve your stats at this time.", msg.Text)
	mockStore.AssertExpectations(t)
}