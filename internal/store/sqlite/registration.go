package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/korjavin/dutyassistant/internal/store"
)

// UpsertRegistrationRequest creates or replaces the registration request of a Telegram user.
func (s *SQLiteStore) UpsertRegistrationRequest(ctx context.Context, req *store.RegistrationRequest) error {
	status := req.Status
	if status == "" {
		status = store.RegistrationPending
	}
	requestedAt := req.RequestedAt
	if requestedAt.IsZero() {
		requestedAt = time.Now()
	}
	query := `
		INSERT INTO registration_requests (telegram_user_id, first_name, last_name, username, status, requested_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(telegram_user_id) DO UPDATE SET
			first_name = excluded.first_name,
			last_name = excluded.last_name,
			username = excluded.username,
			status = excluded.status,
			requested_at = excluded.requested_at
	`
	_, err := s.db.ExecContext(ctx, query, req.TelegramUserID, req.FirstName, req.LastName, req.Username, status, requestedAt.UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("could not save registration request: %w", err)
	}
	return nil
}

// GetRegistrationRequest returns the registration request of a Telegram user, or nil if none.
func (s *SQLiteStore) GetRegistrationRequest(ctx context.Context, telegramUserID int64) (*store.RegistrationRequest, error) {
	query := `SELECT telegram_user_id, first_name, last_name, username, status, requested_at
	          FROM registration_requests WHERE telegram_user_id = ?`
	req := &store.RegistrationRequest{}
	var requestedAt string
	err := s.db.QueryRowContext(ctx, query, telegramUserID).Scan(
		&req.TelegramUserID, &req.FirstName, &req.LastName, &req.Username, &req.Status, &requestedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found is not an error
		}
		return nil, fmt.Errorf("could not query registration request: %w", err)
	}
	if t, parseErr := time.Parse(time.RFC3339, requestedAt); parseErr == nil {
		req.RequestedAt = t
	}
	return req, nil
}

// DeleteRegistrationRequest removes the registration request of a Telegram user.
func (s *SQLiteStore) DeleteRegistrationRequest(ctx context.Context, telegramUserID int64) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM registration_requests WHERE telegram_user_id = ?`, telegramUserID); err != nil {
		return fmt.Errorf("could not delete registration request: %w", err)
	}
	return nil
}
