package handlers_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/korjavin/dutyassistant/internal/mocks"
	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/korjavin/dutyassistant/internal/telegram/handlers"
	"github.com/korjavin/dutyassistant/internal/telegram/keyboard"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestHandleSchedule(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil)
	message := &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: 123}}

	// Mock store to return some duties
	duties := []*store.Duty{
		{DutyDate: time.Now(), User: &store.User{FirstName: "Test"}},
	}
	mockStore.On("GetDutiesByMonth", mock.Anything, time.Now().Year(), time.Now().Month()).Return(duties, nil)

	msg, err := h.HandleSchedule(message)

	assert.NoError(t, err)
	assert.Equal(t, int64(123), msg.ChatID)
	assert.Contains(t, msg.Text, "Duty schedule for")
	assert.NotNil(t, msg.ReplyMarkup)
	mockStore.AssertExpectations(t)
}

func TestHandleCalendarCallback(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil)
	now := time.Date(2023, 5, 15, 0, 0, 0, 0, time.UTC)

	// Mock store to return empty duties for any month query
	mockStore.On("GetDutiesByMonth", mock.Anything, mock.Anything, mock.Anything).Return([]*store.Duty{}, nil)

	testCases := []struct {
		name          string
		action        string
		expectedMonth string
	}{
		{"Next Month", keyboard.ActionNextMonth, "June 2023"},
		{"Previous Month", keyboard.ActionPrevMonth, "April 2023"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			callbackData := fmt.Sprintf("%s:%s", tc.action, now.Format("2006-01-02"))
			callbackQuery := &tgbotapi.CallbackQuery{
				ID: "test_callback_id",
				Message: &tgbotapi.Message{
					Chat:      &tgbotapi.Chat{ID: 123},
					MessageID: 789,
				},
				Data: callbackData,
			}

			editMsg, err := h.HandleCalendarCallback(callbackQuery)

			assert.NoError(t, err)
			assert.Equal(t, int64(123), editMsg.ChatID)
			assert.Contains(t, editMsg.Text, tc.expectedMonth)
			assert.NotNil(t, editMsg.ReplyMarkup)
		})
	}
	// Assert that GetDutiesByMonth was called for both next and previous month
	mockStore.AssertNumberOfCalls(t, "GetDutiesByMonth", 2)
}