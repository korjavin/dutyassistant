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

func TestHandleVolunteer(t *testing.T) {
	h := handlers.New(nil, nil)
	message := &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: 123}}

	msg, err := h.HandleVolunteer(message)

	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "Volunteer for duty!")
	assert.NotNil(t, msg.ReplyMarkup)
}

func TestHandleVolunteer_WithArgs_Success(t *testing.T) {
	mockStore := new(mocks.MockStore)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.New(mockStore, mockScheduler)

	message := &tgbotapi.Message{
		Chat:     &tgbotapi.Chat{ID: 123},
		From:     &tgbotapi.User{ID: 456},
		Text:     "/volunteer 3",
		Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 10}},
	}

	user := &store.User{ID: 1, TelegramUserID: 456}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(456)).Return(user, nil)
	mockScheduler.On("VolunteerForDuty", mock.Anything, user, 3).Return(nil)

	msg, err := h.HandleVolunteer(message)

	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "Added 3 day(s) to your volunteer queue")
	mockStore.AssertExpectations(t)
	mockScheduler.AssertExpectations(t)
}

func TestHandleVolunteerDaysCallback_Success(t *testing.T) {
	mockStore := new(mocks.MockStore)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.New(mockStore, mockScheduler)

	user := &tgbotapi.User{ID: 456, FirstName: "Test"}
	callbackQuery := &tgbotapi.CallbackQuery{
		ID:      "test_callback_id",
		From:    user,
		Message: &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: 123}, MessageID: 789},
		Data:    "volunteer_days:5",
	}

	storeUser := &store.User{ID: 1, TelegramUserID: 456}
	mockStore.On("GetUserByTelegramID", mock.Anything, user.ID).Return(storeUser, nil)
	mockScheduler.On("VolunteerForDuty", mock.Anything, storeUser, 5).Return(nil)

	editMsg, err := h.HandleVolunteerDaysCallback(callbackQuery)

	assert.NoError(t, err)
	assert.Contains(t, editMsg.Text, "Added 5 day(s) to your volunteer queue")
	mockStore.AssertExpectations(t)
	mockScheduler.AssertExpectations(t)
}

func TestHandleVolunteerDaysCallback_Failure(t *testing.T) {
	mockStore := new(mocks.MockStore)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.New(mockStore, mockScheduler)

	user := &tgbotapi.User{ID: 456, FirstName: "Test"}
	callbackQuery := &tgbotapi.CallbackQuery{
		ID:      "test_callback_id",
		From:    user,
		Message: &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: 123}, MessageID: 789},
		Data:    "volunteer_days:5",
	}

	storeUser := &store.User{ID: 1, TelegramUserID: 456}
	mockStore.On("GetUserByTelegramID", mock.Anything, user.ID).Return(storeUser, nil)
	mockScheduler.On("VolunteerForDuty", mock.Anything, storeUser, 5).Return(errors.New("scheduler error"))

	editMsg, err := h.HandleVolunteerDaysCallback(callbackQuery)

	assert.NoError(t, err)
	assert.Contains(t, editMsg.Text, "Sorry, we couldn't process your volunteer request")
	mockScheduler.AssertExpectations(t)
}

func TestHandleVolunteerDaysCallback_UserNotFound(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil)

	user := &tgbotapi.User{ID: 456, FirstName: "Test"}
	callbackQuery := &tgbotapi.CallbackQuery{
		ID:      "test_callback_id",
		From:    user,
		Message: &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: 123}, MessageID: 789},
		Data:    "volunteer_days:5",
	}

	mockStore.On("GetUserByTelegramID", mock.Anything, user.ID).Return(nil, nil)

	editMsg, err := h.HandleVolunteerDaysCallback(callbackQuery)

	assert.NoError(t, err)
	assert.Contains(t, editMsg.Text, "Could not find your user profile")
	mockStore.AssertExpectations(t)
}
