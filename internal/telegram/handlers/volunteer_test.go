package handlers_test

import (
	"errors"
	"fmt"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/mocks"
	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/korjavin/dutyassistant/internal/telegram/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestHandleVolunteer(t *testing.T) {
	h := handlers.New(nil, nil, 0)
	message := &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: 123}}

	msg, err := h.HandleVolunteer(message)

	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "Volunteer for duty!")
	assert.Contains(t, msg.Text, "How many days would you like to volunteer for?")
	assert.NotNil(t, msg.ReplyMarkup)
}

func TestHandleVolunteerCallback_Success(t *testing.T) {
	mockStore := new(mocks.MockStore)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.New(mockStore, mockScheduler, 0)

	// dateStr := "2023-05-20" // Removed
	// dutyDate, _ := time.Parse("2006-01-02", dateStr) // Removed
	// callbackData := fmt.Sprintf("%s:%s", keyboard.ActionSelectDay, dateStr) // Old format?
	// HandleVolunteerDaysCallback expects volunteer_days:days
	days := 5
	callbackData := fmt.Sprintf("volunteer_days:%d", days)

	user := &tgbotapi.User{ID: 456, FirstName: "Test"}
	callbackQuery := &tgbotapi.CallbackQuery{
		ID:      "test_callback_id",
		From:    user,
		Message: &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: 123}, MessageID: 789},
		Data:    callbackData,
	}

	storeUser := &store.User{ID: 1, TelegramUserID: 456}
	mockStore.On("GetUserByTelegramID", mock.Anything, user.ID).Return(storeUser, nil)
	mockScheduler.On("VolunteerForDuty", mock.Anything, storeUser, days).Return(nil)

	editMsg, err := h.HandleVolunteerDaysCallback(callbackQuery)

	assert.NoError(t, err)
	assert.Contains(t, editMsg.Text, fmt.Sprintf("Added %d day(s) to your volunteer queue", days))
	assert.Nil(t, editMsg.ReplyMarkup, "Keyboard should be removed on success")
	mockStore.AssertExpectations(t)
	mockScheduler.AssertExpectations(t)
}

func TestHandleVolunteerCallback_Failure(t *testing.T) {
	mockStore := new(mocks.MockStore)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.New(mockStore, mockScheduler, 0)

	days := 5
	callbackData := fmt.Sprintf("volunteer_days:%d", days)
	user := &tgbotapi.User{ID: 456, FirstName: "Test"}
	callbackQuery := &tgbotapi.CallbackQuery{
		ID:      "test_callback_id",
		From:    user,
		Message: &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: 123}, MessageID: 789},
		Data:    callbackData,
	}

	storeUser := &store.User{ID: 1, TelegramUserID: 456}
	mockStore.On("GetUserByTelegramID", mock.Anything, user.ID).Return(storeUser, nil)
	mockScheduler.On("VolunteerForDuty", mock.Anything, storeUser, days).Return(errors.New("scheduler error"))

	editMsg, err := h.HandleVolunteerDaysCallback(callbackQuery)

	assert.NoError(t, err)
	assert.Contains(t, editMsg.Text, "Sorry, we couldn't process your volunteer request")
	mockScheduler.AssertExpectations(t)
}

func TestHandleVolunteerCallback_UserNotFound(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0)

	days := 5
	callbackData := fmt.Sprintf("volunteer_days:%d", days)
	user := &tgbotapi.User{ID: 456, FirstName: "Test"}
	callbackQuery := &tgbotapi.CallbackQuery{
		ID:      "test_callback_id",
		From:    user,
		Message: &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: 123}, MessageID: 789},
		Data:    callbackData,
	}

	mockStore.On("GetUserByTelegramID", mock.Anything, user.ID).Return(nil, nil)

	editMsg, err := h.HandleVolunteerDaysCallback(callbackQuery)

	assert.NoError(t, err)
	assert.Contains(t, editMsg.Text, "Could not find your user profile")
	mockStore.AssertExpectations(t)
}
