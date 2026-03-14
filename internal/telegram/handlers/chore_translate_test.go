package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/llm"
	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/korjavin/dutyassistant/internal/store/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestHandleChoreTranslate_Success(t *testing.T) {
	ctx := context.Background()

	// Create mock LLM server that returns a translation
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		resp := chatResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{{Message: struct {
				Content string `json:"content"`
			}{Content: "Clean kitchen"}}},
		}
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(mockHandler)
	defer server.Close()

	llmClient := llm.NewClient("test-key", server.URL, 10, "", nil)
	mockStore := new(mocks.MockStore)

	adminID := int64(123)
	h := NewWithAdminID(mockStore, nil, 0, adminID, llmClient)

	// When AdminID is configured and matches, checkAdmin doesn't call the database

	// Mock GetRecurringChore to return a chore with non-Latin description
	chore := &store.RecurringChore{ID: 42, Description: "Убрать кухню"}
	mockStore.On("GetRecurringChore", ctx, int64(42)).Return(chore, nil)

	// Mock UpdateRecurringChoreDescription
	mockStore.On("UpdateRecurringChoreDescription", ctx, int64(42), "Clean kitchen").Return(nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 789},
		From: &tgbotapi.User{ID: adminID},
		Text: "/chore translate 42",
		Entities: []tgbotapi.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: 6},
		},
	}

	response, err := h.HandleChoreTranslate(message)
	assert.NoError(t, err)
	assert.Contains(t, response.Text, "✅ Chore")
	assert.Contains(t, response.Text, "42")
	assert.Contains(t, response.Text, "translated!")
	assert.Contains(t, response.Text, "Old:")
	assert.Contains(t, response.Text, "New:")
	assert.Contains(t, response.Text, "Clean kitchen")
	assert.Equal(t, tgbotapi.ModeHTML, response.ParseMode)

	mockStore.AssertExpectations(t)
}

func TestHandleChoreTranslate_AlreadyLatin(t *testing.T) {
	ctx := context.Background()

	mockStore := new(mocks.MockStore)
	adminID := int64(123)
	h := NewWithAdminID(mockStore, nil, 0, adminID, nil)

	// When AdminID is configured and matches, checkAdmin doesn't call the database

	// Mock GetRecurringChore to return a chore with Latin (English) description
	chore := &store.RecurringChore{ID: 42, Description: "Clean the kitchen"}
	mockStore.On("GetRecurringChore", ctx, int64(42)).Return(chore, nil)

	// UpdateRecurringChoreDescription should NOT be called since description is already Latin

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 789},
		From: &tgbotapi.User{ID: adminID},
		Text: "/chore translate 42",
		Entities: []tgbotapi.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: 6},
		},
	}

	response, err := h.HandleChoreTranslate(message)
	assert.NoError(t, err)
	assert.Contains(t, response.Text, "ℹ️ Chore")
	assert.Contains(t, response.Text, "42")
	assert.Contains(t, response.Text, "already in English")
	assert.Contains(t, response.Text, "Clean the kitchen")

	mockStore.AssertExpectations(t)
	mockStore.AssertNotCalled(t, "UpdateRecurringChoreDescription")
}

func TestHandleChoreTranslate_InvalidChoreID(t *testing.T) {
	ctx := context.Background()

	mockStore := new(mocks.MockStore)
	adminID := int64(123)
	h := NewWithAdminID(mockStore, nil, 0, adminID, nil)

	// When AdminID is configured and matches, checkAdmin doesn't call the database

	// Mock GetRecurringChore to return nil (chore not found)
	mockStore.On("GetRecurringChore", ctx, int64(999)).Return(nil, nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 789},
		From: &tgbotapi.User{ID: adminID},
		Text: "/chore translate 999",
		Entities: []tgbotapi.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: 6},
		},
	}

	response, err := h.HandleChoreTranslate(message)
	assert.NoError(t, err)
	assert.Contains(t, response.Text, "❌ Chore not found")

	mockStore.AssertExpectations(t)
}

func TestHandleChoreTranslate_NonAdmin(t *testing.T) {
	mockStore := new(mocks.MockStore)
	adminID := int64(123)
	nonAdminID := int64(456)
	h := NewWithAdminID(mockStore, nil, 0, adminID, nil)

	// Mock the admin check to return non-admin (will call database since IDs don't match)
	nonAdminUser := &store.User{ID: 2, TelegramUserID: nonAdminID, FirstName: "User", IsAdmin: false}
	mockStore.On("GetUserByTelegramID", mock.Anything, nonAdminID).Return(nonAdminUser, nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 789},
		From: &tgbotapi.User{ID: nonAdminID},
		Text: "/chore translate 42",
		Entities: []tgbotapi.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: 6},
		},
	}

	response, err := h.HandleChoreTranslate(message)
	assert.NoError(t, err)
	assert.Contains(t, response.Text, "❌ Only admins can translate")

	// Verify that GetRecurringChore was NOT called (access denied before that)
	mockStore.AssertNotCalled(t, "GetRecurringChore")
}

func TestHandleChoreTranslate_InvalidIDFormat(t *testing.T) {
	mockStore := new(mocks.MockStore)
	adminID := int64(123)
	h := NewWithAdminID(mockStore, nil, 0, adminID, nil)

	// When AdminID is configured and matches, checkAdmin doesn't call the database

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 789},
		From: &tgbotapi.User{ID: adminID},
		Text: "/chore translate abc",
		Entities: []tgbotapi.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: 6},
		},
	}

	response, err := h.HandleChoreTranslate(message)
	assert.NoError(t, err)
	assert.Contains(t, response.Text, "❌ Invalid chore ID")
}

func TestHandleChoreTranslate_LLMErrorFallback(t *testing.T) {
	ctx := context.Background()

	// Mock server that returns an error
	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer errorServer.Close()

	llmClient := llm.NewClient("test-key", errorServer.URL, 10, "", nil)
	mockStore := new(mocks.MockStore)
	adminID := int64(123)
	h := NewWithAdminID(mockStore, nil, 0, adminID, llmClient)

	// When AdminID is configured and matches, checkAdmin doesn't call the database

	// Mock GetRecurringChore to return a chore with non-Latin description
	chore := &store.RecurringChore{ID: 42, Description: "Убрать кухню"}
	mockStore.On("GetRecurringChore", ctx, int64(42)).Return(chore, nil)

	// When LLM fails, should return original description, so UpdateRecurringChoreDescription
	// should NOT be called (no change needed)
	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 789},
		From: &tgbotapi.User{ID: adminID},
		Text: "/chore translate 42",
		Entities: []tgbotapi.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: 6},
		},
	}

	response, err := h.HandleChoreTranslate(message)
	assert.NoError(t, err)
	assert.Contains(t, response.Text, "ℹ️ Chore")
	assert.Contains(t, response.Text, "already in English")

	mockStore.AssertExpectations(t)
	mockStore.AssertNotCalled(t, "UpdateRecurringChoreDescription")
}

func TestHandleChoreTranslate_NoLLMClient(t *testing.T) {
	ctx := context.Background()

	mockStore := new(mocks.MockStore)
	adminID := int64(123)
	h := NewWithAdminID(mockStore, nil, 0, adminID, nil) // No LLM client

	// When AdminID is configured and matches, checkAdmin doesn't call the database

	// Mock GetRecurringChore to return a chore with non-Latin description
	chore := &store.RecurringChore{ID: 42, Description: "Убрать кухню"}
	mockStore.On("GetRecurringChore", ctx, int64(42)).Return(chore, nil)

	// Without LLM client, should return original description
	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 789},
		From: &tgbotapi.User{ID: adminID},
		Text: "/chore translate 42",
		Entities: []tgbotapi.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: 6},
		},
	}

	response, err := h.HandleChoreTranslate(message)
	assert.NoError(t, err)
	assert.Contains(t, response.Text, "ℹ️ Chore")
	assert.Contains(t, response.Text, "translation is disabled")
	assert.Contains(t, response.Text, "Убрать кухню")

	mockStore.AssertExpectations(t)
	mockStore.AssertNotCalled(t, "UpdateRecurringChoreDescription")
}

func TestHandleChoreTranslate_UpdateFails(t *testing.T) {
	ctx := context.Background()

	// Create mock LLM server that returns a translation
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		resp := chatResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{{Message: struct {
				Content string `json:"content"`
			}{Content: "Clean kitchen"}}},
		}
		json.NewEncoder(w).Encode(resp)
	})

	server := httptest.NewServer(mockHandler)
	defer server.Close()

	llmClient := llm.NewClient("test-key", server.URL, 10, "", nil)
	mockStore := new(mocks.MockStore)
	adminID := int64(123)
	h := NewWithAdminID(mockStore, nil, 0, adminID, llmClient)

	// When AdminID is configured and matches, checkAdmin doesn't call the database

	// Mock GetRecurringChore to return a chore with non-Latin description
	chore := &store.RecurringChore{ID: 42, Description: "Убрать кухню"}
	mockStore.On("GetRecurringChore", ctx, int64(42)).Return(chore, nil)

	// Mock UpdateRecurringChoreDescription to fail
	mockStore.On("UpdateRecurringChoreDescription", ctx, int64(42), "Clean kitchen").Return(assert.AnError)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 789},
		From: &tgbotapi.User{ID: adminID},
		Text: "/chore translate 42",
		Entities: []tgbotapi.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: 6},
		},
	}

	response, err := h.HandleChoreTranslate(message)
	assert.NoError(t, err)
	assert.Contains(t, response.Text, "❌ Failed to update chore description")

	mockStore.AssertExpectations(t)
}
