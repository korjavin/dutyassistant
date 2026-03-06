package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/korjavin/dutyassistant/internal/store"
)

// CreateRecurringChore adds a new recurring chore to the database.
func (s *SQLiteStore) CreateRecurringChore(ctx context.Context, chore *store.RecurringChore) error {
	query := `
		INSERT INTO recurring_chores (description, interval_days, next_run_at, is_active, created_at)
		VALUES (?, ?, ?, ?, ?)
	`
	result, err := s.db.ExecContext(ctx, query,
		chore.Description,
		chore.Interval,
		chore.NextRunAt.UTC().Format(time.RFC3339),
		1, // is_active
		chore.CreatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	chore.ID = id
	return nil
}

// GetRecurringChore retrieves a single recurring chore by its ID.
func (s *SQLiteStore) GetRecurringChore(ctx context.Context, id int64) (*store.RecurringChore, error) {
	query := `
		SELECT id, description, interval_days, next_run_at, is_active, created_at
		FROM recurring_chores
		WHERE id = ?
	`
	row := s.db.QueryRowContext(ctx, query, id)

	var chore store.RecurringChore
	var nextRunAtStr, createdAtStr string
	var isActiveInt int

	err := row.Scan(
		&chore.ID,
		&chore.Description,
		&chore.Interval,
		&nextRunAtStr,
		&isActiveInt,
		&createdAtStr,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // not found
		}
		return nil, err
	}

	chore.IsActive = isActiveInt == 1

	if t, err := time.Parse(time.RFC3339, nextRunAtStr); err == nil {
		chore.NextRunAt = t
	}
	if t, err := time.Parse(time.RFC3339, createdAtStr); err == nil {
		chore.CreatedAt = t
	}

	return &chore, nil
}

// GetActiveRecurringChores retrieves all active recurring chores.
func (s *SQLiteStore) GetActiveRecurringChores(ctx context.Context) ([]*store.RecurringChore, error) {
	query := `
		SELECT id, description, interval_days, next_run_at, is_active, created_at
		FROM recurring_chores
		WHERE is_active = 1
		ORDER BY id ASC
	`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chores []*store.RecurringChore
	for rows.Next() {
		var chore store.RecurringChore
		var nextRunAtStr, createdAtStr string
		var isActiveInt int

		err := rows.Scan(
			&chore.ID,
			&chore.Description,
			&chore.Interval,
			&nextRunAtStr,
			&isActiveInt,
			&createdAtStr,
		)
		if err != nil {
			return nil, err
		}

		chore.IsActive = isActiveInt == 1
		if t, err := time.Parse(time.RFC3339, nextRunAtStr); err == nil {
			chore.NextRunAt = t
		}
		if t, err := time.Parse(time.RFC3339, createdAtStr); err == nil {
			chore.CreatedAt = t
		}

		chores = append(chores, &chore)
	}

	return chores, rows.Err()
}

// GetDueRecurringChores retrieves all active recurring chores that are due before the given time.
func (s *SQLiteStore) GetDueRecurringChores(ctx context.Context, before time.Time) ([]*store.RecurringChore, error) {
	query := `
		SELECT id, description, interval_days, next_run_at, is_active, created_at
		FROM recurring_chores
		WHERE is_active = 1 AND next_run_at <= ?
		ORDER BY id ASC
	`
	rows, err := s.db.QueryContext(ctx, query, before.UTC().Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chores []*store.RecurringChore
	for rows.Next() {
		var chore store.RecurringChore
		var nextRunAtStr, createdAtStr string
		var isActiveInt int

		err := rows.Scan(
			&chore.ID,
			&chore.Description,
			&chore.Interval,
			&nextRunAtStr,
			&isActiveInt,
			&createdAtStr,
		)
		if err != nil {
			return nil, err
		}

		chore.IsActive = isActiveInt == 1
		if t, err := time.Parse(time.RFC3339, nextRunAtStr); err == nil {
			chore.NextRunAt = t
		}
		if t, err := time.Parse(time.RFC3339, createdAtStr); err == nil {
			chore.CreatedAt = t
		}

		chores = append(chores, &chore)
	}

	return chores, rows.Err()
}

// UpdateRecurringChoreNextRun updates the next_run_at time for a recurring chore.
func (s *SQLiteStore) UpdateRecurringChoreNextRun(ctx context.Context, id int64, nextRun time.Time) error {
	query := `
		UPDATE recurring_chores
		SET next_run_at = ?
		WHERE id = ?
	`
	_, err := s.db.ExecContext(ctx, query, nextRun.UTC().Format(time.RFC3339), id)
	return err
}

// CancelRecurringChore sets a recurring chore to inactive.
func (s *SQLiteStore) CancelRecurringChore(ctx context.Context, id int64) error {
	query := `
		UPDATE recurring_chores
		SET is_active = 0
		WHERE id = ?
	`
	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows // Could use a custom not found error here if desired
	}

	return nil
}
