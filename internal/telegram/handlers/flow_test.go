package handlers_test

import (
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/korjavin/dutyassistant/internal/mocks"
	"github.com/korjavin/dutyassistant/internal/telegram/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestHandleCancelFlow(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := &handlers.Handlers{
		Store: mockStore,
	}

	mockStore.On("GetUserByTelegramID", mock.Anything, int64(111)).Return(&store.User{IsAdmin: true}, nil)

	// Create a mock callback query
	q := &tgbotapi.CallbackQuery{
		ID: "123",
		From: &tgbotapi.User{ID: 111},
		Message: &tgbotapi.Message{
			MessageID: 456,
			Chat: &tgbotapi.Chat{
				ID: 789,
			},
		},
		Data: "cancel_flow",
	}

	// Call the handler
	resp, err := h.HandleCancelFlow(q)
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	// Check the response type and content
	editMsg, ok := resp.(tgbotapi.EditMessageTextConfig)
	assert.True(t, ok)
	assert.Equal(t, int64(789), editMsg.ChatID)
	assert.Equal(t, 456, editMsg.MessageID)
	assert.Equal(t, "❌ Operation cancelled", editMsg.Text)
	assert.Nil(t, editMsg.ReplyMarkup)
}
