package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAllSviniyaBalances(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	// Create test users
	user1 := &store.User{TelegramUserID: 1, FirstName: "Alice", IsAdmin: false, IsActive: true}
	user2 := &store.User{TelegramUserID: 2, FirstName: "Bob", IsAdmin: false, IsActive: true}
	require.NoError(t, s.CreateUser(ctx, user1))
	require.NoError(t, s.CreateUser(ctx, user2))

	// Set some balances
	require.NoError(t, s.SetSviniyaBalance(ctx, user1.ID, 5))
	require.NoError(t, s.SetSviniyaBalance(ctx, user2.ID, 3))

	// Get all balances
	balances, err := s.GetAllSviniyaBalances(ctx)
	require.NoError(t, err)
	require.Len(t, balances, 2)

	// Should be ordered by balance DESC, then name
	assert.Equal(t, user1.ID, balances[0].UserID)
	assert.Equal(t, "Alice", balances[0].UserName)
	assert.Equal(t, 5, balances[0].Balance)
	assert.Equal(t, user2.ID, balances[1].UserID)
	assert.Equal(t, "Bob", balances[1].UserName)
	assert.Equal(t, 3, balances[1].Balance)
}

func TestGetSviniyaBalance(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	// Create test user
	user := &store.User{TelegramUserID: 1, FirstName: "Alice", IsAdmin: false, IsActive: true}
	require.NoError(t, s.CreateUser(ctx, user))

	// Test non-existent balance
	balance, err := s.GetSviniyaBalance(ctx, user.ID)
	require.NoError(t, err)
	require.Nil(t, balance)

	// Set a balance
	require.NoError(t, s.SetSviniyaBalance(ctx, user.ID, 7))

	// Get the balance
	balance, err = s.GetSviniyaBalance(ctx, user.ID)
	require.NoError(t, err)
	require.NotNil(t, balance)
	assert.Equal(t, user.ID, balance.UserID)
	assert.Equal(t, "Alice", balance.UserName)
	assert.Equal(t, 7, balance.Balance)
}

func TestAddSviniyaBalance(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	// Create test user
	user := &store.User{TelegramUserID: 1, FirstName: "Alice", IsAdmin: false, IsActive: true}
	require.NoError(t, s.CreateUser(ctx, user))

	// Add to non-existent balance (creates it)
	err := s.AddSviniyaBalance(ctx, user.ID, 3)
	require.NoError(t, err)

	balance, err := s.GetSviniyaBalance(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, 3, balance.Balance)

	// Add more to existing balance
	err = s.AddSviniyaBalance(ctx, user.ID, 2)
	require.NoError(t, err)

	balance, err = s.GetSviniyaBalance(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, 5, balance.Balance)
}

func TestSetSviniyaBalance(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	// Create test user
	user := &store.User{TelegramUserID: 1, FirstName: "Alice", IsAdmin: false, IsActive: true}
	require.NoError(t, s.CreateUser(ctx, user))

	// Set initial balance
	err := s.SetSviniyaBalance(ctx, user.ID, 10)
	require.NoError(t, err)

	balance, err := s.GetSviniyaBalance(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, 10, balance.Balance)

	// Update to new value
	err = s.SetSviniyaBalance(ctx, user.ID, 15)
	require.NoError(t, err)

	balance, err = s.GetSviniyaBalance(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, 15, balance.Balance)

	// Set to zero
	err = s.SetSviniyaBalance(ctx, user.ID, 0)
	require.NoError(t, err)

	balance, err = s.GetSviniyaBalance(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, balance.Balance)
}

func TestDecrementSviniyaBalance(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	// Create test user
	user := &store.User{TelegramUserID: 1, FirstName: "Alice", IsAdmin: false, IsActive: true}
	require.NoError(t, s.CreateUser(ctx, user))

	// Set initial balance
	require.NoError(t, s.SetSviniyaBalance(ctx, user.ID, 3))

	// Decrement once
	err := s.DecrementSviniyaBalance(ctx, user.ID)
	require.NoError(t, err)

	balance, err := s.GetSviniyaBalance(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, balance.Balance)

	// Decrement to zero
	err = s.DecrementSviniyaBalance(ctx, user.ID)
	require.NoError(t, err)
	err = s.DecrementSviniyaBalance(ctx, user.ID)
	require.NoError(t, err)

	balance, err = s.GetSviniyaBalance(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, balance.Balance)

	// Decrement below zero (should fail with insufficient balance error)
	err = s.DecrementSviniyaBalance(ctx, user.ID)
	require.Error(t, err)
	assert.True(t, errors.Is(err, store.ErrInsufficientBalance), "Should return ErrInsufficientBalance")

	balance, err = s.GetSviniyaBalance(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, balance.Balance)
}

func TestSviniyaBalancesWithMultipleUsers(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	// Create multiple users
	alice := &store.User{TelegramUserID: 1, FirstName: "Alice", IsAdmin: false, IsActive: true}
	bob := &store.User{TelegramUserID: 2, FirstName: "Bob", IsAdmin: false, IsActive: true}
	charlie := &store.User{TelegramUserID: 3, FirstName: "Charlie", IsAdmin: false, IsActive: true}
	require.NoError(t, s.CreateUser(ctx, alice))
	require.NoError(t, s.CreateUser(ctx, bob))
	require.NoError(t, s.CreateUser(ctx, charlie))

	// Only give balances to Alice and Bob (Charlie should not appear in GetAllSviniyaBalances)
	require.NoError(t, s.SetSviniyaBalance(ctx, alice.ID, 10))
	require.NoError(t, s.SetSviniyaBalance(ctx, bob.ID, 5))

	// Get all balances - should only return users with balances
	balances, err := s.GetAllSviniyaBalances(ctx)
	require.NoError(t, err)
	require.Len(t, balances, 2)

	// Verify ordering (balance DESC, then name)
	assert.Equal(t, alice.ID, balances[0].UserID)
	assert.Equal(t, "Alice", balances[0].UserName)
	assert.Equal(t, 10, balances[0].Balance)
	assert.Equal(t, bob.ID, balances[1].UserID)
	assert.Equal(t, "Bob", balances[1].UserName)
	assert.Equal(t, 5, balances[1].Balance)
}

func TestGetSviniyaMonthlyGrant_NotGranted(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	// Check for a month that hasn't been granted
	userID, granted, err := s.GetSviniyaMonthlyGrant(ctx, 2026, time.March)
	require.NoError(t, err)
	assert.False(t, granted, "Should not be granted")
	assert.Zero(t, userID, "UserID should be zero when not granted")
}

func TestRecordSviniyaMonthlyGrantAndGet(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	// Create test user
	user := &store.User{TelegramUserID: 1, FirstName: "Alice", IsAdmin: false, IsActive: true}
	require.NoError(t, s.CreateUser(ctx, user))

	// Record a grant for March 2026
	err := s.RecordSviniyaMonthlyGrant(ctx, 2026, time.March, user.ID)
	require.NoError(t, err)

	// Check that it was recorded
	userID, granted, err := s.GetSviniyaMonthlyGrant(ctx, 2026, time.March)
	require.NoError(t, err)
	assert.True(t, granted, "Should be granted")
	assert.Equal(t, user.ID, userID, "UserID should match")

	// Check a different month - should not be granted
	userID, granted, err = s.GetSviniyaMonthlyGrant(ctx, 2026, time.April)
	require.NoError(t, err)
	assert.False(t, granted, "Different month should not be granted")
}

func TestRecordSviniyaMonthlyGrant_Idempotent(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	// Create test user
	user := &store.User{TelegramUserID: 1, FirstName: "Alice", IsAdmin: false, IsActive: true}
	require.NoError(t, s.CreateUser(ctx, user))

	// Record a grant for March 2026
	err := s.RecordSviniyaMonthlyGrant(ctx, 2026, time.March, user.ID)
	require.NoError(t, err)

	// Try to record again - should fail due to UNIQUE constraint
	err = s.RecordSviniyaMonthlyGrant(ctx, 2026, time.March, user.ID)
	assert.Error(t, err, "Should fail when recording duplicate grant for same month")
}

func TestGrantSviniyaForMonth_Success(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	// Create test user
	user := &store.User{TelegramUserID: 1, FirstName: "Alice", IsAdmin: false, IsActive: true}
	require.NoError(t, s.CreateUser(ctx, user))

	// Grant sviniya for March 2026
	err := s.GrantSviniyaForMonth(ctx, 2026, time.March, user.ID)
	require.NoError(t, err, "First grant should succeed")

	// Verify balance was incremented
	balance, err := s.GetSviniyaBalance(ctx, user.ID)
	require.NoError(t, err)
	require.NotNil(t, balance)
	assert.Equal(t, 1, balance.Balance, "Balance should be 1 after grant")

	// Verify grant was recorded
	userID, granted, err := s.GetSviniyaMonthlyGrant(ctx, 2026, time.March)
	require.NoError(t, err)
	assert.True(t, granted, "Grant should be recorded")
	assert.Equal(t, user.ID, userID, "Grant should be for the correct user")
}

func TestGrantSviniyaForMonth_Idempotent(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	// Create test user
	user := &store.User{TelegramUserID: 1, FirstName: "Alice", IsAdmin: false, IsActive: true}
	require.NoError(t, s.CreateUser(ctx, user))

	t.Logf("User created with ID: %d", user.ID)

	// Grant sviniya for March 2026
	err := s.GrantSviniyaForMonth(ctx, 2026, time.March, user.ID)
	require.NoError(t, err, "First grant should succeed")

	t.Logf("First grant succeeded")

	// Verify balance was created and incremented
	balance, err := s.GetSviniyaBalance(ctx, user.ID)
	require.NoError(t, err)
	require.NotNil(t, balance)
	assert.Equal(t, 1, balance.Balance, "Balance should be 1 after first grant")

	t.Logf("Balance verified: %d", balance.Balance)

	// Try to grant again for same month - should fail with already granted error
	err = s.GrantSviniyaForMonth(ctx, 2026, time.March, user.ID)
	require.Error(t, err, "Second grant should fail")
	assert.True(t, errors.Is(err, store.ErrSviniyaAlreadyGranted), "Should return ErrSviniyaAlreadyGranted")

	t.Logf("Second grant failed as expected: %v", err)

	// Verify balance was only incremented once (not twice)
	balance, err = s.GetSviniyaBalance(ctx, user.ID)
	t.Logf("Getting balance after failed grant...")
	require.NoError(t, err)
	require.NotNil(t, balance)
	assert.Equal(t, 1, balance.Balance, "Balance should still be 1, not 2")
}

func TestGrantSviniyaForMonth_AddsToExistingBalance(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	// Create test user
	user := &store.User{TelegramUserID: 1, FirstName: "Alice", IsAdmin: false, IsActive: true}
	require.NoError(t, s.CreateUser(ctx, user))

	// Set existing balance to 5
	require.NoError(t, s.SetSviniyaBalance(ctx, user.ID, 5))

	// Grant sviniya for March 2026
	err := s.GrantSviniyaForMonth(ctx, 2026, time.March, user.ID)
	require.NoError(t, err, "Grant should succeed")

	// Verify balance was incremented (5 + 1 = 6)
	balance, err := s.GetSviniyaBalance(ctx, user.ID)
	require.NoError(t, err)
	require.NotNil(t, balance)
	assert.Equal(t, 6, balance.Balance, "Balance should be 6 after adding grant to existing balance")
}

