package handlers_test

import (
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/mocks"
	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/korjavin/dutyassistant/internal/telegram/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestHandleCancelInteractive(t *testing.T) {
	mockStore := new(mocks.MockStore)
	sm := handlers.NewSessionManager()
	h := &handlers.Handlers{
		Store:          mockStore,
		SessionManager: sm,
	}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(123)).Return(&store.User{IsAdmin: true}, nil)

	mockStore.On("GetUserByTelegramID", mock.Anything, int64(123)).Return(&store.User{IsAdmin: true}, nil)

	now := time.Now()
	mockStore.On("ListActiveChores", mock.Anything).Return([]*store.Chore{{ID: 1, Description: "Task 1"}}, nil)
	mockStore.On("GetActiveRecurringChores", mock.Anything).Return([]*store.RecurringChore{{ID: 2, Description: "Recurring 1"}}, nil)
	mockStore.On("GetDutiesByMonth", mock.Anything, mock.Anything, mock.Anything).Return([]*store.Duty{
		{UserID: 1, DutyDate: now.AddDate(0, 0, 1), User: &store.User{FirstName: "Alice"}},
		{UserID: 2, DutyDate: now.AddDate(0, 0, 2), User: nil}, // Orphan duty row
	}, nil)

	msg := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 456},
		From: &tgbotapi.User{ID: 123},
		Text: "/cancel",
	}

	resp, err := h.HandleCancel(msg)
	assert.NoError(t, err)

	assert.Contains(t, resp.Text, "Select an item to cancel:")
	assert.NotNil(t, resp.ReplyMarkup)
}

func TestHandleCancelAssignmentCallback(t *testing.T) {
	mockStore := new(mocks.MockStore)
	sm := handlers.NewSessionManager()
	h := &handlers.Handlers{
		Store:          mockStore,
		SessionManager: sm,
	}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(123)).Return(&store.User{IsAdmin: true}, nil)

	q := &tgbotapi.CallbackQuery{
		ID:   "cb1",
		From: &tgbotapi.User{ID: 123},
		Message: &tgbotapi.Message{
			Chat:      &tgbotapi.Chat{ID: 456},
			MessageID: 789,
		},
		Data: "cancel_assignment:D1:2023-10-25",
	}

	resp, err := h.HandleCancelAssignmentCallback(q)
	assert.NoError(t, err)

	editMsg, ok := resp.(tgbotapi.EditMessageTextConfig)
	assert.True(t, ok)
	assert.Contains(t, editMsg.Text, "Are you sure you want to cancel the duty on 2023-10-25?")
	assert.NotNil(t, editMsg.ReplyMarkup)
}

func TestHandleCancelAssignmentConfirmCallback(t *testing.T) {
	mockStore := new(mocks.MockStore)
	sm := handlers.NewSessionManager()
	h := &handlers.Handlers{
		Store:          mockStore,
		SessionManager: sm,
	}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(123)).Return(&store.User{IsAdmin: true}, nil)

	mockStore.On("DeleteDuty", mock.Anything, mock.Anything).Return(nil)

	q := &tgbotapi.CallbackQuery{
		ID:   "cb1",
		From: &tgbotapi.User{ID: 123},
		Message: &tgbotapi.Message{
			Chat:      &tgbotapi.Chat{ID: 456},
			MessageID: 789,
		},
		Data: "cancel_assignment_confirm:D1:2023-10-25",
	}

	resp, err := h.HandleCancelAssignmentConfirmCallback(q)
	assert.NoError(t, err)

	editMsg, ok := resp.(tgbotapi.EditMessageTextConfig)
	assert.True(t, ok)
	assert.Contains(t, editMsg.Text, "cancelled successfully.")
}
