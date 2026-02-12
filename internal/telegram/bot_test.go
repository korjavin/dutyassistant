package telegram

import (
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/mocks"
	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/korjavin/dutyassistant/internal/telegram/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestHandleCommand_Chore(t *testing.T) {
	mockStore := new(mocks.MockStore)
	mockScheduler := new(mocks.MockScheduler)

	// Create handlers with AdminID=123, GroupID=0
	h := handlers.NewWithAdminID(mockStore, mockScheduler, 0, 123)

	// Create bot with handlers
	bot := &Bot{
		handlers: h,
	}

	// Setup expectations for HandleChore
	// 1. ListActiveUsers (Called by HandleChore)
	// Returning empty list is enough to verify dispatch happened and HandleChore was entered
	mockStore.On("ListActiveUsers", mock.Anything).Return([]*store.User{}, nil)

	// Create message
	msg := &tgbotapi.Message{
		Chat:     &tgbotapi.Chat{ID: 100},
		From:     &tgbotapi.User{ID: 123},
		Text:     "/chore Clean",
		Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 6}},
	}

	// Call private handleCommand method
	resp, err := bot.handleCommand(msg)

	// Assert dispatch success
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	// Check if it's a message config and has expected text from HandleChore
	// Since ListActiveUsers returns empty, HandleChore returns "No active users found..."
	msgConfig, ok := resp.(tgbotapi.MessageConfig)
	assert.True(t, ok)
	assert.Equal(t, "No active users found to assign the chore to.", msgConfig.Text)

	mockStore.AssertExpectations(t)
}
