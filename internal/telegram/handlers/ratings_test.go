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

func TestHandleRatingsCalendar_PopulatedMonth(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := NewWithAdminID(mockStore, nil, 0, 123, nil)

	originalNow := TimeNow
	TimeNow = func() time.Time {
		return time.Date(2026, time.March, 3, 12, 0, 0, 0, time.UTC)
	}
	defer func() {
		TimeNow = originalNow
	}()

	participants := []*store.User{
		{ID: 10, FirstName: "Alice"},
		{ID: 11, FirstName: "Bob"},
	}
	ratings := []*store.ParticipantDailyRating{
		{ParticipantID: 10, ParticipantName: "Alice", RatingDate: time.Date(2026, time.March, 1, 8, 0, 0, 0, time.UTC), Score: 5},
		{ParticipantID: 11, ParticipantName: "Bob", RatingDate: time.Date(2026, time.March, 1, 8, 0, 0, 0, time.UTC), Score: 4},
		{ParticipantID: 10, ParticipantName: "Alice", RatingDate: time.Date(2026, time.March, 2, 8, 0, 0, 0, time.UTC), Score: 3},
	}

	mockStore.On("GetParticipantsForRating", mock.Anything).Return(participants, nil).Once()
	mockStore.On("GetCurrentMonthParticipantRatings", mock.Anything, time.Date(2026, time.March, 3, 0, 0, 0, 0, time.UTC)).Return(ratings, nil).Once()

	msg, err := h.HandleRatingsCalendar(&tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 800},
		From: &tgbotapi.User{ID: 123},
	})
	assert.NoError(t, err)
	assert.Equal(t, tgbotapi.ModeHTML, msg.ParseMode)
	assert.Contains(t, msg.Text, "Participant ratings for March 2026")
	assert.Contains(t, msg.Text, "Showing 2026-03-01 through 2026-03-03.")
	assert.Contains(t, msg.Text, "<pre>")
	assert.Contains(t, msg.Text, "Date        Alice  Bob")
	assert.Contains(t, msg.Text, "2026-03-01  5      4")
	assert.Contains(t, msg.Text, "2026-03-02  3      -")
	assert.Contains(t, msg.Text, "2026-03-03  -      -")
	assert.Contains(t, msg.Text, "Missing scores are shown as -.")

	mockStore.AssertExpectations(t)
}

func TestHandleRatingsCalendar_EmptyMonth(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := NewWithAdminID(mockStore, nil, 0, 123, nil)

	originalNow := TimeNow
	TimeNow = func() time.Time {
		return time.Date(2026, time.March, 2, 12, 0, 0, 0, time.UTC)
	}
	defer func() {
		TimeNow = originalNow
	}()

	participants := []*store.User{
		{ID: 10, FirstName: "Alice"},
	}

	mockStore.On("GetParticipantsForRating", mock.Anything).Return(participants, nil).Once()
	mockStore.On("GetCurrentMonthParticipantRatings", mock.Anything, time.Date(2026, time.March, 2, 0, 0, 0, 0, time.UTC)).Return([]*store.ParticipantDailyRating{}, nil).Once()

	msg, err := h.HandleRatingsCalendar(&tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 801},
		From: &tgbotapi.User{ID: 123},
	})
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "2026-03-01  -")
	assert.Contains(t, msg.Text, "2026-03-02  -")
	assert.Contains(t, msg.Text, "Missing scores are shown as -.")

	mockStore.AssertExpectations(t)
}

func TestHandleRatingsCalendar_KeepsPreviouslyRatedInactiveParticipants(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := NewWithAdminID(mockStore, nil, 0, 123, nil)

	originalNow := TimeNow
	TimeNow = func() time.Time {
		return time.Date(2026, time.March, 3, 12, 0, 0, 0, time.UTC)
	}
	defer func() {
		TimeNow = originalNow
	}()

	ratings := []*store.ParticipantDailyRating{
		{ParticipantID: 42, ParticipantName: "Zoe", RatingDate: time.Date(2026, time.March, 1, 8, 0, 0, 0, time.UTC), Score: 4},
	}

	mockStore.On("GetParticipantsForRating", mock.Anything).Return([]*store.User{}, nil).Once()
	mockStore.On("GetCurrentMonthParticipantRatings", mock.Anything, time.Date(2026, time.March, 3, 0, 0, 0, 0, time.UTC)).Return(ratings, nil).Once()

	msg, err := h.HandleRatingsCalendar(&tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 803},
		From: &tgbotapi.User{ID: 123},
	})
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "Participant ratings for March 2026")
	assert.Contains(t, msg.Text, "Date        Zoe")
	assert.Contains(t, msg.Text, "2026-03-01  4")
	assert.Contains(t, msg.Text, "2026-03-02  -")
	assert.Contains(t, msg.Text, "2026-03-03  -")

	mockStore.AssertExpectations(t)
}

func TestHandleRatingsCalendar_IncludesPreviouslyRatedInactiveParticipants(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := NewWithAdminID(mockStore, nil, 0, 123, nil)

	originalNow := TimeNow
	TimeNow = func() time.Time {
		return time.Date(2026, time.March, 3, 12, 0, 0, 0, time.UTC)
	}
	defer func() {
		TimeNow = originalNow
	}()

	participants := []*store.User{
		{ID: 10, FirstName: "Alice"},
	}
	ratings := []*store.ParticipantDailyRating{
		{ParticipantID: 10, ParticipantName: "Alice", RatingDate: time.Date(2026, time.March, 1, 8, 0, 0, 0, time.UTC), Score: 5},
		{ParticipantID: 11, ParticipantName: "Bob", RatingDate: time.Date(2026, time.March, 1, 8, 0, 0, 0, time.UTC), Score: 4},
	}

	mockStore.On("GetParticipantsForRating", mock.Anything).Return(participants, nil).Once()
	mockStore.On("GetCurrentMonthParticipantRatings", mock.Anything, time.Date(2026, time.March, 3, 0, 0, 0, 0, time.UTC)).Return(ratings, nil).Once()

	msg, err := h.HandleRatingsCalendar(&tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 803},
		From: &tgbotapi.User{ID: 123},
	})
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "Date        Alice  Bob")
	assert.Contains(t, msg.Text, "2026-03-01  5      4")

	mockStore.AssertExpectations(t)
}

func TestHandleRatingsCalendar_SortsCombinedParticipantsInStableOrder(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := NewWithAdminID(mockStore, nil, 0, 123, nil)

	originalNow := TimeNow
	TimeNow = func() time.Time {
		return time.Date(2026, time.March, 3, 12, 0, 0, 0, time.UTC)
	}
	defer func() {
		TimeNow = originalNow
	}()

	participants := []*store.User{
		{ID: 11, FirstName: "Bob"},
	}
	ratings := []*store.ParticipantDailyRating{
		{ParticipantID: 10, ParticipantName: "Alice", RatingDate: time.Date(2026, time.March, 1, 8, 0, 0, 0, time.UTC), Score: 5},
		{ParticipantID: 11, ParticipantName: "Bob", RatingDate: time.Date(2026, time.March, 1, 8, 0, 0, 0, time.UTC), Score: 4},
	}

	mockStore.On("GetParticipantsForRating", mock.Anything).Return(participants, nil).Once()
	mockStore.On("GetCurrentMonthParticipantRatings", mock.Anything, time.Date(2026, time.March, 3, 0, 0, 0, 0, time.UTC)).Return(ratings, nil).Once()

	msg, err := h.HandleRatingsCalendar(&tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 805},
		From: &tgbotapi.User{ID: 123},
	})
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "Date        Alice  Bob")
	assert.Contains(t, msg.Text, "2026-03-01  5      4")

	mockStore.AssertExpectations(t)
}

func TestHandleRatingsCalendar_AdminAccessControl(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := NewWithAdminID(mockStore, nil, 0, 123, nil)

	msg, err := h.HandleRatingsCalendar(&tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 802},
		From: &tgbotapi.User{ID: 999},
	})
	assert.NoError(t, err)
	assert.Equal(t, adminOnlyMessage, msg.Text)
}

func TestPrepareDailyRatingsReminder_SkipsWithoutParticipants(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := NewWithAdminID(mockStore, nil, 0, 123, nil)

	mockStore.On("GetParticipantsForRating", mock.Anything).Return([]*store.User{}, nil).Once()

	msg, ok, err := h.PrepareDailyRatingsReminder(900, 123, time.Date(2026, time.March, 13, 20, 50, 0, 0, time.UTC))
	assert.NoError(t, err)
	assert.False(t, ok)
	assert.Nil(t, msg)

	_, exists := h.SessionManager.GetSession(900)
	assert.False(t, exists)

	mockStore.AssertExpectations(t)
}

func TestPrepareDailyRatingsReminder_NonAdminGetsAdminOnlyMessage(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := NewWithAdminID(mockStore, nil, 0, 123, nil)

	msg, ok, err := h.PrepareDailyRatingsReminder(901, 456, time.Date(2026, time.March, 13, 20, 50, 0, 0, time.UTC))
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.NotNil(t, msg)
	assert.Equal(t, adminOnlyMessage, msg.Text)

	_, exists := h.SessionManager.GetSession(901)
	assert.False(t, exists)

	mockStore.AssertNotCalled(t, "GetParticipantsForRating", mock.Anything)
}

func TestPrepareDailyRatingsReminder_DoesNotOverrideExistingNonRatingsSession(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := NewWithAdminID(mockStore, nil, 0, 123, nil)

	h.SessionManager.StartSession(902, 123, SessionTypeChoreCreation)

	msg, ok, err := h.PrepareDailyRatingsReminder(902, 123, time.Date(2026, time.March, 13, 20, 50, 0, 0, time.UTC))
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.NotNil(t, msg)
	assert.Equal(t, "Finish your current admin workflow before submitting participant ratings.", msg.Text)

	session, exists := h.SessionManager.GetSession(902)
	assert.True(t, exists)
	assert.Equal(t, SessionTypeChoreCreation, session.Type)

	mockStore.AssertNotCalled(t, "GetParticipantsForRating", mock.Anything)
}

func TestPrepareDailyRatingsReminder_ExcludesConfiguredAdminParticipant(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := NewWithAdminID(mockStore, nil, 0, 123, nil)

	participants := []*store.User{
		{ID: 1, TelegramUserID: 123, FirstName: "Admin"},
		{ID: 2, TelegramUserID: 999, FirstName: "Alice"},
	}
	mockStore.On("GetParticipantsForRating", mock.Anything).Return(participants, nil).Once()

	msg, ok, err := h.PrepareDailyRatingsReminder(903, 123, time.Date(2026, time.March, 13, 20, 50, 0, 0, time.UTC))
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.NotNil(t, msg)
	assert.Contains(t, msg.Text, "1. Alice")
	assert.NotContains(t, msg.Text, "Admin")

	session, exists := h.SessionManager.GetSession(903)
	assert.True(t, exists)
	storedParticipants, ok := sessionParticipantsFromSession(session)
	assert.True(t, ok)
	assert.Len(t, storedParticipants, 1)
	assert.Equal(t, int64(2), storedParticipants[0].ID)

	mockStore.AssertExpectations(t)
}

func TestStartDailyRatingsSession_BuildsStablePrompt(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := NewWithAdminID(mockStore, nil, 0, 123, nil)

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

func TestStartDailyRatingsSession_NoParticipants(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := NewWithAdminID(mockStore, nil, 0, 123, nil)

	mockStore.On("GetParticipantsForRating", mock.Anything).Return([]*store.User{}, nil).Once()

	msg, err := h.StartDailyRatingsSession(501, 123, time.Date(2026, time.March, 13, 20, 50, 0, 0, time.UTC))
	assert.NoError(t, err)
	assert.Equal(t, "No active participants are available for rating right now.", msg.Text)

	mockStore.AssertExpectations(t)
}

func TestHandleDailyRatingsInteractive_SendsGroupNotification(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := NewWithAdminID(mockStore, nil, -1001, 123, nil)

	ratingDate := time.Date(2026, time.March, 13, 20, 50, 0, 0, time.UTC)
	participants := []*store.User{
		{ID: 10, FirstName: "Alice"},
		{ID: 11, FirstName: "Bob"},
	}

	mockStore.On("GetParticipantsForRating", mock.Anything).Return(participants, nil).Once()
	mockStore.On("SaveDailyParticipantRatings", mock.Anything, normalizeRatingDate(ratingDate), mock.MatchedBy(func(ratings []*store.ParticipantDailyRating) bool {
		return len(ratings) == 2 && ratings[0].Score == 5 && ratings[1].Score == 3
	})).Return(nil).Once()

	originalNow := TimeNow
	TimeNow = func() time.Time {
		return ratingDate // Same day as session creation
	}
	defer func() {
		TimeNow = originalNow
	}()

	_, err := h.StartDailyRatingsSession(700, 123, ratingDate)
	assert.NoError(t, err)

	msg, err := h.HandleDailyRatingsInteractive(&tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 700},
		From: &tgbotapi.User{ID: 123},
		Text: "5 3",
	})
	assert.NoError(t, err)
	assert.NotNil(t, msg)

	mockStore.AssertExpectations(t)
}

func TestHandleDailyRatingsInteractive_ValidSubmission(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := NewWithAdminID(mockStore, nil, 0, 123, nil)

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

	originalNow := TimeNow
	TimeNow = func() time.Time {
		return ratingDate // Same day as session creation
	}
	defer func() {
		TimeNow = originalNow
	}()

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
	h := NewWithAdminID(mockStore, nil, 0, 123, nil)

	ratingDate := time.Date(2026, time.March, 13, 0, 0, 0, 0, time.UTC)
	participants := []*store.User{
		{ID: 10, FirstName: "Alice"},
		{ID: 11, FirstName: "Bob"},
	}

	mockStore.On("GetParticipantsForRating", mock.Anything).Return(participants, nil).Once()

	originalNow := TimeNow
	TimeNow = func() time.Time {
		return ratingDate // Same day as session creation
	}
	defer func() {
		TimeNow = originalNow
	}()

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
	h := NewWithAdminID(mockStore, nil, 0, 123, nil)

	ratingDate := time.Date(2026, time.March, 13, 0, 0, 0, 0, time.UTC)
	participants := []*store.User{
		{ID: 10, FirstName: "Alice"},
		{ID: 11, FirstName: "Bob"},
	}

	mockStore.On("GetParticipantsForRating", mock.Anything).Return(participants, nil).Once()

	originalNow := TimeNow
	TimeNow = func() time.Time {
		return ratingDate // Same day as session creation
	}
	defer func() {
		TimeNow = originalNow
	}()

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

func TestHandleDailyRatingsInteractive_ExpiredSessionMissingParticipants(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := NewWithAdminID(mockStore, nil, 0, 123, nil)

	h.SessionManager.StartSession(705, 123, SessionTypeDailyRatings)
	session, exists := h.SessionManager.GetSession(705)
	assert.True(t, exists)
	session.SetData(ratingSessionDateKey, normalizeRatingDate(time.Date(2026, time.March, 13, 0, 0, 0, 0, time.UTC)))

	msg, err := h.HandleDailyRatingsInteractive(&tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 705},
		From: &tgbotapi.User{ID: 123},
		Text: "5 5",
	})
	assert.NoError(t, err)
	assert.Equal(t, "The rating session expired. Please start a new rating prompt.", msg.(tgbotapi.MessageConfig).Text)

	_, exists = h.SessionManager.GetSession(705)
	assert.False(t, exists)
}

func TestHandleDailyRatingsInteractive_ExpiredSessionMissingDate(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := NewWithAdminID(mockStore, nil, 0, 123, nil)

	h.SessionManager.StartSession(706, 123, SessionTypeDailyRatings)
	session, exists := h.SessionManager.GetSession(706)
	assert.True(t, exists)
	session.SetData(ratingSessionParticipantsKey, []ratingSessionParticipant{
		{ID: 10, Name: "Alice"},
	})

	msg, err := h.HandleDailyRatingsInteractive(&tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 706},
		From: &tgbotapi.User{ID: 123},
		Text: "5",
	})
	assert.NoError(t, err)
	assert.Equal(t, "The rating session expired. Please start a new rating prompt.", msg.(tgbotapi.MessageConfig).Text)

	_, exists = h.SessionManager.GetSession(706)
	assert.False(t, exists)
}

func TestHandleDailyRatingsInteractive_UnauthorizedSenderIgnored(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := NewWithAdminID(mockStore, nil, 0, 123, nil)

	ratingDate := time.Date(2026, time.March, 13, 0, 0, 0, 0, time.UTC)
	participants := []*store.User{
		{ID: 10, FirstName: "Alice"},
		{ID: 11, FirstName: "Bob"},
	}

	mockStore.On("GetParticipantsForRating", mock.Anything).Return(participants, nil).Once()

	originalNow := TimeNow
	TimeNow = func() time.Time {
		return ratingDate // Same day as session creation
	}
	defer func() {
		TimeNow = originalNow
	}()

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
	h := NewWithAdminID(mockStore, nil, 0, 123, nil)

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

	originalNow := TimeNow
	TimeNow = func() time.Time {
		return ratingDate // Same day as session creation
	}
	defer func() {
		TimeNow = originalNow
	}()

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

func TestHandleDailyRatingsInteractive_SaveFailureReturnsGenericError(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := NewWithAdminID(mockStore, nil, 0, 123, nil)

	ratingDate := time.Date(2026, time.March, 13, 0, 0, 0, 0, time.UTC)
	participants := []*store.User{
		{ID: 10, FirstName: "Alice"},
		{ID: 11, FirstName: "Bob"},
	}

	mockStore.On("GetParticipantsForRating", mock.Anything).Return(participants, nil).Once()
	mockStore.On("SaveDailyParticipantRatings", mock.Anything, normalizeRatingDate(ratingDate), mock.Anything).Return(assert.AnError).Once()

	originalNow := TimeNow
	TimeNow = func() time.Time {
		return ratingDate // Same day as session creation
	}
	defer func() {
		TimeNow = originalNow
	}()

	_, err := h.StartDailyRatingsSession(707, 123, ratingDate)
	assert.NoError(t, err)

	msg, err := h.HandleDailyRatingsInteractive(&tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 707},
		From: &tgbotapi.User{ID: 123},
		Text: "5 4",
	})
	assert.NoError(t, err)
	assert.Equal(t, genericErrorMessage, msg.(tgbotapi.MessageConfig).Text)

	_, exists := h.SessionManager.GetSession(707)
	assert.True(t, exists)

	mockStore.AssertExpectations(t)
}

func TestHandleDailyRatingsInteractive_RejectsSubmissionAfterMonthEndCutoff(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := NewWithAdminID(mockStore, nil, 0, 123, nil)

	originalNow := TimeNow
	TimeNow = func() time.Time {
		return time.Date(2026, time.March, 31, 21, 0, 0, 0, time.UTC)
	}
	defer func() {
		TimeNow = originalNow
	}()

	ratingDate := time.Date(2026, time.March, 31, 20, 50, 0, 0, time.UTC)
	participants := []*store.User{
		{ID: 10, FirstName: "Alice"},
		{ID: 11, FirstName: "Bob"},
	}

	mockStore.On("GetParticipantsForRating", mock.Anything).Return(participants, nil).Once()

	_, err := h.StartDailyRatingsSession(708, 123, ratingDate)
	assert.NoError(t, err)

	msg, err := h.HandleDailyRatingsInteractive(&tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 708},
		From: &tgbotapi.User{ID: 123},
		Text: "5 4",
	})
	assert.NoError(t, err)
	assert.Equal(t, "The rating session expired. Please start a new rating prompt.", msg.(tgbotapi.MessageConfig).Text)

	_, exists := h.SessionManager.GetSession(708)
	assert.False(t, exists)

	mockStore.AssertExpectations(t)
}

func TestFormatDailyAndMonthlySummary(t *testing.T) {
	now := time.Date(2026, time.March, 13, 0, 0, 0, 0, time.UTC)
	dailyRatings := []*store.ParticipantDailyRating{
		{ParticipantName: "Alice", Score: 5},
		{ParticipantName: "Bob", Score: 4},
	}
	totals := []*store.ParticipantMonthlyTotal{
		{ParticipantName: "Alice", TotalScore: 15},
		{ParticipantName: "Bob", TotalScore: 12},
		{ParticipantName: "Cara", TotalScore: 8},
	}

	result := formatDailyAndMonthlySummary(dailyRatings, totals, now)
	expected := "<b>Daily Ratings for 2026-03-13</b>\n" +
		"Alice: 5\n" +
		"Bob: 4\n\n" +
		"<b>Monthly Standings (March 2026)</b>\n" +
		"1. Alice - 15 point(s)\n" +
		"2. Bob - 12 point(s)\n" +
		"3. Cara - 8 point(s)"

	assert.Equal(t, expected, result)
}

func TestBuildMonthlyRatingsWinnersAnnouncement_LastDayFormatting(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := NewWithAdminID(mockStore, nil, -1001, 123, nil)

	now := time.Date(2026, time.March, 31, 21, 0, 0, 0, time.UTC)
	totals := []*store.ParticipantMonthlyTotal{
		{ParticipantID: 10, ParticipantName: "Alice", TotalScore: 14, DaysRated: 4},
		{ParticipantID: 11, ParticipantName: "Bob", TotalScore: 11, DaysRated: 4},
		{ParticipantID: 12, ParticipantName: "Cara", TotalScore: 8, DaysRated: 3},
		{ParticipantID: 13, ParticipantName: "Dan", TotalScore: 6, DaysRated: 2},
	}

	mockStore.On("GetMonthlyParticipantTotals", mock.Anything, 2026, time.March).Return(totals, nil).Once()

	msg, ok, err := h.BuildMonthlyRatingsWinnersAnnouncement(now)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.NotNil(t, msg)
	assert.Equal(t, int64(-1001), msg.ChatID)
	assert.Contains(t, msg.Text, "Monthly participant ratings for March 2026")
	assert.Contains(t, msg.Text, "1st: Alice with 14 point(s)")
	assert.Contains(t, msg.Text, "2nd: Bob with 11 point(s)")
	assert.Contains(t, msg.Text, "3rd: Cara with 8 point(s)")
	assert.Contains(t, msg.Text, "1. Alice - 14 point(s) across 4 rated day(s)")
	assert.Contains(t, msg.Text, "4. Dan - 6 point(s) across 2 rated day(s)")

	mockStore.AssertExpectations(t)
}

func TestBuildMonthlyRatingsWinnersAnnouncement_NotLastDaySkips(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := NewWithAdminID(mockStore, nil, -1001, 123, nil)

	msg, ok, err := h.BuildMonthlyRatingsWinnersAnnouncement(time.Date(2026, time.March, 30, 21, 0, 0, 0, time.UTC))
	assert.NoError(t, err)
	assert.False(t, ok)
	assert.Nil(t, msg)

	mockStore.AssertNotCalled(t, "GetMonthlyParticipantTotals", mock.Anything, mock.Anything, mock.Anything)
}

func TestIsLastDayOfMonth(t *testing.T) {
	assert.True(t, isLastDayOfMonth(time.Date(2026, time.February, 28, 12, 0, 0, 0, time.UTC)))
	assert.True(t, isLastDayOfMonth(time.Date(2024, time.February, 29, 12, 0, 0, 0, time.UTC)))
	assert.False(t, isLastDayOfMonth(time.Date(2026, time.March, 30, 12, 0, 0, 0, time.UTC)))
	assert.True(t, isLastDayOfMonth(time.Date(2026, time.March, 31, 12, 0, 0, 0, time.UTC)))
}

func TestNormalizeRatingDate_UsesBerlinCalendarDay(t *testing.T) {
	assert.Equal(
		t,
		time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC),
		normalizeRatingDate(time.Date(2026, time.February, 28, 23, 30, 0, 0, time.UTC)),
	)
}

func TestHandleRatingsCalendar_UsesBerlinDateBoundary(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := NewWithAdminID(mockStore, nil, 0, 123, nil)

	originalNow := TimeNow
	TimeNow = func() time.Time {
		return time.Date(2026, time.February, 28, 23, 30, 0, 0, time.UTC)
	}
	defer func() {
		TimeNow = originalNow
	}()

	participants := []*store.User{
		{ID: 10, FirstName: "Alice"},
	}

	mockStore.On("GetParticipantsForRating", mock.Anything).Return(participants, nil).Once()
	mockStore.On("GetCurrentMonthParticipantRatings", mock.Anything, time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)).Return([]*store.ParticipantDailyRating{}, nil).Once()

	msg, err := h.HandleRatingsCalendar(&tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 804},
		From: &tgbotapi.User{ID: 123},
	})
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "Participant ratings for March 2026")
	assert.Contains(t, msg.Text, "Showing 2026-03-01 through 2026-03-01.")

	mockStore.AssertExpectations(t)
}

func TestHandleRatingsCalendar_ExcludesConfiguredAdminParticipant(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := NewWithAdminID(mockStore, nil, 0, 123, nil)

	originalNow := TimeNow
	TimeNow = func() time.Time {
		return time.Date(2026, time.March, 2, 12, 0, 0, 0, time.UTC)
	}
	defer func() {
		TimeNow = originalNow
	}()

	participants := []*store.User{
		{ID: 1, TelegramUserID: 123, FirstName: "Admin"},
		{ID: 2, TelegramUserID: 999, FirstName: "Alice"},
	}

	mockStore.On("GetParticipantsForRating", mock.Anything).Return(participants, nil).Once()
	mockStore.On("GetCurrentMonthParticipantRatings", mock.Anything, time.Date(2026, time.March, 2, 0, 0, 0, 0, time.UTC)).Return([]*store.ParticipantDailyRating{}, nil).Once()

	msg, err := h.HandleRatingsCalendar(&tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 806},
		From: &tgbotapi.User{ID: 123},
	})
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "Date        Alice")
	assert.NotContains(t, msg.Text, "Admin")

	mockStore.AssertExpectations(t)
}

func TestBuildMonthlyRatingsWinnersAnnouncement_WaitsForInFlightSave(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := NewWithAdminID(mockStore, nil, -1001, 123, nil)

	originalNow := TimeNow
	TimeNow = func() time.Time {
		return time.Date(2026, time.March, 31, 18, 59, 0, 0, time.UTC)
	}
	defer func() {
		TimeNow = originalNow
	}()

	participants := []*store.User{
		{ID: 10, FirstName: "Alice"},
	}
	saveStarted := make(chan struct{})
	releaseSave := make(chan struct{})

	mockStore.On("GetParticipantsForRating", mock.Anything).Return(participants, nil).Once()
	mockStore.On("SaveDailyParticipantRatings", mock.Anything, time.Date(2026, time.March, 31, 0, 0, 0, 0, time.UTC), mock.Anything).Run(func(args mock.Arguments) {
		close(saveStarted)
		<-releaseSave
	}).Return(nil).Once()
	mockStore.On("GetMonthlyParticipantTotals", mock.Anything, 2026, time.March).Return([]*store.ParticipantMonthlyTotal{
		{ParticipantID: 10, ParticipantName: "Alice", TotalScore: 5, DaysRated: 1},
	}, nil).Once()

	_, err := h.StartDailyRatingsSession(807, 123, time.Date(2026, time.March, 31, 18, 50, 0, 0, time.UTC))
	assert.NoError(t, err)

	saveDone := make(chan struct{})
	go func() {
		defer close(saveDone)
		_, _ = h.HandleDailyRatingsInteractive(&tgbotapi.Message{
			Chat: &tgbotapi.Chat{ID: 807},
			From: &tgbotapi.User{ID: 123},
			Text: "5",
		})
	}()

	<-saveStarted

	announcementDone := make(chan struct{})
	go func() {
		defer close(announcementDone)
		_, _, _ = h.BuildMonthlyRatingsWinnersAnnouncement(time.Date(2026, time.March, 31, 19, 0, 0, 0, time.UTC))
	}()

	select {
	case <-announcementDone:
		t.Fatal("announcement should wait for the in-flight rating save")
	case <-time.After(100 * time.Millisecond):
	}

	close(releaseSave)
	<-saveDone
	<-announcementDone

	mockStore.AssertExpectations(t)
}
