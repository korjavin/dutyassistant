package handlers_test

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/mocks"
	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/korjavin/dutyassistant/internal/telegram/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestHandleChore_NotAdmin(t *testing.T) {
	mockStore := new(mocks.MockStore)
	// Create handlers with groupID=0
	h := handlers.New(mockStore, nil, 0)

	// User is not admin
	nonAdminUser := &store.User{ID: 2, TelegramUserID: 456, IsAdmin: false}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(456)).Return(nonAdminUser, nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 789},
		From: &tgbotapi.User{ID: 456},
		Text: "/chore Clean setup",
	}

	msg, err := h.HandleChore(message)
	assert.NoError(t, err)
	assert.Equal(t, "Sorry, this command is for admins only.", msg.Text)
}

func TestHandleChore_NoArgs(t *testing.T) {
	_, _, h := setupAdminTest(t)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 789},
		From: &tgbotapi.User{ID: 123},
		Text: "/chore", // No description
	}

	msg, err := h.HandleChore(message)
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "To assign a chore, please provide a description")
}

func TestHandleChore_Success(t *testing.T) {
	mockStore := new(mocks.MockStore)
	mockScheduler := new(mocks.MockScheduler)
	// Create with admin ID 123 and group ID 0
	h := handlers.NewWithAdminID(mockStore, mockScheduler, 0, 123)

	// Admin user setup not needed if we use NewWithAdminID and match ID

	// Active users for selection
	activeUsers := []*store.User{
		{ID: 10, FirstName: "Alice", IsActive: true},
		{ID: 11, FirstName: "Bob", IsActive: true},
	}
	mockStore.On("ListActiveUsers", mock.Anything).Return(activeUsers, nil)

	// Mock off-duty check - everyone on duty
	mockStore.On("IsUserOffDuty", mock.Anything, mock.Anything, mock.Anything).Return(false, nil)

	message := &tgbotapi.Message{
		Chat:     &tgbotapi.Chat{ID: 789},
		From:     &tgbotapi.User{ID: 123},
		Text:     "/chore Clean kitchen",
		Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 6}},
	}

	msg, err := h.HandleChore(message)
	assert.NoError(t, err)

	// Message should state success and mention one of the users
	assert.Contains(t, msg.Text, "Assigned chore to")
	// Should mention Alice or Bob
	// isAlice := assert.Contains(t, msg.Text, "Alice")
	// isBob := assert.Contains(t, msg.Text, "Bob")
	// We check if either is present
	assert.True(t, strings.Contains(msg.Text, "Alice") || strings.Contains(msg.Text, "Bob"))
}

func TestHandleChore_NoActiveUsers(t *testing.T) {
	mockStore, _, h := setupAdminTest(t)

	mockStore.On("ListActiveUsers", mock.Anything).Return([]*store.User{}, nil)

	message := &tgbotapi.Message{
		Chat:     &tgbotapi.Chat{ID: 789},
		From:     &tgbotapi.User{ID: 123},
		Text:     "/chore Clean kitchen",
		Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 6}},
	}

	msg, err := h.HandleChore(message)
	assert.NoError(t, err)
	assert.Equal(t, "No active users found to assign the chore to.", msg.Text)
}

func TestHandleChore_HTMLInjection(t *testing.T) {
	mockStore := new(mocks.MockStore)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.NewWithAdminID(mockStore, mockScheduler, 0, 123)

	activeUsers := []*store.User{
		{ID: 10, FirstName: "<b>EvilUser</b>", IsActive: true},
	}
	mockStore.On("ListActiveUsers", mock.Anything).Return(activeUsers, nil)
	mockStore.On("IsUserOffDuty", mock.Anything, mock.Anything, mock.Anything).Return(false, nil)

	message := &tgbotapi.Message{
		Chat:     &tgbotapi.Chat{ID: 789},
		From:     &tgbotapi.User{ID: 123},
		Text:     "/chore <i>malicious description</i>",
		Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 6}},
	}

	msg, err := h.HandleChore(message)
	assert.NoError(t, err)

	// Check that HTML tags are escaped in the output
	assert.Contains(t, msg.Text, "&lt;b&gt;EvilUser&lt;/b&gt;")
	assert.Contains(t, msg.Text, "&lt;i&gt;malicious description&lt;/i&gt;")
}

type RoundTripFunc func(req *http.Request) *http.Response

func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

func NewTestClient(fn RoundTripFunc) *http.Client {
	return &http.Client{
		Transport: fn,
	}
}

func TestHandleChore_GroupAnnouncement(t *testing.T) {
	mockStore := new(mocks.MockStore)
	mockScheduler := new(mocks.MockScheduler)
	groupID := int64(-1001234567890)
	h := handlers.NewWithAdminID(mockStore, mockScheduler, groupID, 123)

	activeUsers := []*store.User{
		{ID: 10, FirstName: "Alice", IsActive: true},
	}
	mockStore.On("ListActiveUsers", mock.Anything).Return(activeUsers, nil)
	mockStore.On("IsUserOffDuty", mock.Anything, mock.Anything, mock.Anything).Return(false, nil)

	// Mock Bot API client
	client := NewTestClient(func(req *http.Request) *http.Response {
		// Verify expected URL and payload if needed
		// For now simple check that it returns success for sendMessage
		if req.URL.String() == "https://api.telegram.org/botTOKEN/sendMessage" {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(`{"ok":true, "result": {"message_id": 1, "chat": {"id": -1001234567890, "type": "supergroup"}}}`)),
				Header:     make(http.Header),
			}
		}
		if req.URL.String() == "https://api.telegram.org/botTOKEN/getMe" {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(`{"ok":true, "result": {"id": 123456, "is_bot": true, "first_name": "TestBot", "username": "test_bot"}}`)),
				Header:     make(http.Header),
			}
		}
		return &http.Response{
			StatusCode: 404,
			Body:       io.NopCloser(bytes.NewBufferString(`{}`)),
			Header:     make(http.Header),
		}
	})

	bot, err := tgbotapi.NewBotAPIWithClient("TOKEN", tgbotapi.APIEndpoint, client)
	assert.NoError(t, err)
	bot.Debug = true
	h.SetBot(bot)

	message := &tgbotapi.Message{
		Chat:     &tgbotapi.Chat{ID: 789}, // DM
		From:     &tgbotapi.User{ID: 123},
		Text:     "/chore Clean kitchen",
		Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 6}},
	}

	msg, err := h.HandleChore(message)
	assert.NoError(t, err)

	// Check response to user
	assert.Contains(t, msg.Text, "Assigned chore to")
	assert.Contains(t, msg.Text, "Announced in group")
}

func TestHandleChore_WeightedSelection(t *testing.T) {
	// This test verifies that the weighted selection works correctly
	// by running multiple iterations and checking distribution
	mockStore := new(mocks.MockStore)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.NewWithAdminID(mockStore, mockScheduler, 0, 123)

	// Create users with different AdminQueueDays
	// Alice: 0 days (weight = 1.0)
	// Bob: 50 days (weight = 1.0 + 50*0.02 = 2.0)
	// Charlie: 100 days (weight = 1.0 + 100*0.02 = 3.0)
	// Total weight = 6.0
	// Expected probabilities: Alice ~16.7%, Bob ~33.3%, Charlie ~50%
	activeUsers := []*store.User{
		{ID: 10, FirstName: "Alice", IsActive: true, AdminQueueDays: 0},
		{ID: 11, FirstName: "Bob", IsActive: true, AdminQueueDays: 50},
		{ID: 12, FirstName: "Charlie", IsActive: true, AdminQueueDays: 100},
	}

	mockStore.On("ListActiveUsers", mock.Anything).Return(activeUsers, nil)
	mockStore.On("IsUserOffDuty", mock.Anything, mock.Anything, mock.Anything).Return(false, nil)

	// Run 1000 iterations to check distribution
	counts := make(map[string]int)
	iterations := 1000

	for i := 0; i < iterations; i++ {
		message := &tgbotapi.Message{
			Chat:     &tgbotapi.Chat{ID: 789},
			From:     &tgbotapi.User{ID: 123},
			Text:     "/chore Clean kitchen",
			Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 6}},
		}

		msg, err := h.HandleChore(message)
		assert.NoError(t, err)

		// Count which user was selected
		if strings.Contains(msg.Text, "Alice") {
			counts["Alice"]++
		} else if strings.Contains(msg.Text, "Bob") {
			counts["Bob"]++
		} else if strings.Contains(msg.Text, "Charlie") {
			counts["Charlie"]++
		}
	}

	t.Logf("Distribution over %d iterations:", iterations)
	t.Logf("Alice (0 days, weight 1.0): %d (%.1f%%, expected ~16.7%%)", counts["Alice"], float64(counts["Alice"])/float64(iterations)*100)
	t.Logf("Bob (50 days, weight 2.0): %d (%.1f%%, expected ~33.3%%)", counts["Bob"], float64(counts["Bob"])/float64(iterations)*100)
	t.Logf("Charlie (100 days, weight 3.0): %d (%.1f%%, expected ~50%%)", counts["Charlie"], float64(counts["Charlie"])/float64(iterations)*100)

	// Verify that:
	// 1. All users were occasionally selected (non-zero counts)
	assert.Greater(t, counts["Alice"], 0, "Alice should be selected at least once")
	assert.Greater(t, counts["Bob"], 0, "Bob should be selected at least once")
	assert.Greater(t, counts["Charlie"], 0, "Charlie should be selected at least once")

	// 2. Charlie (highest weight) should be selected most often
	assert.Greater(t, counts["Charlie"], counts["Bob"], "Charlie should be selected more than Bob")
	assert.Greater(t, counts["Bob"], counts["Alice"], "Bob should be selected more than Alice")

	// 3. Rough distribution check (allowing for statistical variance)
	// Charlie should get roughly 50% (+/- 10% with 1000 iterations is reasonable)
	charliePercent := float64(counts["Charlie"]) / float64(iterations) * 100
	assert.Greater(t, charliePercent, 40.0, "Charlie should get at least 40%")
	assert.Less(t, charliePercent, 60.0, "Charlie should get at most 60%")

	// Bob should get roughly 33% (+/- 10%)
	bobPercent := float64(counts["Bob"]) / float64(iterations) * 100
	assert.Greater(t, bobPercent, 23.0, "Bob should get at least 23%")
	assert.Less(t, bobPercent, 43.0, "Bob should get at most 43%")

	// Ensure total is correct
	total := counts["Alice"] + counts["Bob"] + counts["Charlie"]
	assert.Equal(t, iterations, total, "Total selections should equal iterations")
}
