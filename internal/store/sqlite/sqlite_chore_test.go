package sqlite

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecurringChores(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	// 1. Create a recurring chore
	now := time.Now()
	nextRun := now.Add(24 * time.Hour)
	chore := &store.RecurringChore{
		Description: "Take out trash",
		Interval:    3,
		NextRunAt:   nextRun,
		CreatedAt:   now,
	}

	err := s.CreateRecurringChore(ctx, chore)
	require.NoError(t, err)
	assert.NotZero(t, chore.ID)

	// 2. Get the recurring chore
	fetchedChore, err := s.GetRecurringChore(ctx, chore.ID)
	require.NoError(t, err)
	require.NotNil(t, fetchedChore)
	assert.Equal(t, chore.Description, fetchedChore.Description)
	assert.Equal(t, chore.Interval, fetchedChore.Interval)
	assert.Equal(t, chore.NextRunAt.Unix(), fetchedChore.NextRunAt.Unix())
	assert.True(t, fetchedChore.IsActive)

	// 3. Get active recurring chores
	chores, err := s.GetActiveRecurringChores(ctx)
	require.NoError(t, err)
	require.Len(t, chores, 1)
	assert.Equal(t, chore.ID, chores[0].ID)

	// 4. Update next run
	newNextRun := nextRun.Add(3 * 24 * time.Hour)
	err = s.UpdateRecurringChoreNextRun(ctx, chore.ID, newNextRun)
	require.NoError(t, err)

	fetchedChore, _ = s.GetRecurringChore(ctx, chore.ID)
	assert.Equal(t, newNextRun.Unix(), fetchedChore.NextRunAt.Unix())

	// 5. Get due recurring chores
	dueChores, err := s.GetDueRecurringChores(ctx, now)
	require.NoError(t, err)
	require.Empty(t, dueChores)

	dueChores, err = s.GetDueRecurringChores(ctx, newNextRun.Add(time.Hour))
	require.NoError(t, err)
	require.Len(t, dueChores, 1)

	// 6. Cancel the recurring chore
	err = s.CancelRecurringChore(ctx, chore.ID)
	require.NoError(t, err)

	fetchedChore, _ = s.GetRecurringChore(ctx, chore.ID)
	assert.False(t, fetchedChore.IsActive)

	// Active list should be empty
	chores, _ = s.GetActiveRecurringChores(ctx)
	assert.Empty(t, chores)

	// Due list should be empty (since it's inactive)
	dueChores, _ = s.GetDueRecurringChores(ctx, newNextRun.Add(time.Hour))
	assert.Empty(t, dueChores)

	// Test canceling non-existent chore
	err = s.CancelRecurringChore(ctx, 999)
	assert.Equal(t, sql.ErrNoRows, err)
}

func TestGetActiveChoresByUserID(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	// 1. Create test users
	user1 := &store.User{TelegramUserID: 111, FirstName: "UserOne"}
	err := s.CreateUser(ctx, user1)
	require.NoError(t, err)

	user2 := &store.User{TelegramUserID: 222, FirstName: "UserTwo"}
	err = s.CreateUser(ctx, user2)
	require.NoError(t, err)

	now := time.Now()

	// 2. Add chores for user1
	chore1 := &store.Chore{
		UserID:      user1.ID,
		Description: "UserOne Active Chore",
		AssignedAt:  now,
		DeadlineAt:  now.Add(24 * time.Hour),
		ReminderID:  "u1_active",
	}
	err = s.CreateChore(ctx, chore1)
	require.NoError(t, err)

	completedTime := now.Add(-1 * time.Hour)
	chore2 := &store.Chore{
		UserID:      user1.ID,
		Description: "UserOne Completed Chore",
		AssignedAt:  now.Add(-48 * time.Hour),
		DeadlineAt:  now.Add(-24 * time.Hour),
		CompletedAt: &completedTime,
		ReminderID:  "u1_completed",
	}
	err = s.CreateChore(ctx, chore2)
	require.NoError(t, err)

	// 3. Add chores for user2
	chore3 := &store.Chore{
		UserID:      user2.ID,
		Description: "UserTwo Active Chore",
		AssignedAt:  now,
		DeadlineAt:  now.Add(24 * time.Hour),
		ReminderID:  "u2_active",
	}
	err = s.CreateChore(ctx, chore3)
	require.NoError(t, err)

	// 4. Test: User with no chores (non-existent user ID)
	chores, err := s.GetActiveChoresByUserID(ctx, 9999)
	require.NoError(t, err)
	assert.Empty(t, chores, "Expected empty list for user with no chores")

	// 5. Test: Get user1's active chores
	chores1, err := s.GetActiveChoresByUserID(ctx, user1.ID)
	require.NoError(t, err)
	require.Len(t, chores1, 1, "Expected exactly 1 active chore for user1")
	assert.Equal(t, chore1.ID, chores1[0].ID)
	assert.Equal(t, "UserOne Active Chore", chores1[0].Description)
	assert.Nil(t, chores1[0].CompletedAt)

	// 6. Test: Get user2's active chores
	chores2, err := s.GetActiveChoresByUserID(ctx, user2.ID)
	require.NoError(t, err)
	require.Len(t, chores2, 1, "Expected exactly 1 active chore for user2")
	assert.Equal(t, chore3.ID, chores2[0].ID)
	assert.Equal(t, "UserTwo Active Chore", chores2[0].Description)
}

func TestGetUserWeeklyStats(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	// 1. Create test users
	user1 := &store.User{TelegramUserID: 111, FirstName: "Alice"}
	err := s.CreateUser(ctx, user1)
	require.NoError(t, err)

	user2 := &store.User{TelegramUserID: 222, FirstName: "Bob"}
	err = s.CreateUser(ctx, user2)
	require.NoError(t, err)

	now := time.Now().UTC()
	since := now.Add(-7 * 24 * time.Hour)

	// Test 1: Empty results
	stats, err := s.GetUserWeeklyStats(ctx, since)
	require.NoError(t, err)
	assert.Empty(t, stats)

	// Add chores for Alice
	// Chore 1: Completed quickly, on time
	c1Completed := now.Add(-3 * 24 * time.Hour)
	err = s.CreateChore(ctx, &store.Chore{
		UserID:      user1.ID,
		Description: "Alice Chore 1",
		AssignedAt:  c1Completed.Add(-1 * time.Hour), // 1 hr exec
		DeadlineAt:  c1Completed.Add(24 * time.Hour), // Not late
		CompletedAt: &c1Completed,
		ReminderID:  "a_c1",
	})
	require.NoError(t, err)

	// Chore 2: Completed, late
	c2Completed := now.Add(-1 * 24 * time.Hour)
	err = s.CreateChore(ctx, &store.Chore{
		UserID:      user1.ID,
		Description: "Alice Chore 2",
		AssignedAt:  c2Completed.Add(-3 * time.Hour), // 3 hr exec
		DeadlineAt:  c2Completed.Add(-1 * time.Hour), // 1 hr late
		CompletedAt: &c2Completed,
		ReminderID:  "a_c2",
	})
	require.NoError(t, err)

	// Chore 3: Out of the 'since' window (completed 8 days ago)
	c3Completed := now.Add(-8 * 24 * time.Hour)
	err = s.CreateChore(ctx, &store.Chore{
		UserID:      user1.ID,
		Description: "Alice Chore 3",
		AssignedAt:  c3Completed.Add(-2 * time.Hour),
		DeadlineAt:  c3Completed.Add(24 * time.Hour),
		CompletedAt: &c3Completed,
		ReminderID:  "a_c3",
	})
	require.NoError(t, err)

	// Add chores for Bob
	// Chore 1: Completed, on time
	b1Completed := now.Add(-2 * 24 * time.Hour)
	err = s.CreateChore(ctx, &store.Chore{
		UserID:      user2.ID,
		Description: "Bob Chore 1",
		AssignedAt:  b1Completed.Add(-2 * time.Hour), // 2 hr exec
		DeadlineAt:  b1Completed.Add(24 * time.Hour),
		CompletedAt: &b1Completed,
		ReminderID:  "b_c1",
	})
	require.NoError(t, err)

	// Test 2: Check results
	stats, err = s.GetUserWeeklyStats(ctx, since)
	require.NoError(t, err)
	require.Len(t, stats, 2)

	// Order should be Alice (2 completed), Bob (1 completed)
	assert.Equal(t, "Alice", stats[0].Name)
	assert.Equal(t, 2, stats[0].CompletedCount)
	// Alice: 1hr + 3hr exec = avg 2hr (7200s)
	assert.Equal(t, float64(7200), stats[0].AvgExecSeconds)
	// Alice: 0 late + 1hr late = avg 30m (1800s)
	assert.Equal(t, float64(1800), stats[0].AvgLateSeconds)

	assert.Equal(t, "Bob", stats[1].Name)
	assert.Equal(t, 1, stats[1].CompletedCount)
	// Bob: 2hr exec = 7200s
	assert.Equal(t, float64(7200), stats[1].AvgExecSeconds)
	// Bob: 0 late = 0s
	assert.Equal(t, float64(0), stats[1].AvgLateSeconds)
}
