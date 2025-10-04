package handlers_test

import (
	"errors"
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

func TestHandleVolunteer(t *testing.T) {
	h := handlers.New(nil, nil)
	message := &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: 123}}

	msg, err := h.HandleVolunteer(message)

	assert.NoError(t, err)
	assert.Equal(t, "Please select a date to volunteer for duty.", msg.Text)
	assert.NotNil(t, msg.ReplyMarkup)
}

func TestHandleVolunteerCallback_Success(t *testing.T) {
	mockStore := new(mocks.MockStore)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.New(mockStore, mockScheduler)

	dateStr := "2023-05-20"
	dutyDate, _ := time.Parse("2006-01-02", dateStr)
	callbackData := fmt.Sprintf("%s:%s", keyboard.ActionSelectDay, dateStr)
	user := &tgbotapi.User{ID: 456, FirstName: "Test"}
	callbackQuery := &tgbotapi.CallbackQuery{
		ID:      "test_callback_id",
		From:    user,
		Message: &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: 123}, MessageID: 789},
		Data:    callbackData,
	}

	storeUser := &store.User{ID: 1, TelegramUserID: 456}
	mockStore.On("GetUserByTelegramID", mock.Anything, user.ID).Return(storeUser, nil)
	mockScheduler.On("VolunteerForDuty", mock.Anything, storeUser, dutyDate).Return(nil)

	editMsg, err := h.HandleVolunteerCallback(callbackQuery)

	assert.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("Thank you for volunteering for duty on %s!", dateStr), editMsg.Text)
	assert.Nil(t, editMsg.ReplyMarkup, "Keyboard should be removed on success")
	mockStore.AssertExpectations(t)
	mockScheduler.AssertExpectations(t)
}

func TestHandleVolunteerCallback_Failure(t *testing.T) {
	mockStore := new(mocks.MockStore)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.New(mockStore, mockScheduler)

	dateStr := "2023-05-20"
	dutyDate, _ := time.Parse("2006-01-02", dateStr)
	callbackData := fmt.Sprintf("%s:%s", keyboard.ActionSelectDay, dateStr)
	user := &tgbotapi.User{ID: 456, FirstName: "Test"}
	callbackQuery := &tgbotapi.CallbackQuery{
		ID:      "test_callback_id",
		From:    user,
		Message: &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: 123}, MessageID: 789},
		Data:    callbackData,
	}

	storeUser := &store.User{ID: 1, TelegramUserID: 456}
	mockStore.On("GetUserByTelegramID", mock.Anything, user.ID).Return(storeUser, nil)
	mockScheduler.On("VolunteerForDuty", mock.Anything, storeUser, dutyDate).Return(errors.New("scheduler error"))

	editMsg, err := h.HandleVolunteerCallback(callbackQuery)

	assert.NoError(t, err)
	assert.Contains(t, editMsg.Text, "Sorry, we couldn't process your request")
	mockScheduler.AssertExpectations(t)
}

func TestHandleVolunteerCallback_UserNotFound(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil)

	dateStr := "2023-05-20"
	callbackData := fmt.Sprintf("%s:%s", keyboard.ActionSelectDay, dateStr)
	user := &tgbotapi.User{ID: 456, FirstName: "Test"}
	callbackQuery := &tgbotapi.CallbackQuery{
		ID:      "test_callback_id",
		From:    user,
		Message: &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: 123}, MessageID: 789},
		Data:    callbackData,
	}

	mockStore.On("GetUserByTelegramID", mock.Anything, user.ID).Return(nil, nil)

	editMsg, err := h.HandleVolunteerCallback(callbackQuery)

	assert.NoError(t, err)
	assert.Equal(t, "Could not find your user profile. Please use /start first.", editMsg.Text)
	mockStore.AssertExpectations(t)
}