package sqlite

import (
	"context"
	"testing"

	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtendedVolunteer(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	alice := &store.User{TelegramUserID: 1, FirstName: "Alice", IsActive: true}
	bob := &store.User{TelegramUserID: 2, FirstName: "Bob", IsActive: true}
	require.NoError(t, s.CreateUser(ctx, alice))
	require.NoError(t, s.CreateUser(ctx, bob))

	ok, err := s.IsExtendedVolunteer(ctx, alice.ID)
	require.NoError(t, err)
	assert.False(t, ok, "default is not extended")

	require.NoError(t, s.SetExtendedVolunteer(ctx, alice.ID, true))
	require.NoError(t, s.SetExtendedVolunteer(ctx, alice.ID, true), "enabling twice is fine")
	ok, _ = s.IsExtendedVolunteer(ctx, alice.ID)
	assert.True(t, ok)
	ok, _ = s.IsExtendedVolunteer(ctx, bob.ID)
	assert.False(t, ok, "other users are unaffected")

	require.NoError(t, s.SetExtendedVolunteer(ctx, alice.ID, false))
	ok, _ = s.IsExtendedVolunteer(ctx, alice.ID)
	assert.False(t, ok)
}
