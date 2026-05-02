package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/korjavin/dutyassistant/internal/store"
)

// GetUserByID retrieves a user by their internal ID.
func (s *SQLiteStore) GetUserByID(ctx context.Context, id int64) (*store.User, error) {
	query := `SELECT id, telegram_user_id, first_name, is_admin, is_active, volunteer_queue_days, admin_queue_days, off_duty_start, off_duty_end
	          FROM users WHERE id = ?`
	row := s.db.QueryRowContext(ctx, query, id)
	user, err := scanUser(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found is not an error
		}
		return nil, fmt.Errorf("could not query user: %w", err)
	}
	return user, nil
}
