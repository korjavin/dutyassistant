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
			is_active INTEGER NOT NULL DEFAULT 1,
			volunteer_queue_days INTEGER NOT NULL DEFAULT 0,
			admin_queue_days INTEGER NOT NULL DEFAULT 0,
			off_duty_start TEXT,
			off_duty_end TEXT
		);

		CREATE TABLE IF NOT EXISTS duties (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			duty_date TEXT UNIQUE NOT NULL,
			assignment_type TEXT NOT NULL,
			created_at TEXT NOT NULL,
			completed_at TEXT,
			FOREIGN KEY(user_id) REFERENCES users(id)
		);
	`
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return err
	}

	// Add new columns to existing tables if they don't exist
	alterations := []string{
		`ALTER TABLE users ADD COLUMN volunteer_queue_days INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE users ADD COLUMN admin_queue_days INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE users ADD COLUMN off_duty_start TEXT`,
		`ALTER TABLE users ADD COLUMN off_duty_end TEXT`,
		`ALTER TABLE duties ADD COLUMN completed_at TEXT`,
	}

	for _, alteration := range alterations {
		// Ignore errors for columns that already exist
		s.db.ExecContext(ctx, alteration)
	}

	return nil
}

// scanUser is a helper to scan a user row with all fields including new ones
func scanUser(row *sql.Row) (*store.User, error) {
	user := &store.User{}
	var offDutyStart, offDutyEnd sql.NullString
	err := row.Scan(&user.ID, &user.TelegramUserID, &user.FirstName, &user.IsAdmin, &user.IsActive,
		&user.VolunteerQueueDays, &user.AdminQueueDays, &offDutyStart, &offDutyEnd)
	if err != nil {
		return nil, err
	}

	if offDutyStart.Valid {
		t, _ := time.Parse("2006-01-02", offDutyStart.String)
		user.OffDutyStart = &t
	}
	if offDutyEnd.Valid {
		t, _ := time.Parse("2006-01-02", offDutyEnd.String)
		user.OffDutyEnd = &t
	}

	return user, nil
}

// scanUserRows is a helper to scan multiple user rows
func scanUserRows(rows *sql.Rows) (*store.User, error) {
	user := &store.User{}
	var offDutyStart, offDutyEnd sql.NullString
	err := rows.Scan(&user.ID, &user.TelegramUserID, &user.FirstName, &user.IsAdmin, &user.IsActive,
		&user.VolunteerQueueDays, &user.AdminQueueDays, &offDutyStart, &offDutyEnd)
	if err != nil {
		return nil, err
	}

	if offDutyStart.Valid {
		t, _ := time.Parse("2006-01-02", offDutyStart.String)
		user.OffDutyStart = &t
	}
	if offDutyEnd.Valid {
		t, _ := time.Parse("2006-01-02", offDutyEnd.String)
		user.OffDutyEnd = &t
	}

	return user, nil
}

// CreateUser adds a new user to the database.
func (s *SQLiteStore) CreateUser(ctx context.Context, user *store.User) error {
	query := `INSERT INTO users (telegram_user_id, first_name, is_admin, is_active, volunteer_queue_days, admin_queue_days, off_duty_start, off_duty_end)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	var offDutyStart, offDutyEnd interface{}
	if user.OffDutyStart != nil {
		offDutyStart = user.OffDutyStart.Format("2006-01-02")
	}
	if user.OffDutyEnd != nil {
		offDutyEnd = user.OffDutyEnd.Format("2006-01-02")
	}

	res, err := s.db.ExecContext(ctx, query, user.TelegramUserID, user.FirstName, user.IsAdmin, user.IsActive,
		user.VolunteerQueueDays, user.AdminQueueDays, offDutyStart, offDutyEnd)
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
	query := `SELECT id, telegram_user_id, first_name, is_admin, is_active, volunteer_queue_days, admin_queue_days, off_duty_start, off_duty_end
	          FROM users WHERE telegram_user_id = ?`
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

// ListActiveUsers retrieves all users who are currently active.
func (s *SQLiteStore) ListActiveUsers(ctx context.Context) ([]*store.User, error) {
	query := `SELECT id, telegram_user_id, first_name, is_admin, is_active, volunteer_queue_days, admin_queue_days, off_duty_start, off_duty_end
	          FROM users WHERE is_active = 1`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("could not query active users: %w", err)
	}
	defer rows.Close()

	var users []*store.User
	for rows.Next() {
		user, err := scanUserRows(rows)
		if err != nil {
			return nil, fmt.Errorf("could not scan user row: %w", err)
		}
		users = append(users, user)
	}
	return users, nil
}

// GetUserByName retrieves a user by their first name.
func (s *SQLiteStore) GetUserByName(ctx context.Context, name string) (*store.User, error) {
	query := `SELECT id, telegram_user_id, first_name, is_admin, is_active, volunteer_queue_days, admin_queue_days, off_duty_start, off_duty_end
	          FROM users WHERE first_name = ?`
	row := s.db.QueryRowContext(ctx, query, name)
	user, err := scanUser(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found is not an error
		}
		return nil, fmt.Errorf("could not query user by name: %w", err)
	}
	return user, nil
}

// ListAllUsers retrieves all users (both active and inactive).
func (s *SQLiteStore) ListAllUsers(ctx context.Context) ([]*store.User, error) {
	query := `SELECT id, telegram_user_id, first_name, is_admin, is_active, volunteer_queue_days, admin_queue_days, off_duty_start, off_duty_end
	          FROM users ORDER BY first_name`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("could not query all users: %w", err)
	}
	defer rows.Close()

	var users []*store.User
	for rows.Next() {
		user, err := scanUserRows(rows)
		if err != nil {
			return nil, fmt.Errorf("could not scan user row: %w", err)
		}
		users = append(users, user)
	}
	return users, nil
}

// GetUserStats retrieves aggregated statistics for a user.
func (s *SQLiteStore) GetUserStats(ctx context.Context, userID int64) (*store.UserStats, error) {
	stats := &store.UserStats{}

	// Get total duties
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM duties WHERE user_id = ?`, userID).Scan(&stats.TotalDuties)
	if err != nil {
		return nil, fmt.Errorf("could not count total duties: %w", err)
	}

	// Get duties this month
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM duties WHERE user_id = ? AND duty_date >= ? AND duty_date < ?`,
		userID, start.Format("2006-01-02"), end.Format("2006-01-02")).Scan(&stats.DutiesThisMonth)
	if err != nil {
		return nil, fmt.Errorf("could not count duties this month: %w", err)
	}

	// Get next duty date
	var nextDate string
	err = s.db.QueryRowContext(ctx,
		`SELECT duty_date FROM duties WHERE user_id = ? AND duty_date >= ? ORDER BY duty_date LIMIT 1`,
		userID, time.Now().Format("2006-01-02")).Scan(&nextDate)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("could not get next duty date: %w", err)
	}
	stats.NextDutyDate = nextDate

	return stats, nil
}

// UpdateUser updates a user's details.
func (s *SQLiteStore) UpdateUser(ctx context.Context, user *store.User) error {
	query := `UPDATE users SET first_name = ?, is_admin = ?, is_active = ?, volunteer_queue_days = ?, admin_queue_days = ?, off_duty_start = ?, off_duty_end = ? WHERE id = ?`

	var offDutyStart, offDutyEnd interface{}
	if user.OffDutyStart != nil {
		offDutyStart = user.OffDutyStart.Format("2006-01-02")
	}
	if user.OffDutyEnd != nil {
		offDutyEnd = user.OffDutyEnd.Format("2006-01-02")
	}

	_, err := s.db.ExecContext(ctx, query, user.FirstName, user.IsAdmin, user.IsActive,
		user.VolunteerQueueDays, user.AdminQueueDays, offDutyStart, offDutyEnd, user.ID)
	if err != nil {
		return fmt.Errorf("could not update user: %w", err)
	}
	return nil
}

// CreateDuty creates a new duty assignment.
func (s *SQLiteStore) CreateDuty(ctx context.Context, duty *store.Duty) error {
	query := `INSERT INTO duties (user_id, duty_date, assignment_type, created_at, completed_at) VALUES (?, ?, ?, ?, ?)`

	var completedAt interface{}
	if duty.CompletedAt != nil {
		completedAt = duty.CompletedAt.UTC().Format(time.RFC3339)
	}

	res, err := s.db.ExecContext(ctx, query, duty.UserID, duty.DutyDate.Format("2006-01-02"), string(duty.AssignmentType), duty.CreatedAt.UTC().Format(time.RFC3339), completedAt)
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
		SELECT d.id, d.user_id, d.duty_date, d.assignment_type, d.created_at, d.completed_at,
		       u.id, u.telegram_user_id, u.first_name, u.is_admin, u.is_active
		FROM duties d
		JOIN users u ON d.user_id = u.id
		WHERE d.duty_date = ?
	`
	row := s.db.QueryRowContext(ctx, query, date.Format("2006-01-02"))
	duty := &store.Duty{User: &store.User{}}
	var dutyDateStr, assignmentTypeStr, createdAtStr string
	var completedAtStr sql.NullString

	err := row.Scan(
		&duty.ID, &duty.UserID, &dutyDateStr, &assignmentTypeStr, &createdAtStr, &completedAtStr,
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
	if completedAtStr.Valid {
		t, err := time.Parse(time.RFC3339, completedAtStr.String)
		if err != nil {
			return nil, fmt.Errorf("could not parse completed at: %w", err)
		}
		duty.CompletedAt = &t
	}
	duty.AssignmentType = store.AssignmentType(assignmentTypeStr)

	return duty, nil
}

// UpdateDuty updates an existing duty.
func (s *SQLiteStore) UpdateDuty(ctx context.Context, duty *store.Duty) error {
	query := `UPDATE duties SET user_id = ?, assignment_type = ?, completed_at = ? WHERE duty_date = ?`

	var completedAt interface{}
	if duty.CompletedAt != nil {
		completedAt = duty.CompletedAt.UTC().Format(time.RFC3339)
	}

	_, err := s.db.ExecContext(ctx, query, duty.UserID, string(duty.AssignmentType), completedAt, duty.DutyDate.Format("2006-01-02"))
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
		SELECT d.id, d.user_id, d.duty_date, d.assignment_type, d.created_at, d.completed_at,
		       u.id, u.telegram_user_id, u.first_name, u.is_admin, u.is_active,
		       u.volunteer_queue_days, u.admin_queue_days, u.off_duty_start, u.off_duty_end
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
		var completedAtStr, offDutyStart, offDutyEnd sql.NullString
		err := rows.Scan(
			&duty.ID, &duty.UserID, &dutyDateStr, &assignmentTypeStr, &createdAtStr, &completedAtStr,
			&duty.User.ID, &duty.User.TelegramUserID, &duty.User.FirstName, &duty.User.IsAdmin, &duty.User.IsActive,
			&duty.User.VolunteerQueueDays, &duty.User.AdminQueueDays, &offDutyStart, &offDutyEnd,
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
		if completedAtStr.Valid {
			t, err := time.Parse(time.RFC3339, completedAtStr.String)
			if err != nil {
				return nil, fmt.Errorf("could not parse completed at from month query: %w", err)
			}
			duty.CompletedAt = &t
		}
		if offDutyStart.Valid {
			t, _ := time.Parse("2006-01-02", offDutyStart.String)
			duty.User.OffDutyStart = &t
		}
		if offDutyEnd.Valid {
			t, _ := time.Parse("2006-01-02", offDutyEnd.String)
			duty.User.OffDutyEnd = &t
		}
		duty.AssignmentType = store.AssignmentType(assignmentTypeStr)
		duties = append(duties, duty)
	}
	return duties, nil
}

// AddToVolunteerQueue adds days to a user's volunteer queue.
func (s *SQLiteStore) AddToVolunteerQueue(ctx context.Context, userID int64, days int) error {
	query := `UPDATE users SET volunteer_queue_days = volunteer_queue_days + ? WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, days, userID)
	if err != nil {
		return fmt.Errorf("could not add to volunteer queue: %w", err)
	}
	return nil
}

// AddToAdminQueue adds days to a user's admin assignment queue.
func (s *SQLiteStore) AddToAdminQueue(ctx context.Context, userID int64, days int) error {
	query := `UPDATE users SET admin_queue_days = admin_queue_days + ? WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, days, userID)
	if err != nil {
		return fmt.Errorf("could not add to admin queue: %w", err)
	}
	return nil
}

// DecrementVolunteerQueue decrements a user's volunteer queue by 1 (minimum 0).
func (s *SQLiteStore) DecrementVolunteerQueue(ctx context.Context, userID int64) error {
	query := `UPDATE users SET volunteer_queue_days = MAX(0, volunteer_queue_days - 1) WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("could not decrement volunteer queue: %w", err)
	}
	return nil
}

// DecrementAdminQueue decrements a user's admin queue by 1 (minimum 0).
func (s *SQLiteStore) DecrementAdminQueue(ctx context.Context, userID int64) error {
	query := `UPDATE users SET admin_queue_days = MAX(0, admin_queue_days - 1) WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("could not decrement admin queue: %w", err)
	}
	return nil
}

// GetUsersWithVolunteerQueue returns all active users with volunteer queue > 0.
func (s *SQLiteStore) GetUsersWithVolunteerQueue(ctx context.Context) ([]*store.User, error) {
	query := `
		SELECT id, telegram_user_id, first_name, is_admin, is_active,
		       volunteer_queue_days, admin_queue_days, off_duty_start, off_duty_end
		FROM users
		WHERE is_active = 1 AND volunteer_queue_days > 0
		ORDER BY volunteer_queue_days DESC
	`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("could not query users with volunteer queue: %w", err)
	}
	defer rows.Close()

	var users []*store.User
	for rows.Next() {
		user, err := scanUserRows(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}

// GetUsersWithAdminQueue returns all active users with admin queue > 0.
func (s *SQLiteStore) GetUsersWithAdminQueue(ctx context.Context) ([]*store.User, error) {
	query := `
		SELECT id, telegram_user_id, first_name, is_admin, is_active,
		       volunteer_queue_days, admin_queue_days, off_duty_start, off_duty_end
		FROM users
		WHERE is_active = 1 AND admin_queue_days > 0
		ORDER BY admin_queue_days DESC
	`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("could not query users with admin queue: %w", err)
	}
	defer rows.Close()

	var users []*store.User
	for rows.Next() {
		user, err := scanUserRows(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}

// SetOffDuty sets a user's off-duty period.
func (s *SQLiteStore) SetOffDuty(ctx context.Context, userID int64, start, end time.Time) error {
	query := `UPDATE users SET off_duty_start = ?, off_duty_end = ? WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, start.Format("2006-01-02"), end.Format("2006-01-02"), userID)
	if err != nil {
		return fmt.Errorf("could not set off-duty: %w", err)
	}
	return nil
}

// ClearOffDuty clears a user's off-duty period.
func (s *SQLiteStore) ClearOffDuty(ctx context.Context, userID int64) error {
	query := `UPDATE users SET off_duty_start = NULL, off_duty_end = NULL WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("could not clear off-duty: %w", err)
	}
	return nil
}

// IsUserOffDuty checks if a user is off-duty on a specific date.
func (s *SQLiteStore) IsUserOffDuty(ctx context.Context, userID int64, date time.Time) (bool, error) {
	query := `
		SELECT COUNT(*) FROM users
		WHERE id = ? AND off_duty_start IS NOT NULL AND off_duty_end IS NOT NULL
		AND ? >= off_duty_start AND ? <= off_duty_end
	`
	dateStr := date.Format("2006-01-02")
	var count int
	err := s.db.QueryRowContext(ctx, query, userID, dateStr, dateStr).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("could not check off-duty status: %w", err)
	}
	return count > 0, nil
}

// GetOffDutyUsers returns all users who are off-duty on a specific date.
func (s *SQLiteStore) GetOffDutyUsers(ctx context.Context, date time.Time) ([]*store.User, error) {
	query := `
		SELECT id, telegram_user_id, first_name, is_admin, is_active,
		       volunteer_queue_days, admin_queue_days, off_duty_start, off_duty_end
		FROM users
		WHERE off_duty_start IS NOT NULL AND off_duty_end IS NOT NULL
		AND ? >= off_duty_start AND ? <= off_duty_end
	`
	dateStr := date.Format("2006-01-02")
	rows, err := s.db.QueryContext(ctx, query, dateStr, dateStr)
	if err != nil {
		return nil, fmt.Errorf("could not query off-duty users: %w", err)
	}
	defer rows.Close()

	var users []*store.User
	for rows.Next() {
		user, err := scanUserRows(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}

// CompleteDuty marks a duty as completed by setting completed_at timestamp.
func (s *SQLiteStore) CompleteDuty(ctx context.Context, date time.Time) error {
	query := `UPDATE duties SET completed_at = ? WHERE duty_date = ?`
	_, err := s.db.ExecContext(ctx, query, time.Now().UTC().Format(time.RFC3339), date.Format("2006-01-02"))
	if err != nil {
		return fmt.Errorf("could not complete duty: %w", err)
	}
	return nil
}

// GetTodaysDuty retrieves today's duty assignment.
func (s *SQLiteStore) GetTodaysDuty(ctx context.Context) (*store.Duty, error) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return s.GetDutyByDate(ctx, today)
}

// GetCompletedDutiesInRange retrieves all completed duties in a date range.
func (s *SQLiteStore) GetCompletedDutiesInRange(ctx context.Context, start, end time.Time) ([]*store.Duty, error) {
	query := `
		SELECT d.id, d.user_id, d.duty_date, d.assignment_type, d.created_at, d.completed_at,
		       u.id, u.telegram_user_id, u.first_name, u.is_admin, u.is_active
		FROM duties d
		JOIN users u ON d.user_id = u.id
		WHERE d.duty_date >= ? AND d.duty_date < ? AND d.completed_at IS NOT NULL
		ORDER BY d.duty_date
	`
	rows, err := s.db.QueryContext(ctx, query, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if err != nil {
		return nil, fmt.Errorf("could not query completed duties: %w", err)
	}
	defer rows.Close()

	var duties []*store.Duty
	for rows.Next() {
		duty := &store.Duty{User: &store.User{}}
		var dutyDateStr, assignmentTypeStr, createdAtStr, completedAtStr string
		err := rows.Scan(
			&duty.ID, &duty.UserID, &dutyDateStr, &assignmentTypeStr, &createdAtStr, &completedAtStr,
			&duty.User.ID, &duty.User.TelegramUserID, &duty.User.FirstName, &duty.User.IsAdmin, &duty.User.IsActive,
		)
		if err != nil {
			return nil, fmt.Errorf("could not scan completed duty row: %w", err)
		}
		duty.DutyDate, err = time.Parse("2006-01-02", dutyDateStr)
		if err != nil {
			return nil, fmt.Errorf("could not parse duty date: %w", err)
		}
		duty.CreatedAt, err = time.Parse(time.RFC3339, createdAtStr)
		if err != nil {
			return nil, fmt.Errorf("could not parse created at: %w", err)
		}
		t, err := time.Parse(time.RFC3339, completedAtStr)
		if err != nil {
			return nil, fmt.Errorf("could not parse completed at: %w", err)
		}
		duty.CompletedAt = &t
		duty.AssignmentType = store.AssignmentType(assignmentTypeStr)
		duties = append(duties, duty)
	}
	return duties, nil
}