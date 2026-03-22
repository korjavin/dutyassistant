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

func TestHandleChoreInteractiveNoArgs(t *testing.T) {
	mockStore := new(mocks.MockStore)
	sm := handlers.NewSessionManager()
	h := &handlers.Handlers{
		Store:          mockStore,
		SessionManager: sm,
	}

	mockStore.On("GetUserByTelegramID", mock.Anything, int64(123)).Return(&store.User{IsAdmin: true}, nil)

	msg := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 456},
		From: &tgbotapi.User{ID: 123},
		Text: "/chore",
	}

	resp, err := h.HandleChore(msg)
	assert.NoError(t, err)

	msgConfig := resp.(tgbotapi.MessageConfig)
	assert.Contains(t, msgConfig.Text, "Chore Management")
	assert.NotNil(t, msgConfig.ReplyMarkup)
}

func TestHandleChoreActionList(t *testing.T) {
	mockStore := new(mocks.MockStore)
	sm := handlers.NewSessionManager()
	h := &handlers.Handlers{
		Store:          mockStore,
		SessionManager: sm,
	}

	mockStore.On("GetUserByTelegramID", mock.Anything, int64(123)).Return(&store.User{IsAdmin: true}, nil)

	chores := []*store.Chore{
		{ID: 1, Description: "Test chore", AssignedAt: time.Now(), User: &store.User{FirstName: "Alice"}},
	}
	rChores := []*store.RecurringChore{
		{ID: 1, Description: "Recurring chore", Interval: 7, NextRunAt: time.Now()},
	}

	mockStore.On("ListActiveChores", mock.Anything).Return(chores, nil)
	mockStore.On("GetActiveRecurringChores", mock.Anything).Return(rChores, nil)

	q := &tgbotapi.CallbackQuery{
		ID:   "cb1",
		From: &tgbotapi.User{ID: 123},
		Message: &tgbotapi.Message{
			Chat:      &tgbotapi.Chat{ID: 456},
			MessageID: 789,
		},
		Data: "chore_action:list",
	}

	resp, err := h.HandleChoreActionCallback(q)
	assert.NoError(t, err)

	editMsg, ok := resp.(tgbotapi.EditMessageTextConfig)
	assert.True(t, ok)
	assert.Contains(t, editMsg.Text, "Test chore")
	assert.Contains(t, editMsg.Text, "Recurring chore")
	assert.Nil(t, editMsg.ReplyMarkup)
}

func TestHandleChoreActionDelete(t *testing.T) {
	mockStore := new(mocks.MockStore)
	sm := handlers.NewSessionManager()
	h := &handlers.Handlers{
		Store:          mockStore,
		SessionManager: sm,
	}

	mockStore.On("GetUserByTelegramID", mock.Anything, int64(123)).Return(&store.User{IsAdmin: true}, nil)
	mockStore.On("ListActiveChores", mock.Anything).Return([]*store.Chore{{ID: 5, Description: "Short"}}, nil)
	mockStore.On("GetActiveRecurringChores", mock.Anything).Return([]*store.RecurringChore{}, nil)

	q := &tgbotapi.CallbackQuery{
		ID:   "cb1",
		From: &tgbotapi.User{ID: 123},
		Message: &tgbotapi.Message{
			Chat:      &tgbotapi.Chat{ID: 456},
			MessageID: 789,
		},
		Data: "chore_action:delete",
	}

	resp, err := h.HandleChoreActionCallback(q)
	assert.NoError(t, err)

	editMsg, ok := resp.(tgbotapi.EditMessageTextConfig)
	assert.True(t, ok)
	assert.Contains(t, editMsg.Text, "Select a chore to delete:")
	assert.NotNil(t, editMsg.ReplyMarkup)
}

func TestHandleChoreDeleteConfirmCallback(t *testing.T) {
	mockStore := new(mocks.MockStore)
	sm := handlers.NewSessionManager()
	h := &handlers.Handlers{
		Store:          mockStore,
		SessionManager: sm,
	}

	mockStore.On("GetUserByTelegramID", mock.Anything, int64(123)).Return(&store.User{IsAdmin: true}, nil)
	mockStore.On("CancelChore", mock.Anything, int64(5)).Return(&store.Chore{}, nil)

	q := &tgbotapi.CallbackQuery{
		ID:   "cb1",
		From: &tgbotapi.User{ID: 123},
		Message: &tgbotapi.Message{
			Chat:      &tgbotapi.Chat{ID: 456},
			MessageID: 789,
		},
		Data: "chore_delete_confirm:A5",
	}

	resp, err := h.HandleChoreDeleteConfirmCallback(q)
	assert.NoError(t, err)

	editMsg, ok := resp.(tgbotapi.EditMessageTextConfig)
	assert.True(t, ok)
	assert.Contains(t, editMsg.Text, "✅ Chore deleted successfully.")
}
