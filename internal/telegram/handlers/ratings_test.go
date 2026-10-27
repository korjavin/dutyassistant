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

	result := formatDailyAndMonthlySummary(dailyRatings, totals, now, nil)
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
	mockStore.On("GrantSviniyaForMonth", mock.Anything, 2026, time.March, int64(10)).Return(nil).Once()

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

func TestBuildMonthlyRatingsWinnersAnnouncement_GrantsSviniyaToWinner(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := NewWithAdminID(mockStore, nil, -1001, 123, nil)

	now := time.Date(2026, time.March, 31, 21, 0, 0, 0, time.UTC)
	totals := []*store.ParticipantMonthlyTotal{
		{ParticipantID: 10, ParticipantName: "Alice", TotalScore: 14, DaysRated: 4},
		{ParticipantID: 11, ParticipantName: "Bob", TotalScore: 11, DaysRated: 4},
		{ParticipantID: 12, ParticipantName: "Cara", TotalScore: 8, DaysRated: 3},
	}

	mockStore.On("GetMonthlyParticipantTotals", mock.Anything, 2026, time.March).Return(totals, nil).Once()
	mockStore.On("GrantSviniyaForMonth", mock.Anything, 2026, time.March, int64(10)).Return(nil).Once()

	msg, ok, err := h.BuildMonthlyRatingsWinnersAnnouncement(now)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.NotNil(t, msg)
	assert.Equal(t, int64(-1001), msg.ChatID)
	assert.Contains(t, msg.Text, "Monthly participant ratings for March 2026")
	assert.Contains(t, msg.Text, "1st: Alice with 14 point(s)")

	mockStore.AssertExpectations(t)
}

func TestBuildMonthlyRatingsWinnersAnnouncement_ToleratesStoreError(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := NewWithAdminID(mockStore, nil, -1001, 123, nil)

	now := time.Date(2026, time.March, 31, 21, 0, 0, 0, time.UTC)
	totals := []*store.ParticipantMonthlyTotal{
		{ParticipantID: 10, ParticipantName: "Alice", TotalScore: 14, DaysRated: 4},
		{ParticipantID: 11, ParticipantName: "Bob", TotalScore: 11, DaysRated: 4},
		{ParticipantID: 12, ParticipantName: "Cara", TotalScore: 8, DaysRated: 3},
	}

	mockStore.On("GetMonthlyParticipantTotals", mock.Anything, 2026, time.March).Return(totals, nil).Once()
	mockStore.On("GrantSviniyaForMonth", mock.Anything, 2026, time.March, int64(10)).Return(assert.AnError).Once()

	msg, ok, err := h.BuildMonthlyRatingsWinnersAnnouncement(now)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.NotNil(t, msg)
	assert.Equal(t, int64(-1001), msg.ChatID)
	assert.Contains(t, msg.Text, "Monthly participant ratings for March 2026")
	assert.Contains(t, msg.Text, "1st: Alice with 14 point(s)")

	mockStore.AssertExpectations(t)
}

func TestBuildMonthlyRatingsWinnersAnnouncement_Idempotent(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := NewWithAdminID(mockStore, nil, -1001, 123, nil)

	now := time.Date(2026, time.March, 31, 21, 0, 0, 0, time.UTC)
	totals := []*store.ParticipantMonthlyTotal{
		{ParticipantID: 10, ParticipantName: "Alice", TotalScore: 14, DaysRated: 4},
		{ParticipantID: 11, ParticipantName: "Bob", TotalScore: 11, DaysRated: 4},
		{ParticipantID: 12, ParticipantName: "Cara", TotalScore: 8, DaysRated: 3},
	}

	// Simulate that the sviniya was already granted for this month
	// GrantSviniyaForMonth will return ErrSviniyaAlreadyGranted to test idempotent path
	mockStore.On("GetMonthlyParticipantTotals", mock.Anything, 2026, time.March).Return(totals, nil).Once()
	mockStore.On("GrantSviniyaForMonth", mock.Anything, 2026, time.March, int64(10)).Return(store.ErrSviniyaAlreadyGranted).Once()

	msg, ok, err := h.BuildMonthlyRatingsWinnersAnnouncement(now)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.NotNil(t, msg)
	assert.Equal(t, int64(-1001), msg.ChatID)
	assert.Contains(t, msg.Text, "Monthly participant ratings for March 2026")
	assert.Contains(t, msg.Text, "1st: Alice with 14 point(s)")

	mockStore.AssertExpectations(t)
}

func TestBuildRatingsCalendarTable_ShowsEarScores(t *testing.T) {
	participants := []ratingSessionParticipant{
		{ID: 10, Name: "Alice"},
		{ID: 11, Name: "Bob"},
	}
	ratings := []*store.ParticipantDailyRating{
		{ParticipantID: 10, ParticipantName: "Alice", RatingDate: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC), Score: 5, HasEar: true},
		{ParticipantID: 11, ParticipantName: "Bob", RatingDate: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC), Score: 4},
		{ParticipantID: 10, ParticipantName: "Alice", RatingDate: time.Date(2026, time.March, 2, 0, 0, 0, 0, time.UTC), Score: 5},
		{ParticipantID: 11, ParticipantName: "Bob", RatingDate: time.Date(2026, time.March, 2, 0, 0, 0, 0, time.UTC), Score: 5, HasEar: true},
	}
	now := time.Date(2026, time.March, 2, 0, 0, 0, 0, time.UTC)

	table := buildRatingsCalendarTable(participants, ratings, now)
	assert.Contains(t, table, "2026-03-01  5e     4")
	assert.Contains(t, table, "2026-03-02  5      5e")
}

func TestHandleRatingsCalendar_ShowsEarInCalendar(t *testing.T) {
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
		{ID: 11, FirstName: "Bob"},
	}
	ratings := []*store.ParticipantDailyRating{
		{ParticipantID: 10, ParticipantName: "Alice", RatingDate: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC), Score: 5, HasEar: true},
		{ParticipantID: 11, ParticipantName: "Bob", RatingDate: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC), Score: 4},
	}

	mockStore.On("GetParticipantsForRating", mock.Anything).Return(participants, nil).Once()
	mockStore.On("GetCurrentMonthParticipantRatings", mock.Anything, time.Date(2026, time.March, 2, 0, 0, 0, 0, time.UTC)).Return(ratings, nil).Once()

	msg, err := h.HandleRatingsCalendar(&tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 810},
		From: &tgbotapi.User{ID: 123},
	})
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "5e")
	assert.Contains(t, msg.Text, "2026-03-01")

	mockStore.AssertExpectations(t)
}

func TestFormatDailyAndMonthlySummary_WithEars(t *testing.T) {
	now := time.Date(2026, time.March, 13, 0, 0, 0, 0, time.UTC)
	dailyRatings := []*store.ParticipantDailyRating{
		{ParticipantName: "Alice", Score: 5, HasEar: true},
		{ParticipantName: "Bob", Score: 4},
	}
	totals := []*store.ParticipantMonthlyTotal{
		{ParticipantName: "Alice", TotalScore: 47, EarCount: 3},
		{ParticipantName: "Bob", TotalScore: 40, EarCount: 0},
	}

	result := formatDailyAndMonthlySummary(dailyRatings, totals, now, nil)
	assert.Contains(t, result, "Alice: 5e")
	assert.Contains(t, result, "Bob: 4")
	assert.NotContains(t, result, "Bob: 4e")
	assert.Contains(t, result, "1. Alice - 47 point(s), 3 ear(s)")
	assert.Contains(t, result, "2. Bob - 40 point(s)")
	assert.NotContains(t, result, "Bob - 40 point(s), 0 ear(s)")
}

func TestFormatMonthlyRatingsWinnersDigest_WithEars(t *testing.T) {
	now := time.Date(2026, time.March, 31, 0, 0, 0, 0, time.UTC)
	totals := []*store.ParticipantMonthlyTotal{
		{ParticipantID: 10, ParticipantName: "Alice", TotalScore: 47, DaysRated: 10, EarCount: 3},
		{ParticipantID: 11, ParticipantName: "Bob", TotalScore: 40, DaysRated: 10, EarCount: 1},
		{ParticipantID: 12, ParticipantName: "Cara", TotalScore: 35, DaysRated: 8, EarCount: 0},
	}

	result := formatMonthlyRatingsWinnersDigest(totals, now, false)

	// Winners section shows ears
	assert.Contains(t, result, "1st: Alice with 47 point(s), 3 ear(s)")
	assert.Contains(t, result, "2nd: Bob with 40 point(s), 1 ear(s)")
	assert.Contains(t, result, "3rd: Cara with 35 point(s)\n")
	assert.NotContains(t, result, "Cara with 35 point(s), 0 ear(s)")

	// Totals section shows ears
	assert.Contains(t, result, "1. Alice - 47 point(s), 3 ear(s) across 10 rated day(s)")
	assert.Contains(t, result, "2. Bob - 40 point(s), 1 ear(s) across 10 rated day(s)")
	assert.Contains(t, result, "3. Cara - 35 point(s) across 8 rated day(s)")
}

func TestFormatMonthlyRatingsWinnersDigest_NoEars(t *testing.T) {
	now := time.Date(2026, time.March, 31, 0, 0, 0, 0, time.UTC)
	totals := []*store.ParticipantMonthlyTotal{
		{ParticipantID: 10, ParticipantName: "Alice", TotalScore: 14, DaysRated: 4, EarCount: 0},
		{ParticipantID: 11, ParticipantName: "Bob", TotalScore: 11, DaysRated: 4, EarCount: 0},
	}

	result := formatMonthlyRatingsWinnersDigest(totals, now, false)
	assert.NotContains(t, result, "ear(s)")
	assert.Contains(t, result, "1st: Alice with 14 point(s)")
	assert.Contains(t, result, "1. Alice - 14 point(s) across 4 rated day(s)")
}
