package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/korjavin/dutyassistant/internal/store"
)

// recurringChoreColumns is the SELECT list shared by the recurring chore queries.
// The LEFT JOIN on users fills in the fixed assignee's name, if any.
const recurringChoreColumns = `
		SELECT rc.id, rc.description, rc.interval_days, rc.next_run_at, rc.is_active, rc.created_at,
			rc.assignee_user_id, u.first_name, rc.ends_at
		FROM recurring_chores rc
		LEFT JOIN users u ON u.id = rc.assignee_user_id
`

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

// scanRecurringChore scans a row selected with recurringChoreColumns.
func scanRecurringChore(row rowScanner) (*store.RecurringChore, error) {
	var chore store.RecurringChore
	var nextRunAtStr, createdAtStr string
	var isActiveInt int
	var assigneeID sql.NullInt64
	var assigneeName, endsAtStr sql.NullString

	err := row.Scan(
		&chore.ID,
		&chore.Description,
		&chore.Interval,
		&nextRunAtStr,
		&isActiveInt,
		&createdAtStr,
		&assigneeID,
		&assigneeName,
		&endsAtStr,
	)
	if err != nil {
		return nil, err
	}

	chore.IsActive = isActiveInt == 1
	chore.AssigneeUserID = assigneeID.Int64
	chore.AssigneeName = assigneeName.String

	if t, err := time.Parse(time.RFC3339, nextRunAtStr); err == nil {
		chore.NextRunAt = t
	}
	if t, err := time.Parse(time.RFC3339, createdAtStr); err == nil {
		chore.CreatedAt = t
	}
	if endsAtStr.Valid {
		if t, err := time.Parse(time.RFC3339, endsAtStr.String); err == nil {
			chore.EndsAt = &t
		}
	}

	return &chore, nil
}

// CreateRecurringChore adds a new recurring chore to the database.
func (s *SQLiteStore) CreateRecurringChore(ctx context.Context, chore *store.RecurringChore) error {
	query := `
		INSERT INTO recurring_chores (description, interval_days, next_run_at, is_active, created_at, assignee_user_id, ends_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	var assigneeID sql.NullInt64
	if chore.AssigneeUserID != 0 {
		assigneeID = sql.NullInt64{Int64: chore.AssigneeUserID, Valid: true}
	}
	var endsAt sql.NullString
	if chore.EndsAt != nil {
		endsAt = sql.NullString{String: chore.EndsAt.UTC().Format(time.RFC3339), Valid: true}
	}
	result, err := s.db.ExecContext(ctx, query,
		chore.Description,
		chore.Interval,
		chore.NextRunAt.UTC().Format(time.RFC3339),
		1, // is_active
		chore.CreatedAt.UTC().Format(time.RFC3339),
		assigneeID,
		endsAt,
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
	query := recurringChoreColumns + `
		WHERE rc.id = ?
	`
	chore, err := scanRecurringChore(s.db.QueryRowContext(ctx, query, id))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // not found
		}
		return nil, err
	}
	return chore, nil
}

// GetActiveRecurringChores retrieves all active recurring chores.
func (s *SQLiteStore) GetActiveRecurringChores(ctx context.Context) ([]*store.RecurringChore, error) {
	query := recurringChoreColumns + `
		WHERE rc.is_active = 1
		ORDER BY rc.id ASC
	`
	return s.queryRecurringChores(ctx, query)
}

// GetDueRecurringChores retrieves all active recurring chores that are due before the given time.
func (s *SQLiteStore) GetDueRecurringChores(ctx context.Context, before time.Time) ([]*store.RecurringChore, error) {
	query := recurringChoreColumns + `
		WHERE rc.is_active = 1 AND rc.next_run_at <= ?
		ORDER BY rc.id ASC
	`
	return s.queryRecurringChores(ctx, query, before.UTC().Format(time.RFC3339))
}

func (s *SQLiteStore) queryRecurringChores(ctx context.Context, query string, args ...any) ([]*store.RecurringChore, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chores []*store.RecurringChore
	for rows.Next() {
		chore, err := scanRecurringChore(rows)
		if err != nil {
			return nil, err
		}
		chores = append(chores, chore)
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

// UpdateRecurringChoreDescription updates the description for a recurring chore.
func (s *SQLiteStore) UpdateRecurringChoreDescription(ctx context.Context, id int64, description string) error {
	query := `
		UPDATE recurring_chores
		SET description = ?
		WHERE id = ? AND is_active = 1
	`
	result, err := s.db.ExecContext(ctx, query, description, id)
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

// CancelRecurringChore sets a recurring chore to inactive. It is also how a
// chore that has run for the last time (one-off, or past its end) is retired.
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
