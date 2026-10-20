package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistrationRequests(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	got, err := s.GetRegistrationRequest(ctx, 42)
	require.NoError(t, err)
	assert.Nil(t, got)

	requestedAt := time.Date(2026, 10, 20, 18, 30, 0, 0, time.UTC)
	require.NoError(t, s.UpsertRegistrationRequest(ctx, &store.RegistrationRequest{
		TelegramUserID: 42, FirstName: "Petya", LastName: "Ivanov", Username: "petya_i", RequestedAt: requestedAt,
	}))
	got, err = s.GetRegistrationRequest(ctx, 42)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, store.RegistrationRequest{
		TelegramUserID: 42, FirstName: "Petya", LastName: "Ivanov", Username: "petya_i",
		Status: store.RegistrationPending, RequestedAt: requestedAt,
	}, *got)

	got.Status = store.RegistrationRejected
	require.NoError(t, s.UpsertRegistrationRequest(ctx, got))
	got, err = s.GetRegistrationRequest(ctx, 42)
	require.NoError(t, err)
	assert.Equal(t, store.RegistrationRejected, got.Status)

	require.NoError(t, s.DeleteRegistrationRequest(ctx, 42))
	got, err = s.GetRegistrationRequest(ctx, 42)
	require.NoError(t, err)
	assert.Nil(t, got)

	// Requests never create users.
	users, err := s.ListAllUsers(ctx)
	require.NoError(t, err)
	assert.Empty(t, users)
}
