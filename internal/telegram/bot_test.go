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

// TestBot_HandleUpdate_SessionCancel verifies that the bot intercepts /cancel
// commands when a user is in an active interactive session and correctly
// routes them to the interactive session handler instead of the global command handler.
func TestBot_HandleUpdate_SessionCancel(t *testing.T) {
	mockStore := new(mocks.MockStore)
	mockScheduler := new(mocks.MockScheduler)

	// We need a real API bot for handleUpdate to be able to "send" the response,
	// but we can mock or just ignore the error if it fails to send.
	// We just want to test if it routes to the right place.
	// Or we can just call handleUpdate and observe the session state changing!

	// Create handlers
	h := handlers.NewWithAdminID(mockStore, mockScheduler, 0, 123)

	// Create bot
	// We pass a dummy token, it won't actually connect in this test unless it calls b.api.Send
	// Let's create an API instance that will fail on send but it's enough to run the router
	dummyAPI, _ := tgbotapi.NewBotAPI("dummy:token")
	bot := &Bot{
		api:      dummyAPI,
		handlers: h,
	}

	// 1. Setup session
	chatID := int64(100)
	userID := int64(123)
	h.SessionManager.StartSession(chatID, userID, handlers.SessionTypeEditChore)

	// Ensure session exists
	session, exists := h.SessionManager.GetSession(chatID)
	assert.True(t, exists)
	assert.Equal(t, handlers.SessionTypeEditChore, session.Type)

	// 2. Create the /cancel message update
	update := tgbotapi.Update{
		Message: &tgbotapi.Message{
			Chat: &tgbotapi.Chat{ID: chatID},
			From: &tgbotapi.User{ID: userID},
			Text: "/cancel",
			Entities: []tgbotapi.MessageEntity{
				{Type: "bot_command", Offset: 0, Length: 7},
			},
		},
	}

	// 3. Process the update
	// handleUpdate is a void method that usually calls b.api.Send internally.
	// Since we use a dummy API, it will try to send and fail, but the internal routing will occur first.
	// The key assertion is that the session was correctly cancelled by HandleEditChoreInteractive.
	// Since handleUpdate calls b.api.Send and our dummy API is nil or uninitialized fully,
	// we expect a panic or we can just mock it. Wait, dummyAPI returned by NewBotAPI isn't fully ready.
	// Let's just catch the panic to let the test finish, since we only care about the session state.
	defer func() {
		recover()
		// 4. Verify session is gone
		_, exists = h.SessionManager.GetSession(chatID)
		assert.False(t, exists, "Session should have been deleted by the /cancel interception")
	}()

	bot.handleUpdate(update)
}

// TestBot_HandleUpdate_GlobalCancelDuringSession verifies that when an interactive session is active,
// sending a global /cancel command with arguments (e.g. /cancel chore 1) bypasses the session handler
// and is correctly routed to the global HandleCancel function.
func TestBot_HandleUpdate_GlobalCancelDuringSession(t *testing.T) {
	mockStore := new(mocks.MockStore)
	mockScheduler := new(mocks.MockScheduler)

	// Create handlers
	h := handlers.NewWithAdminID(mockStore, mockScheduler, 0, 123)

	// Mock getting user to satisfy admin check in HandleCancel
	adminUser := &store.User{ID: 1, TelegramUserID: 123, IsAdmin: true}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(123)).Return(adminUser, nil)

	// Mock the behavior of /cancel chore 1
	chore := &store.RecurringChore{ID: 1, Description: "Test Chore", IsActive: true}
	mockStore.On("GetRecurringChore", mock.Anything, int64(1)).Return(chore, nil)
	mockStore.On("CancelRecurringChore", mock.Anything, int64(1)).Return(nil)

	// Create bot
	dummyAPI, _ := tgbotapi.NewBotAPI("dummy:token")
	bot := &Bot{
		api:      dummyAPI,
		handlers: h,
	}

	// 1. Setup session
	chatID := int64(100)
	userID := int64(123)
	h.SessionManager.StartSession(chatID, userID, handlers.SessionTypeEditChore)

	// Ensure session exists
	session, exists := h.SessionManager.GetSession(chatID)
	assert.True(t, exists)
	assert.Equal(t, handlers.SessionTypeEditChore, session.Type)

	// 2. Create the /cancel chore 1 message update
	update := tgbotapi.Update{
		Message: &tgbotapi.Message{
			Chat: &tgbotapi.Chat{ID: chatID},
			From: &tgbotapi.User{ID: userID},
			Text: "/cancel chore 1",
			Entities: []tgbotapi.MessageEntity{
				{Type: "bot_command", Offset: 0, Length: 7}, // Length of /cancel
			},
		},
	}

	// 3. Process the update. Catch the panic that dummyAPI.Send causes.
	defer func() {
		recover()
		// 4. Verify session STILL EXISTS (it wasn't intercepted)
		_, exists = h.SessionManager.GetSession(chatID)
		assert.True(t, exists, "Session should not have been intercepted by a global cancel command")

		// Verify the global cancel handler was indeed called.
		// Use mockStore.AssertCalled instead of AssertExpectations because we want to ignore exactly how many times GetUserByTelegramID is called (it might be called multiple times due to checkAdmin logic).
		mockStore.AssertCalled(t, "GetRecurringChore", mock.Anything, int64(1))
		mockStore.AssertCalled(t, "CancelRecurringChore", mock.Anything, int64(1))
	}()

	bot.handleUpdate(update)
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
