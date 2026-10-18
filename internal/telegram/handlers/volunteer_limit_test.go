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

func volunteerLimitMessage(fromID int64, text string) *tgbotapi.Message {
	return &tgbotapi.Message{
		Chat:     &tgbotapi.Chat{ID: 123},
		From:     &tgbotapi.User{ID: fromID, FirstName: "AdminUser"},
		Text:     text,
		Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 20}},
	}
}

func TestHandleSetVolunteerLimit_Enable(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.NewWithAdminID(mockStore, nil, 0, 999, nil)

	mockStore.On("GetUserByName", mock.Anything, "Ivan").Return(&store.User{ID: 1, FirstName: "Ivan"}, nil)
	mockStore.On("SetExtendedVolunteer", mock.Anything, int64(1), true).Return(nil)

	msg, err := h.HandleSetVolunteerLimit(volunteerLimitMessage(999, "/set_volunteer_limit Ivan on"))
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "Ivan may now volunteer for up to 183 day(s)")
	mockStore.AssertExpectations(t)
}

func TestHandleSetVolunteerLimit_Disable(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.NewWithAdminID(mockStore, nil, 0, 999, nil)

	mockStore.On("GetUserByName", mock.Anything, "Ivan").Return(&store.User{ID: 1, FirstName: "Ivan"}, nil)
	mockStore.On("SetExtendedVolunteer", mock.Anything, int64(1), false).Return(nil)

	msg, err := h.HandleSetVolunteerLimit(volunteerLimitMessage(999, "/set_volunteer_limit Ivan off"))
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "up to 7 day(s)")
	mockStore.AssertExpectations(t)
}

func TestHandleSetVolunteerLimit_ShowsCurrent(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.NewWithAdminID(mockStore, nil, 0, 999, nil)

	mockStore.On("GetUserByName", mock.Anything, "Ivan").Return(&store.User{ID: 1, FirstName: "Ivan"}, nil)
	mockStore.On("IsExtendedVolunteer", mock.Anything, int64(1)).Return(true, nil)

	msg, err := h.HandleSetVolunteerLimit(volunteerLimitMessage(999, "/set_volunteer_limit Ivan"))
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "up to 183 day(s)")
	mockStore.AssertNotCalled(t, "SetExtendedVolunteer", mock.Anything, mock.Anything, mock.Anything)
}

func TestHandleSetVolunteerLimit_NotAdmin(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.NewWithAdminID(mockStore, nil, 0, 999, nil)

	msg, err := h.HandleSetVolunteerLimit(volunteerLimitMessage(111, "/set_volunteer_limit Ivan on"))
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "admin")
	mockStore.AssertNotCalled(t, "SetExtendedVolunteer", mock.Anything, mock.Anything, mock.Anything)
}

func TestHandleSetVolunteerLimit_BadArgument(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.NewWithAdminID(mockStore, nil, 0, 999, nil)

	msg, err := h.HandleSetVolunteerLimit(volunteerLimitMessage(999, "/set_volunteer_limit Ivan maybe"))
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "on or off")
}
