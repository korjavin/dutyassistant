package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/korjavin/dutyassistant/internal/store"

	_ "modernc.org/sqlite"
)

// SQLiteStore is a concrete implementation of the store.Store interface for SQLite.
type SQLiteStore struct {
	db *sql.DB
}

// New creates a new SQLiteStore instance.
func New(ctx context.Context, dataSourceName string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	s := &SQLiteStore{db: db}

	if err := s.migrate(ctx); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return s, nil
}

// migrate creates the necessary database tables if they don't exist.
func (s *SQLiteStore) migrate(ctx context.Context) error {
	const schema = `
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			telegram_user_id INTEGER UNIQUE NOT NULL,
			first_name TEXT NOT NULL,
			is_admin INTEGER NOT NULL DEFAULT 0,
			is_active INTEGER NOT NULL DEFAULT 1
		);

		CREATE TABLE IF NOT EXISTS duties (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			duty_date TEXT UNIQUE NOT NULL,
			assignment_type TEXT NOT NULL,
			created_at TEXT NOT NULL,
			FOREIGN KEY(user_id) REFERENCES users(id)
		);

		CREATE TABLE IF NOT EXISTS round_robin_state (
			user_id INTEGER PRIMARY KEY,
			assignment_count INTEGER NOT NULL DEFAULT 0,
			last_assigned_timestamp TEXT,
			FOREIGN KEY(user_id) REFERENCES users(id)
		);
	`
	_, err := s.db.ExecContext(ctx, schema)
	return err
}

// CreateUser adds a new user to the database.
func (s *SQLiteStore) CreateUser(ctx context.Context, user *store.User) error {
	query := `INSERT INTO users (telegram_user_id, first_name, is_admin, is_active) VALUES (?, ?, ?, ?)`
	res, err := s.db.ExecContext(ctx, query, user.TelegramUserID, user.FirstName, user.IsAdmin, user.IsActive)
	if err != nil {
		return fmt.Errorf("could not insert user: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("could not retrieve last insert ID: %w", err)
	}
	user.ID = id
	return nil
}

// GetUserByTelegramID retrieves a user by their Telegram ID.
func (s *SQLiteStore) GetUserByTelegramID(ctx context.Context, id int64) (*store.User, error) {
	query := `SELECT id, telegram_user_id, first_name, is_admin, is_active FROM users WHERE telegram_user_id = ?`
	row := s.db.QueryRowContext(ctx, query, id)
	user := &store.User{}
	err := row.Scan(&user.ID, &user.TelegramUserID, &user.FirstName, &user.IsAdmin, &user.IsActive)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found is not an error
		}
		return nil, fmt.Errorf("could not query user: %w", err)
	}
	return user, nil
}

// ListActiveUsers retrieves all users who are currently active.
func (s *SQLiteStore) ListActiveUsers(ctx context.Context) ([]*store.User, error) {
	query := `SELECT id, telegram_user_id, first_name, is_admin, is_active FROM users WHERE is_active = 1`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("could not query active users: %w", err)
	}
	defer rows.Close()

	var users []*store.User
	for rows.Next() {
		user := &store.User{}
		if err := rows.Scan(&user.ID, &user.TelegramUserID, &user.FirstName, &user.IsAdmin, &user.IsActive); err != nil {
			return nil, fmt.Errorf("could not scan user row: %w", err)
		}
		users = append(users, user)
	}
	return users, nil
}

// FindUserByName retrieves a user by their first name.
func (s *SQLiteStore) FindUserByName(ctx context.Context, name string) (*store.User, error) {
	query := `SELECT id, telegram_user_id, first_name, is_admin, is_active FROM users WHERE first_name = ?`
	row := s.db.QueryRowContext(ctx, query, name)
	user := &store.User{}
	err := row.Scan(&user.ID, &user.TelegramUserID, &user.FirstName, &user.IsAdmin, &user.IsActive)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found is not an error
		}
		return nil, fmt.Errorf("could not query user by name: %w", err)
	}
	return user, nil
}

// UpdateUser updates a user's details.
func (s *SQLiteStore) UpdateUser(ctx context.Context, user *store.User) error {
	query := `UPDATE users SET first_name = ?, is_admin = ?, is_active = ? WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, user.FirstName, user.IsAdmin, user.IsActive, user.ID)
	if err != nil {
		return fmt.Errorf("could not update user: %w", err)
	}
	return nil
}

// CreateDuty creates a new duty assignment.
func (s *SQLiteStore) CreateDuty(ctx context.Context, duty *store.Duty) error {
	query := `INSERT INTO duties (user_id, duty_date, assignment_type, created_at) VALUES (?, ?, ?, ?)`
	res, err := s.db.ExecContext(ctx, query, duty.UserID, duty.DutyDate.Format("2006-01-02"), string(duty.AssignmentType), duty.CreatedAt.UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("could not insert duty: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("could not retrieve last insert ID for duty: %w", err)
	}
	duty.ID = id
	return nil
}

// GetDutyByDate retrieves a duty by its date, including user info.
func (s *SQLiteStore) GetDutyByDate(ctx context.Context, date time.Time) (*store.Duty, error) {
	query := `
		SELECT d.id, d.user_id, d.duty_date, d.assignment_type, d.created_at,
		       u.id, u.telegram_user_id, u.first_name, u.is_admin, u.is_active
		FROM duties d
		JOIN users u ON d.user_id = u.id
		WHERE d.duty_date = ?
	`
	row := s.db.QueryRowContext(ctx, query, date.Format("2006-01-02"))
	duty := &store.Duty{User: &store.User{}}
	var dutyDateStr, assignmentTypeStr, createdAtStr string

	err := row.Scan(
		&duty.ID, &duty.UserID, &dutyDateStr, &assignmentTypeStr, &createdAtStr,
		&duty.User.ID, &duty.User.TelegramUserID, &duty.User.FirstName, &duty.User.IsAdmin, &duty.User.IsActive,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found
		}
		return nil, fmt.Errorf("could not query duty by date: %w", err)
	}

	duty.DutyDate, err = time.Parse("2006-01-02", dutyDateStr)
	if err != nil {
		return nil, fmt.Errorf("could not parse duty date: %w", err)
	}
	duty.CreatedAt, err = time.Parse(time.RFC3339, createdAtStr)
	if err != nil {
		return nil, fmt.Errorf("could not parse created at: %w", err)
	}
	duty.AssignmentType = store.AssignmentType(assignmentTypeStr)

	return duty, nil
}

// UpdateDuty updates an existing duty.
func (s *SQLiteStore) UpdateDuty(ctx context.Context, duty *store.Duty) error {
	query := `UPDATE duties SET user_id = ?, assignment_type = ? WHERE duty_date = ?`
	_, err := s.db.ExecContext(ctx, query, duty.UserID, string(duty.AssignmentType), duty.DutyDate.Format("2006-01-02"))
	if err != nil {
		return fmt.Errorf("could not update duty: %w", err)
	}
	return nil
}

// DeleteDuty removes a duty assignment for a specific date.
func (s *SQLiteStore) DeleteDuty(ctx context.Context, date time.Time) error {
	query := `DELETE FROM duties WHERE duty_date = ?`
	_, err := s.db.ExecContext(ctx, query, date.Format("2006-01-02"))
	if err != nil {
		return fmt.Errorf("could not delete duty: %w", err)
	}
	return nil
}

// GetDutiesByMonth retrieves all duties for a given month and year.
func (s *SQLiteStore) GetDutiesByMonth(ctx context.Context, year int, month time.Month) ([]*store.Duty, error) {
	start := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)

	query := `
		SELECT d.id, d.user_id, d.duty_date, d.assignment_type, d.created_at,
		       u.id, u.telegram_user_id, u.first_name, u.is_admin, u.is_active
		FROM duties d
		JOIN users u ON d.user_id = u.id
		WHERE d.duty_date >= ? AND d.duty_date < ?
		ORDER BY d.duty_date
	`
	rows, err := s.db.QueryContext(ctx, query, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return nil, fmt.Errorf("could not query duties by month: %w", err)
	}
	defer rows.Close()

	var duties []*store.Duty
	for rows.Next() {
		duty := &store.Duty{User: &store.User{}}
		var dutyDateStr, assignmentTypeStr, createdAtStr string
		err := rows.Scan(
			&duty.ID, &duty.UserID, &dutyDateStr, &assignmentTypeStr, &createdAtStr,
			&duty.User.ID, &duty.User.TelegramUserID, &duty.User.FirstName, &duty.User.IsAdmin, &duty.User.IsActive,
		)
		if err != nil {
			return nil, fmt.Errorf("could not scan duty row: %w", err)
		}
		duty.DutyDate, err = time.Parse("2006-01-02", dutyDateStr)
		if err != nil {
			return nil, fmt.Errorf("could not parse duty date from month query: %w", err)
		}
		duty.CreatedAt, err = time.Parse(time.RFC3339, createdAtStr)
		if err != nil {
			return nil, fmt.Errorf("could not parse created at from month query: %w", err)
		}
		duty.AssignmentType = store.AssignmentType(assignmentTypeStr)
		duties = append(duties, duty)
	}
	return duties, nil
}


// GetNextRoundRobinUser finds the next user for a round-robin assignment.
func (s *SQLiteStore) GetNextRoundRobinUser(ctx context.Context) (*store.User, error) {
	query := `
		SELECT u.id, u.telegram_user_id, u.first_name, u.is_admin, u.is_active
		FROM users u
		LEFT JOIN round_robin_state rrs ON u.id = rrs.user_id
		WHERE u.is_active = 1
		ORDER BY rrs.assignment_count ASC, rrs.last_assigned_timestamp ASC
		LIMIT 1
	`
	row := s.db.QueryRowContext(ctx, query)
	user := &store.User{}
	err := row.Scan(&user.ID, &user.TelegramUserID, &user.FirstName, &user.IsAdmin, &user.IsActive)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No active users found
		}
		return nil, fmt.Errorf("could not get next round robin user: %w", err)
	}
	return user, nil
}


// IncrementAssignmentCount increments the assignment count for a user.
func (s *SQLiteStore) IncrementAssignmentCount(ctx context.Context, userID int64, lastAssigned time.Time) error {
	query := `
		INSERT INTO round_robin_state (user_id, assignment_count, last_assigned_timestamp)
		VALUES (?, 1, ?)
		ON CONFLICT(user_id) DO UPDATE SET
		assignment_count = assignment_count + 1,
		last_assigned_timestamp = excluded.last_assigned_timestamp;
	`
	_, err := s.db.ExecContext(ctx, query, userID, lastAssigned.UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("could not increment assignment count: %w", err)
	}
	return nil
}