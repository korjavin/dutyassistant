package sqlite

import (
	"context"
	"testing"

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

	// Decrement below zero (should stay at zero)
	err = s.DecrementSviniyaBalance(ctx, user.ID)
	require.NoError(t, err)

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
