package handlers_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/llm"
	llmmocks "github.com/korjavin/dutyassistant/internal/mocks"
	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/korjavin/dutyassistant/internal/store/mocks"
	"github.com/korjavin/dutyassistant/internal/telegram/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const previewAdminID = int64(999)

func previewMessage(fromID int64, args string) *tgbotapi.Message {
	text := "/text_candidate_chore"
	if args != "" {
		text += " " + args
	}
	return &tgbotapi.Message{
		Text:     text,
		Chat:     &tgbotapi.Chat{ID: fromID},
		From:     &tgbotapi.User{ID: fromID, FirstName: "Admin"},
		Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: len("/text_candidate_chore")}},
	}
}

func previewChores(now time.Time) []*store.Chore {
	anna := &store.User{ID: 1, TelegramUserID: 111, FirstName: "Anna"}
	bob := &store.User{ID: 2, TelegramUserID: 222, FirstName: "Bob"}
	return []*store.Chore{
		{ID: 1, UserID: 1, Description: "Take out <trash>", AssignedAt: now.Add(-72 * time.Hour), DeadlineAt: now.Add(-48 * time.Hour), User: anna},
		{ID: 2, UserID: 2, Description: "Bob's chore", AssignedAt: now.Add(-time.Hour), User: bob},
	}
}

func fixPreviewTime(t *testing.T) time.Time {
	t.Helper()
	now := time.Date(2026, 3, 14, 17, 0, 0, 0, time.UTC)
	orig := handlers.TimeNow
	handlers.TimeNow = func() time.Time { return now }
	t.Cleanup(func() { handlers.TimeNow = orig })
	return now
}

func TestHandleTextCandidateChore_WithLLM(t *testing.T) {
	now := fixPreviewTime(t)
	st := new(mocks.MockStore)
	llmClient := new(llmmocks.MockLLMClient)
	h := handlers.NewWithAdminID(st, nil, 0, previewAdminID, llmClient)

	st.On("GetUserByName", mock.Anything, "Anna").Return(&store.User{ID: 1, FirstName: "Anna"}, nil)
	st.On("GetActiveChores", mock.Anything).Return(previewChores(now), nil)
	llmClient.On("ChoreNudge", mock.Anything, "Anna", mock.MatchedBy(func(c string) bool {
		return !strings.Contains(c, "Bob's chore") && strings.Contains(c, "OVERDUE by 2 days")
	})).Return("🗑️ Anna, the trash misses you — or the admin hears about it!", nil)

	msg, err := h.HandleTextCandidateChore(previewMessage(previewAdminID, "Anna"))

	assert.NoError(t, err)
	assert.Equal(t, tgbotapi.ModeHTML, msg.ParseMode)
	assert.Equal(t, previewAdminID, msg.ChatID)
	assert.Contains(t, msg.Text, "Preview")
	assert.Contains(t, msg.Text, "not sent to them")
	assert.Contains(t, msg.Text, "the trash misses you")
	assert.Contains(t, msg.Text, "1. <b>Take out &lt;trash&gt;</b>")
	assert.Contains(t, msg.Text, "OVERDUE by 2 days")
	assert.NotContains(t, msg.Text, "Bob")
	assert.NotContains(t, msg.Text, "static fallback")
	llmClient.AssertExpectations(t)
}

func TestHandleTextCandidateChore_LLMFailureShowsFallback(t *testing.T) {
	now := fixPreviewTime(t)
	st := new(mocks.MockStore)
	llmClient := new(llmmocks.MockLLMClient)
	h := handlers.NewWithAdminID(st, nil, 0, previewAdminID, llmClient)

	st.On("GetUserByName", mock.Anything, "Anna").Return(&store.User{ID: 1, FirstName: "Anna"}, nil)
	st.On("GetActiveChores", mock.Anything).Return(previewChores(now), nil)
	llmClient.On("ChoreNudge", mock.Anything, "Anna", mock.Anything).Return("", errors.New("rate limited <429>"))

	msg, err := h.HandleTextCandidateChore(previewMessage(previewAdminID, "Anna"))

	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "Just a gentle reminder")
	assert.Contains(t, msg.Text, "LLM failed")
	assert.Contains(t, msg.Text, "rate limited &lt;429&gt;")
}

func TestHandleTextCandidateChore_NoLLM(t *testing.T) {
	for name, client := range map[string]llm.ClientInterface{
		"nil interface": nil,
		"typed nil":     (*llm.Client)(nil),
	} {
		t.Run(name, func(t *testing.T) {
			now := fixPreviewTime(t)
			st := new(mocks.MockStore)
			h := handlers.NewWithAdminID(st, nil, 0, previewAdminID, client)

			st.On("GetUserByName", mock.Anything, "Anna").Return(&store.User{ID: 1, FirstName: "Anna"}, nil)
			st.On("GetActiveChores", mock.Anything).Return(previewChores(now), nil)

			msg, err := h.HandleTextCandidateChore(previewMessage(previewAdminID, "Anna"))

			assert.NoError(t, err)
			assert.Contains(t, msg.Text, "Just a gentle reminder")
			assert.Contains(t, msg.Text, "LLM is not configured")
		})
	}
}

func TestHandleTextCandidateChore_EdgeCases(t *testing.T) {
	now := fixPreviewTime(t)

	t.Run("non-admin rejected", func(t *testing.T) {
		st := new(mocks.MockStore)
		h := handlers.NewWithAdminID(st, nil, 0, previewAdminID, nil)
		msg, err := h.HandleTextCandidateChore(previewMessage(123, "Anna"))
		assert.NoError(t, err)
		assert.Contains(t, msg.Text, "admin")
		st.AssertNotCalled(t, "GetUserByName", mock.Anything, mock.Anything)
	})

	t.Run("missing name shows usage", func(t *testing.T) {
		h := handlers.NewWithAdminID(new(mocks.MockStore), nil, 0, previewAdminID, nil)
		msg, _ := h.HandleTextCandidateChore(previewMessage(previewAdminID, ""))
		assert.Contains(t, msg.Text, "Usage: /text_candidate_chore <name>")
	})

	t.Run("unknown user", func(t *testing.T) {
		st := new(mocks.MockStore)
		h := handlers.NewWithAdminID(st, nil, 0, previewAdminID, nil)
		st.On("GetUserByName", mock.Anything, "Zed").Return(nil, nil)
		msg, _ := h.HandleTextCandidateChore(previewMessage(previewAdminID, "Zed"))
		assert.Contains(t, msg.Text, "User 'Zed' not found.")
	})

	t.Run("no pending chores", func(t *testing.T) {
		st := new(mocks.MockStore)
		llmClient := new(llmmocks.MockLLMClient)
		h := handlers.NewWithAdminID(st, nil, 0, previewAdminID, llmClient)
		st.On("GetUserByName", mock.Anything, "Carl").Return(&store.User{ID: 3, FirstName: "Carl"}, nil)
		st.On("GetActiveChores", mock.Anything).Return(previewChores(now), nil)
		msg, _ := h.HandleTextCandidateChore(previewMessage(previewAdminID, "Carl"))
		assert.Contains(t, msg.Text, "has no pending chores")
		llmClient.AssertNotCalled(t, "ChoreNudge", mock.Anything, mock.Anything, mock.Anything)
	})
}
