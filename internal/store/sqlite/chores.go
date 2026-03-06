package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/korjavin/dutyassistant/internal/store"
)

// CreateChore creates a new chore in the database.
func (s *SQLiteStore) CreateChore(ctx context.Context, chore *store.Chore) error {
	query := `INSERT INTO chores (user_id, description, assigned_at, deadline_at, completed_at, reminder_id) VALUES (?, ?, ?, ?, ?, ?)`

	var completedAt interface{}
	if chore.CompletedAt != nil {
		completedAt = chore.CompletedAt.UTC().Format(time.RFC3339)
	}

	res, err := s.db.ExecContext(ctx, query, chore.UserID, chore.Description, chore.AssignedAt.UTC().Format(time.RFC3339), chore.DeadlineAt.UTC().Format(time.RFC3339), completedAt, chore.ReminderID)
	if err != nil {
		return fmt.Errorf("could not insert chore: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("could not retrieve last insert ID for chore: %w", err)
	}
	chore.ID = id
	return nil
}

// GetChoreByReminderID retrieves a chore by its unique reminder ID.
func (s *SQLiteStore) GetChoreByReminderID(ctx context.Context, reminderID string) (*store.Chore, error) {
	query := `
		SELECT c.id, c.user_id, c.description, c.assigned_at, c.deadline_at, c.completed_at, c.reminder_id,
		       u.id, u.telegram_user_id, u.first_name, u.is_admin, u.is_active
		FROM chores c
		JOIN users u ON c.user_id = u.id
		WHERE c.reminder_id = ?
	`
	row := s.db.QueryRowContext(ctx, query, reminderID)
	return scanChoreWithUser(row)
}

// GetActiveChores retrieves all chores that are not completed.
func (s *SQLiteStore) GetActiveChores(ctx context.Context) ([]*store.Chore, error) {
	query := `
		SELECT c.id, c.user_id, c.description, c.assigned_at, c.deadline_at, c.completed_at, c.reminder_id,
		       u.id, u.telegram_user_id, u.first_name, u.is_admin, u.is_active
		FROM chores c
		JOIN users u ON c.user_id = u.id
		WHERE c.completed_at IS NULL
		ORDER BY c.deadline_at ASC
	`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("could not query active chores: %w", err)
	}
	defer rows.Close()

	return scanChoreRowsWithUser(rows)
}

// GetOverdueChores retrieves all active chores where the deadline has passed.
func (s *SQLiteStore) GetOverdueChores(ctx context.Context) ([]*store.Chore, error) {
	query := `
		SELECT c.id, c.user_id, c.description, c.assigned_at, c.deadline_at, c.completed_at, c.reminder_id,
		       u.id, u.telegram_user_id, u.first_name, u.is_admin, u.is_active
		FROM chores c
		JOIN users u ON c.user_id = u.id
		WHERE c.completed_at IS NULL AND c.deadline_at < ?
		ORDER BY c.deadline_at ASC
	`
	now := time.Now().UTC().Format(time.RFC3339)
	rows, err := s.db.QueryContext(ctx, query, now)
	if err != nil {
		return nil, fmt.Errorf("could not query overdue chores: %w", err)
	}
	defer rows.Close()

	return scanChoreRowsWithUser(rows)
}

// CompleteChoreByReminderID marks a chore as completed.
func (s *SQLiteStore) CompleteChoreByReminderID(ctx context.Context, reminderID string) error {
	query := `UPDATE chores SET completed_at = ? WHERE reminder_id = ?`
	_, err := s.db.ExecContext(ctx, query, time.Now().UTC().Format(time.RFC3339), reminderID)
	if err != nil {
		return fmt.Errorf("could not complete chore: %w", err)
	}
	return nil
}

// GetTopOverdueChores retrieves the most frequently overdue chore descriptions.
func (s *SQLiteStore) GetTopOverdueChores(ctx context.Context, limit int) ([]*store.ChoreStat, error) {
	// For simplicity, we just return the most recently overdue chores or most frequently.
	// We'll return the ones that are overdue right now, grouped by description or just the list.
	// The requirement is "top 5 repeating overdue". We can group by description and count.
	// To return *store.Chore we might just populate description and ID=count.
	query := `
		SELECT description, COUNT(*) as cnt
		FROM chores
		WHERE completed_at IS NULL AND deadline_at < ?
		GROUP BY description
		ORDER BY cnt DESC
		LIMIT ?
	`
	now := time.Now().UTC().Format(time.RFC3339)
	rows, err := s.db.QueryContext(ctx, query, now, limit)
	if err != nil {
		return nil, fmt.Errorf("could not query top overdue chores: %w", err)
	}
	defer rows.Close()

	var chores []*store.ChoreStat
	for rows.Next() {
		var desc string
		var cnt int
		if err := rows.Scan(&desc, &cnt); err != nil {
			return nil, err
		}
		chores = append(chores, &store.ChoreStat{Description: desc, Count: cnt})
	}
	return chores, nil
}

// GetTopCompletedChoresUsers retrieves users with the most completed chores.
func (s *SQLiteStore) GetTopCompletedChoresUsers(ctx context.Context, limit int) ([]*store.UserChoreStat, error) {
	query := `
		SELECT u.first_name, COUNT(c.id) as cnt
		FROM chores c
		JOIN users u ON c.user_id = u.id
		WHERE c.completed_at IS NOT NULL
		GROUP BY u.id
		ORDER BY cnt DESC
		LIMIT ?
	`
	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("could not query top completed chores users: %w", err)
	}
	defer rows.Close()

	var stats []*store.UserChoreStat
	for rows.Next() {
		var name string
		var cnt int
		if err := rows.Scan(&name, &cnt); err != nil {
			return nil, err
		}
		stats = append(stats, &store.UserChoreStat{Name: name, Count: cnt})
	}
	return stats, nil
}

// Helper methods
func scanChoreWithUser(row *sql.Row) (*store.Chore, error) {
	chore := &store.Chore{User: &store.User{}}
	var assignedAtStr, deadlineAtStr string
	var completedAtStr sql.NullString

	err := row.Scan(
		&chore.ID, &chore.UserID, &chore.Description, &assignedAtStr, &deadlineAtStr, &completedAtStr, &chore.ReminderID,
		&chore.User.ID, &chore.User.TelegramUserID, &chore.User.FirstName, &chore.User.IsAdmin, &chore.User.IsActive,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found
		}
		return nil, err
	}

	chore.AssignedAt, _ = time.Parse(time.RFC3339, assignedAtStr)
	chore.DeadlineAt, _ = time.Parse(time.RFC3339, deadlineAtStr)
	if completedAtStr.Valid {
		t, _ := time.Parse(time.RFC3339, completedAtStr.String)
		chore.CompletedAt = &t
	}
	return chore, nil
}

func scanChoreRowsWithUser(rows *sql.Rows) ([]*store.Chore, error) {
	var chores []*store.Chore
	for rows.Next() {
		chore := &store.Chore{User: &store.User{}}
		var assignedAtStr, deadlineAtStr string
		var completedAtStr sql.NullString

		err := rows.Scan(
			&chore.ID, &chore.UserID, &chore.Description, &assignedAtStr, &deadlineAtStr, &completedAtStr, &chore.ReminderID,
			&chore.User.ID, &chore.User.TelegramUserID, &chore.User.FirstName, &chore.User.IsAdmin, &chore.User.IsActive,
		)
		if err != nil {
			return nil, err
		}

		chore.AssignedAt, _ = time.Parse(time.RFC3339, assignedAtStr)
		chore.DeadlineAt, _ = time.Parse(time.RFC3339, deadlineAtStr)
		if completedAtStr.Valid {
			t, _ := time.Parse(time.RFC3339, completedAtStr.String)
			chore.CompletedAt = &t
		}
		chores = append(chores, chore)
	}
	return chores, nil
}

// GetLastChoreDigestDate retrieves the date the last daily chore digest was sent.
func (s *SQLiteStore) GetLastChoreDigestDate(ctx context.Context) (string, error) {
	query := `SELECT value FROM settings WHERE key = 'last_chore_digest_date'`
	var value string
	err := s.db.QueryRowContext(ctx, query).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("could not get last chore digest date: %w", err)
	}
	return value, nil
}

// SetLastChoreDigestDate sets the date the last daily chore digest was sent.
func (s *SQLiteStore) SetLastChoreDigestDate(ctx context.Context, date string) error {
	query := `INSERT OR REPLACE INTO settings (key, value, updated_at) VALUES (?, ?, ?)`
	_, err := s.db.ExecContext(ctx, query, "last_chore_digest_date", date, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("could not set last chore digest date: %w", err)
	}
	return nil
}
