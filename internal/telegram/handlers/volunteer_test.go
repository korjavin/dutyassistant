package handlers_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/korjavin/dutyassistant/internal/mocks"
	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/korjavin/dutyassistant/internal/telegram/handlers"
	"github.com/korjavin/dutyassistant/internal/telegram/keyboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func TestHandleVolunteer(t *testing.T) {
	// Setup
	mockStore := new(mocks.MockStore)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.New(mockStore, mockScheduler)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
	}

	// Execute
	msg, err := h.HandleVolunteer(message)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, int64(123), msg.ChatID)
	assert.Equal(t, "Please select a date to volunteer for duty.", msg.Text)
	assert.NotNil(t, msg.ReplyMarkup) // Should have a calendar
}

func TestHandleVolunteerCallback_Success(t *testing.T) {
	// Setup
	mockStore := new(mocks.MockStore)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.New(mockStore, mockScheduler)

	dateStr := time.Now().Format("2006-01-02")
	callbackData := fmt.Sprintf("%s:%s", keyboard.ActionSelectDay, dateStr)
	fromUser := &tgbotapi.User{ID: 456, FirstName: "Test"}
	callbackQuery := &tgbotapi.CallbackQuery{
		ID:      "test_callback_id",
		From:    fromUser,
		Message: &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: 123}, MessageID: 789},
		Data:    callbackData,
	}

	// Mock expectations
	expectedUser := &store.User{ID: 1, TelegramUserID: 456, FirstName: "Test"}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(456)).Return(expectedUser, nil)
	mockScheduler.On("VolunteerForDuty", mock.Anything, expectedUser, dateStr).Return(nil)

	// Execute
	editMsg, err := h.HandleVolunteerCallback(callbackQuery)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, int64(123), editMsg.ChatID)
	assert.Equal(t, 789, editMsg.MessageID)
	assert.Contains(t, editMsg.Text, "Thank you for volunteering")
	assert.Nil(t, editMsg.ReplyMarkup) // Keyboard should be removed

	// Verify that the mocks were called as expected
	mockStore.AssertExpectations(t)
	mockScheduler.AssertExpectations(t)
}

func TestHandleVolunteerCallback_SchedulerFailure(t *testing.T) {
	// Setup
	mockStore := new(mocks.MockStore)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.New(mockStore, mockScheduler)

	dateStr := time.Now().Format("2006-01-02")
	callbackData := fmt.Sprintf("%s:%s", keyboard.ActionSelectDay, dateStr)
	fromUser := &tgbotapi.User{ID: 456, FirstName: "Test"}
	callbackQuery := &tgbotapi.CallbackQuery{
		ID:      "test_callback_id",
		From:    fromUser,
		Message: &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: 123}, MessageID: 789},
		Data:    callbackData,
	}

	// Mock expectations
	expectedUser := &store.User{ID: 1, TelegramUserID: 456, FirstName: "Test"}
	schedulerError := errors.New("date is already taken")
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(456)).Return(expectedUser, nil)
	mockScheduler.On("VolunteerForDuty", mock.Anything, expectedUser, dateStr).Return(schedulerError)

	// Execute
	editMsg, err := h.HandleVolunteerCallback(callbackQuery)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, int64(123), editMsg.ChatID)
	assert.Contains(t, editMsg.Text, "Sorry, we couldn't process your request")
	mockScheduler.AssertExpectations(t)
}

func TestHandleVolunteerCallback_UserNotFound(t *testing.T) {
	// Setup
	mockStore := new(mocks.MockStore)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.New(mockStore, mockScheduler)

	dateStr := time.Now().Format("2006-01-02")
	callbackData := fmt.Sprintf("%s:%s", keyboard.ActionSelectDay, dateStr)
	fromUser := &tgbotapi.User{ID: 456, FirstName: "Test"}
	callbackQuery := &tgbotapi.CallbackQuery{
		ID:      "test_callback_id",
		From:    fromUser,
		Message: &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: 123}, MessageID: 789},
		Data:    callbackData,
	}

	// Mock expectations
	storeError := errors.New("user not found")
	mockStore.On("GetUserByTelegramID", context.Background(), int64(456)).Return(nil, storeError)

	// Execute
	editMsg, err := h.HandleVolunteerCallback(callbackQuery)

	// Assert
	assert.NoError(t, err)
	assert.Contains(t, editMsg.Text, "Could not find user")
	mockStore.AssertExpectations(t)
	mockScheduler.AssertNotCalled(t, "VolunteerForDuty")
}