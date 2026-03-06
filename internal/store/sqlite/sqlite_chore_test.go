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
