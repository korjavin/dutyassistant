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
