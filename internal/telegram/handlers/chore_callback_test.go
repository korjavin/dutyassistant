package handlers_test

import (
	"fmt"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/mocks"
	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/korjavin/dutyassistant/internal/telegram/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestHandleChoreListDoneCallback_Success(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.NewWithAdminID(mockStore, nil, 0, 123, nil)

	// Mock getting the chore
	choreID := int64(1)
	chore := &store.Chore{
		ID:          choreID,
		UserID:      2,
		Description: "Take out the trash",
		ReminderID:  "reminder123",
	}
	mockStore.On("GetChoreByID", mock.Anything, choreID).Return(chore, nil)

	// Mock getting the caller user
	callerUser := &store.User{
		ID:             2,
		TelegramUserID: 456,
		FirstName:      "Alice",
	}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(456)).Return(callerUser, nil)

	// Mock completing the chore in the database
	mockStore.On("CompleteChoreByReminderID", mock.Anything, "reminder123").Return(nil)

	// Mock the ReminderManager if it exists
	// Here we just initialize a mock ReminderManager or handle the case where it's nil
	// For testing the callback, the manager is not strictly required if we don't need to mock its methods
	// However, if we need to mock it, we would need to create a mock for ChoreReminderManager

	q := &tgbotapi.CallbackQuery{
		ID:   "1",
		Data: fmt.Sprintf("chore_list_done:%d", choreID),
		From: &tgbotapi.User{
			ID:        456,
			FirstName: "Alice",
		},
		Message: &tgbotapi.Message{
			Chat: &tgbotapi.Chat{
				ID: 789,
			},
			MessageID: 10,
		},
	}

	edit, err := h.HandleChoreListDoneCallback(q)
	assert.NoError(t, err)

	assert.Contains(t, edit.Text, "Chore marked as done")
	assert.Contains(t, edit.Text, "Take out the trash")
}

func TestHandleChoreListDoneCallback_WrongUser(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.NewWithAdminID(mockStore, nil, 0, 123, nil)

	choreID := int64(1)
	chore := &store.Chore{
		ID:          choreID,
		UserID:      3, // Assigned to user 3
		Description: "Take out the trash",
		ReminderID:  "reminder123",
	}
	mockStore.On("GetChoreByID", mock.Anything, choreID).Return(chore, nil)

	callerUser := &store.User{
		ID:             2, // Caller is user 2
		TelegramUserID: 456,
		FirstName:      "Alice",
	}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(456)).Return(callerUser, nil)

	q := &tgbotapi.CallbackQuery{
		ID:   "1",
		Data: fmt.Sprintf("chore_list_done:%d", choreID),
		From: &tgbotapi.User{
			ID:        456,
			FirstName: "Alice",
		},
		Message: &tgbotapi.Message{
			Chat: &tgbotapi.Chat{
				ID: 789,
			},
			MessageID: 10,
		},
	}

	edit, err := h.HandleChoreListDoneCallback(q)
	assert.NoError(t, err)

	assert.Contains(t, edit.Text, "not assigned to you")
}
