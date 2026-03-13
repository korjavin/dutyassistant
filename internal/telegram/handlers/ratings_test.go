package handlers

import (
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/mocks"
	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestStartDailyRatingsSession_BuildsStablePrompt(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := NewWithAdminID(mockStore, nil, 0, 123)

	participants := []*store.User{
		{ID: 10, FirstName: "Alice"},
		{ID: 11, FirstName: "Bob"},
		{ID: 12, FirstName: "Cara"},
	}
	ratingDate := time.Date(2026, time.March, 13, 20, 50, 0, 0, time.FixedZone("CET", 3600))

	mockStore.On("GetParticipantsForRating", mock.Anything).Return(participants, nil).Once()

	msg, err := h.StartDailyRatingsSession(500, 123, ratingDate)
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "Daily participant ratings for 2026-03-13")
	assert.Contains(t, msg.Text, "1. Alice")
	assert.Contains(t, msg.Text, "2. Bob")
	assert.Contains(t, msg.Text, "3. Cara")
	assert.Contains(t, msg.Text, "Example: 5 5 5")

	session, exists := h.SessionManager.GetSession(500)
	assert.True(t, exists)
	assert.Equal(t, SessionTypeDailyRatings, session.Type)
	assert.Equal(t, int64(123), session.UserID)

	mockStore.AssertExpectations(t)
}

func TestHandleDailyRatingsInteractive_ValidSubmission(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := NewWithAdminID(mockStore, nil, 0, 123)

	ratingDate := time.Date(2026, time.March, 13, 20, 50, 0, 0, time.UTC)
	participants := []*store.User{
		{ID: 10, FirstName: "Alice"},
		{ID: 11, FirstName: "Bob"},
	}

	mockStore.On("GetParticipantsForRating", mock.Anything).Return(participants, nil).Once()
	mockStore.On("SaveDailyParticipantRatings", mock.Anything, normalizeRatingDate(ratingDate), mock.MatchedBy(func(ratings []*store.ParticipantDailyRating) bool {
		return len(ratings) == 2 &&
			ratings[0].ParticipantID == 10 && ratings[0].ParticipantName == "Alice" && ratings[0].Score == 5 &&
			ratings[1].ParticipantID == 11 && ratings[1].ParticipantName == "Bob" && ratings[1].Score == 3
	})).Return(nil).Once()

	_, err := h.StartDailyRatingsSession(700, 123, ratingDate)
	assert.NoError(t, err)

	msg, err := h.HandleDailyRatingsInteractive(&tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 700},
		From: &tgbotapi.User{ID: 123},
		Text: "5 3",
	})
	assert.NoError(t, err)
	assert.NotNil(t, msg)

	cfg, ok := msg.(tgbotapi.MessageConfig)
	assert.True(t, ok)
	assert.Contains(t, cfg.Text, "Saved ratings for 2026-03-13")
	assert.Contains(t, cfg.Text, "overwrite today's ratings")

	_, exists := h.SessionManager.GetSession(700)
	assert.True(t, exists)

	mockStore.AssertExpectations(t)
}

func TestHandleDailyRatingsInteractive_InvalidCount(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := NewWithAdminID(mockStore, nil, 0, 123)

	ratingDate := time.Date(2026, time.March, 13, 0, 0, 0, 0, time.UTC)
	participants := []*store.User{
		{ID: 10, FirstName: "Alice"},
		{ID: 11, FirstName: "Bob"},
	}

	mockStore.On("GetParticipantsForRating", mock.Anything).Return(participants, nil).Once()

	_, err := h.StartDailyRatingsSession(701, 123, ratingDate)
	assert.NoError(t, err)

	msg, err := h.HandleDailyRatingsInteractive(&tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 701},
		From: &tgbotapi.User{ID: 123},
		Text: "5",
	})
	assert.NoError(t, err)

	cfg := msg.(tgbotapi.MessageConfig)
	assert.Contains(t, cfg.Text, "expected 2 score(s), received 1")
	assert.Contains(t, cfg.Text, "Participant order:")
	assert.Contains(t, cfg.Text, "1. Alice")
	assert.Contains(t, cfg.Text, "2. Bob")
}

func TestHandleDailyRatingsInteractive_InvalidRange(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := NewWithAdminID(mockStore, nil, 0, 123)

	ratingDate := time.Date(2026, time.March, 13, 0, 0, 0, 0, time.UTC)
	participants := []*store.User{
		{ID: 10, FirstName: "Alice"},
		{ID: 11, FirstName: "Bob"},
	}

	mockStore.On("GetParticipantsForRating", mock.Anything).Return(participants, nil).Once()

	_, err := h.StartDailyRatingsSession(702, 123, ratingDate)
	assert.NoError(t, err)

	msg, err := h.HandleDailyRatingsInteractive(&tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 702},
		From: &tgbotapi.User{ID: 123},
		Text: "5 0",
	})
	assert.NoError(t, err)

	cfg := msg.(tgbotapi.MessageConfig)
	assert.Contains(t, cfg.Text, "scores must be between 1 and 5")
	assert.Contains(t, cfg.Text, "1. Alice")
	assert.Contains(t, cfg.Text, "2. Bob")
}

func TestHandleDailyRatingsInteractive_UnauthorizedSenderIgnored(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := NewWithAdminID(mockStore, nil, 0, 123)

	ratingDate := time.Date(2026, time.March, 13, 0, 0, 0, 0, time.UTC)
	participants := []*store.User{
		{ID: 10, FirstName: "Alice"},
		{ID: 11, FirstName: "Bob"},
	}

	mockStore.On("GetParticipantsForRating", mock.Anything).Return(participants, nil).Once()

	_, err := h.StartDailyRatingsSession(703, 123, ratingDate)
	assert.NoError(t, err)

	msg, err := h.HandleDailyRatingsInteractive(&tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 703},
		From: &tgbotapi.User{ID: 456},
		Text: "5 5",
	})
	assert.NoError(t, err)
	assert.Nil(t, msg)

	session, exists := h.SessionManager.GetSession(703)
	assert.True(t, exists)
	assert.Equal(t, int64(123), session.UserID)
}

func TestHandleDailyRatingsInteractive_OverwriteCorrection(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := NewWithAdminID(mockStore, nil, 0, 123)

	ratingDate := time.Date(2026, time.March, 13, 0, 0, 0, 0, time.UTC)
	participants := []*store.User{
		{ID: 10, FirstName: "Alice"},
		{ID: 11, FirstName: "Bob"},
	}

	mockStore.On("GetParticipantsForRating", mock.Anything).Return(participants, nil).Once()
	mockStore.On("SaveDailyParticipantRatings", mock.Anything, normalizeRatingDate(ratingDate), mock.MatchedBy(func(ratings []*store.ParticipantDailyRating) bool {
		return len(ratings) == 2 && ratings[0].Score == 5 && ratings[1].Score == 4
	})).Return(nil).Once()
	mockStore.On("SaveDailyParticipantRatings", mock.Anything, normalizeRatingDate(ratingDate), mock.MatchedBy(func(ratings []*store.ParticipantDailyRating) bool {
		return len(ratings) == 2 && ratings[0].Score == 2 && ratings[1].Score == 1
	})).Return(nil).Once()

	_, err := h.StartDailyRatingsSession(704, 123, ratingDate)
	assert.NoError(t, err)

	firstMsg, err := h.HandleDailyRatingsInteractive(&tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 704},
		From: &tgbotapi.User{ID: 123},
		Text: "5 4",
	})
	assert.NoError(t, err)
	assert.Contains(t, firstMsg.(tgbotapi.MessageConfig).Text, "Saved ratings for 2026-03-13")

	secondMsg, err := h.HandleDailyRatingsInteractive(&tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 704},
		From: &tgbotapi.User{ID: 123},
		Text: "2 1",
	})
	assert.NoError(t, err)
	assert.Contains(t, secondMsg.(tgbotapi.MessageConfig).Text, "overwrite today's ratings")

	mockStore.AssertExpectations(t)
}
