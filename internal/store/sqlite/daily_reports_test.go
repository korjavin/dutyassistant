package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDailyReports(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()
	user := &store.User{TelegramUserID: 501, FirstName: "Masha", IsActive: true}
	require.NoError(t, s.CreateUser(ctx, user))
	day := time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)

	got, err := s.GetDailyReport(ctx, user.ID, day)
	require.NoError(t, err)
	assert.Nil(t, got)

	report, err := s.EnsureDailyReport(ctx, user.ID, day)
	require.NoError(t, err)
	assert.Equal(t, store.DailyReport{
		ID: report.ID, UserID: user.ID, UserName: "Masha", TelegramUserID: 501,
		ReportDate: day, Status: store.DailyReportCollecting,
	}, *report)

	// Ensuring again returns the same report.
	again, err := s.EnsureDailyReport(ctx, user.ID, day)
	require.NoError(t, err)
	assert.Equal(t, report.ID, again.ID)

	ok, err := s.AppendDailyReport(ctx, report.ID, "Washed the dishes")
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = s.AppendDailyReport(ctx, report.ID, "Fought with my brother")
	require.NoError(t, err)
	assert.True(t, ok)

	got, err = s.GetDailyReportByID(ctx, report.ID)
	require.NoError(t, err)
	assert.Equal(t, "Washed the dishes\nFought with my brother", got.Text)
	assert.Equal(t, 2, got.Parts)

	// Status changes are compare-and-set.
	ok, err = s.SetDailyReportStatus(ctx, report.ID, store.DailyReportCollecting, store.DailyReportSubmitted)
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = s.SetDailyReportStatus(ctx, report.ID, store.DailyReportCollecting, store.DailyReportSubmitted)
	require.NoError(t, err)
	assert.False(t, ok)

	// A submitted report takes no more parts.
	ok, err = s.AppendDailyReport(ctx, report.ID, "late addition")
	require.NoError(t, err)
	assert.False(t, ok)

	require.NoError(t, s.SaveDailyReportProposal(ctx, report.ID, 3, "Mixed day."))
	got, err = s.GetDailyReport(ctx, user.ID, day)
	require.NoError(t, err)
	assert.Equal(t, store.DailyReportProposed, got.Status)
	assert.Equal(t, 3, got.ProposedScore)
	assert.Equal(t, "Mixed day.", got.Analysis)

	// Another day is a separate report.
	_, err = s.EnsureDailyReport(ctx, user.ID, day.AddDate(0, 0, 1))
	require.NoError(t, err)
	list, err := s.ListDailyReports(ctx, day)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, report.ID, list[0].ID)
}

func TestSaveParticipantRating_KeepsOthers(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()
	alice := &store.User{TelegramUserID: 1, FirstName: "Alice", IsActive: true}
	bob := &store.User{TelegramUserID: 2, FirstName: "Bob", IsActive: true}
	require.NoError(t, s.CreateUser(ctx, alice))
	require.NoError(t, s.CreateUser(ctx, bob))
	day := time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)

	require.NoError(t, s.SaveParticipantRating(ctx, &store.ParticipantDailyRating{ParticipantID: alice.ID, RatingDate: day, Score: 4}))
	require.NoError(t, s.SaveParticipantRating(ctx, &store.ParticipantDailyRating{ParticipantID: bob.ID, RatingDate: day, Score: 2}))
	// Overwrite Alice's score; Bob's stays.
	require.NoError(t, s.SaveParticipantRating(ctx, &store.ParticipantDailyRating{ParticipantID: alice.ID, RatingDate: day, Score: 5}))

	ratings, err := s.GetCurrentMonthParticipantRatings(ctx, day)
	require.NoError(t, err)
	require.Len(t, ratings, 2)
	assert.Equal(t, "Alice", ratings[0].ParticipantName)
	assert.Equal(t, 5, ratings[0].Score)
	assert.Equal(t, "Bob", ratings[1].ParticipantName)
	assert.Equal(t, 2, ratings[1].Score)

	assert.Error(t, s.SaveParticipantRating(ctx, &store.ParticipantDailyRating{ParticipantID: bob.ID, RatingDate: day, Score: 6}))
}

func TestGetChoresCompletedBetween(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()
	user := &store.User{TelegramUserID: 7, FirstName: "Vanya", IsActive: true}
	require.NoError(t, s.CreateUser(ctx, user))

	now := time.Now().UTC()
	for _, c := range []*store.Chore{
		{UserID: user.ID, Description: "done", AssignedAt: now.Add(-time.Hour), DeadlineAt: now.Add(time.Hour), ReminderID: "r1"},
		{UserID: user.ID, Description: "pending", AssignedAt: now.Add(-time.Hour), DeadlineAt: now.Add(time.Hour), ReminderID: "r2"},
	} {
		require.NoError(t, s.CreateChore(ctx, c))
	}
	require.NoError(t, s.CompleteChoreByReminderID(ctx, "r1"))

	done, err := s.GetChoresCompletedBetween(ctx, user.ID, now.Add(-time.Hour), now.Add(time.Hour))
	require.NoError(t, err)
	require.Len(t, done, 1)
	assert.Equal(t, "done", done[0].Description)

	done, err = s.GetChoresCompletedBetween(ctx, user.ID, now.Add(time.Hour), now.Add(2*time.Hour))
	require.NoError(t, err)
	assert.Empty(t, done)
}
