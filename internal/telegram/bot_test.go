package telegram

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

func TestHandleMessage_DailyRatingsSession(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.NewWithAdminID(mockStore, nil, 0, 123)
	bot := &Bot{handlers: h}

	ratingDate := time.Date(2026, time.March, 13, 20, 50, 0, 0, time.UTC)
	normalizedDate := time.Date(2026, time.March, 13, 0, 0, 0, 0, time.UTC)
	participants := []*store.User{
		{ID: 10, FirstName: "Alice"},
		{ID: 11, FirstName: "Bob"},
	}

	mockStore.On("GetParticipantsForRating", mock.Anything).Return(participants, nil).Once()
	mockStore.On("SaveDailyParticipantRatings", mock.Anything, normalizedDate, mock.MatchedBy(func(ratings []*store.ParticipantDailyRating) bool {
		return len(ratings) == 2 && ratings[0].Score == 4 && ratings[1].Score == 5
	})).Return(nil).Once()

	_, err := h.StartDailyRatingsSession(100, 123, ratingDate)
	assert.NoError(t, err)

	resp, err := bot.handleMessage(&tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 100},
		From: &tgbotapi.User{ID: 123},
		Text: "4 5",
	})
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	msgConfig, ok := resp.(tgbotapi.MessageConfig)
	assert.True(t, ok)
	assert.Contains(t, msgConfig.Text, "Saved ratings for 2026-03-13")

	mockStore.AssertExpectations(t)
}
