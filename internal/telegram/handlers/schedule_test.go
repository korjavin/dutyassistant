package handlers_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/korjavin/dutyassistant/internal/mocks"
	"github.com/korjavin/dutyassistant/internal/telegram/handlers"
	"github.com/korjavin/dutyassistant/internal/telegram/keyboard"
	"github.com/stretchr/testify/assert"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func TestHandleSchedule(t *testing.T) {
	// Setup
	mockStore := new(mocks.MockStore)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.New(mockStore, mockScheduler)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
	}

	// Execute
	msg, err := h.HandleSchedule(message)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, int64(123), msg.ChatID)
	assert.Contains(t, msg.Text, "Here is the duty schedule for")

	// Check that a keyboard is attached
	assert.NotNil(t, msg.ReplyMarkup)
	keyboardMarkup, ok := msg.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)
	assert.True(t, ok)
	assert.NotEmpty(t, keyboardMarkup.InlineKeyboard)
}

func TestHandleCalendarCallback_NextMonth(t *testing.T) {
	// Setup
	mockStore := new(mocks.MockStore)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.New(mockStore, mockScheduler)

	now := time.Now()
	callbackData := fmt.Sprintf("%s:%s", keyboard.ActionNextMonth, now.Format("2006-01"))
	callbackQuery := &tgbotapi.CallbackQuery{
		ID:   "test_callback_id",
		From: &tgbotapi.User{ID: 456},
		Message: &tgbotapi.Message{
			Chat:      &tgbotapi.Chat{ID: 123},
			MessageID: 789,
		},
		Data: callbackData,
	}

	// Execute
	editMsg, err := h.HandleCalendarCallback(callbackQuery)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, int64(123), editMsg.ChatID)
	assert.Equal(t, 789, editMsg.MessageID)

	nextMonth := now.AddDate(0, 1, 0)
	assert.Contains(t, editMsg.Text, nextMonth.Format("January 2006"))

	// Check that the keyboard is updated
	assert.NotNil(t, editMsg.ReplyMarkup)
}

func TestHandleCalendarCallback_PrevMonth(t *testing.T) {
	// Setup
	mockStore := new(mocks.MockStore)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.New(mockStore, mockScheduler)

	now := time.Now()
	callbackData := fmt.Sprintf("%s:%s", keyboard.ActionPrevMonth, now.Format("2006-01"))
	callbackQuery := &tgbotapi.CallbackQuery{
		ID:   "test_callback_id",
		From: &tgbotapi.User{ID: 456},
		Message: &tgbotapi.Message{
			Chat:      &tgbotapi.Chat{ID: 123},
			MessageID: 789,
		},
		Data: callbackData,
	}

	// Execute
	editMsg, err := h.HandleCalendarCallback(callbackQuery)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, int64(123), editMsg.ChatID)
	assert.Equal(t, 789, editMsg.MessageID)

	prevMonth := now.AddDate(0, -1, 0)
	assert.Contains(t, editMsg.Text, prevMonth.Format("January 2006"))
	assert.NotNil(t, editMsg.ReplyMarkup)
}