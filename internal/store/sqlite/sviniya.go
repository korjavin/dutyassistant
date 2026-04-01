package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/korjavin/dutyassistant/internal/store"
)

// GetAllSviniyaBalances retrieves all sviniya balances with user names.
func (s *SQLiteStore) GetAllSviniyaBalances(ctx context.Context) ([]*store.SviniyaBalance, error) {
	query := `
		SELECT sb.user_id, u.first_name, sb.balance
		FROM sviniya_balances sb
		JOIN users u ON u.id = sb.user_id
		ORDER BY sb.balance DESC, u.first_name COLLATE NOCASE ASC
	`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("could not query sviniya balances: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var balances []*store.SviniyaBalance
	for rows.Next() {
		balance := &store.SviniyaBalance{}
		if err := rows.Scan(&balance.UserID, &balance.UserName, &balance.Balance); err != nil {
			return nil, fmt.Errorf("could not scan sviniya balance row: %w", err)
		}
		balances = append(balances, balance)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("could not iterate sviniya balances: %w", err)
	}

	return balances, nil
}

// GetSviniyaBalance retrieves a single user's sviniya balance by user ID.
func (s *SQLiteStore) GetSviniyaBalance(ctx context.Context, userID int64) (*store.SviniyaBalance, error) {
	query := `
		SELECT sb.user_id, u.first_name, sb.balance
		FROM sviniya_balances sb
		JOIN users u ON u.id = sb.user_id
		WHERE sb.user_id = ?
	`
	row := s.db.QueryRowContext(ctx, query, userID)
	balance := &store.SviniyaBalance{}
	err := row.Scan(&balance.UserID, &balance.UserName, &balance.Balance)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found is not an error
		}
		return nil, fmt.Errorf("could not query sviniya balance: %w", err)
	}

	return balance, nil
}

// AddSviniyaBalance adds amount to a user's sviniya balance (creates if doesn't exist).
func (s *SQLiteStore) AddSviniyaBalance(ctx context.Context, userID int64, amount int) error {
	query := `
		INSERT INTO sviniya_balances (user_id, balance)
		VALUES (?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			balance = balance + ?
	`
	_, err := s.db.ExecContext(ctx, query, userID, amount, amount)
	if err != nil {
		return fmt.Errorf("could not add sviniya balance: %w", err)
	}
	return nil
}

// SetSviniyaBalance sets a user's sviniya balance to a specific value (creates if doesn't exist).
func (s *SQLiteStore) SetSviniyaBalance(ctx context.Context, userID int64, balance int) error {
	query := `
		INSERT INTO sviniya_balances (user_id, balance)
		VALUES (?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			balance = ?
	`
	_, err := s.db.ExecContext(ctx, query, userID, balance, balance)
	if err != nil {
		return fmt.Errorf("could not set sviniya balance: %w", err)
	}
	return nil
}

// DecrementSviniyaBalance decrements a user's sviniya balance by 1.
// Returns an error if the user has no balance record or if balance is already 0.
func (s *SQLiteStore) DecrementSviniyaBalance(ctx context.Context, userID int64) error {
	query := `UPDATE sviniya_balances SET balance = balance - 1 WHERE user_id = ? AND balance > 0`
	result, err := s.db.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("could not decrement sviniya balance: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("could not get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		// Check if user has a balance record
		balance, err := s.GetSviniyaBalance(ctx, userID)
		if err != nil {
			return fmt.Errorf("could not check sviniya balance: %w", err)
		}
		if balance == nil {
			return fmt.Errorf("no sviniya balance record found for user %d", userID)
		}
		if balance.Balance <= 0 {
			return fmt.Errorf("insufficient sviniya balance for user %d", userID)
		}
		return fmt.Errorf("failed to decrement sviniya balance for user %d", userID)
	}
	return nil
}

// GetSviniyaMonthlyGrant checks if a sviniya has already been granted for a given month/year.
// Returns the user ID who received the grant and whether a grant exists.
func (s *SQLiteStore) GetSviniyaMonthlyGrant(ctx context.Context, year int, month time.Month) (userID int64, granted bool, err error) {
	query := `SELECT user_id FROM sviniya_monthly_grants WHERE year = ? AND month = ?`
	row := s.db.QueryRowContext(ctx, query, year, int(month))
	err = row.Scan(&userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("could not query sviniya monthly grant: %w", err)
	}
	return userID, true, nil
}

// RecordSviniyaMonthlyGrant records that a sviniya was granted to a user for a specific month/year.
// This prevents duplicate grants if the function is called multiple times.
func (s *SQLiteStore) RecordSviniyaMonthlyGrant(ctx context.Context, year int, month time.Month, userID int64) error {
	query := `
		INSERT INTO sviniya_monthly_grants (year, month, user_id, granted_at)
		VALUES (?, ?, ?, ?)
	`
	_, err := s.db.ExecContext(ctx, query, year, int(month), userID, time.Now().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("could not record sviniya monthly grant: %w", err)
	}
	return nil
}

// GrantSviniyaForMonth atomically grants a sviniya to a user for a specific month/year.
// It records the grant and adds the balance in a single transaction to prevent either operation from succeeding without the other.
// Returns an error if a grant already exists for this month/year.
func (s *SQLiteStore) GrantSviniyaForMonth(ctx context.Context, year int, month time.Month, userID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("could not begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// Check if grant already exists
	var existingUserID int64
	checkQuery := `SELECT user_id FROM sviniya_monthly_grants WHERE year = ? AND month = ?`
	err = tx.QueryRowContext(ctx, checkQuery, year, int(month)).Scan(&existingUserID)
	if err == nil {
		// Grant already exists
		tx.Rollback()
		return fmt.Errorf("sviniya already granted for %s %d to user %d", month, year, existingUserID)
	}
	if err != sql.ErrNoRows {
		return fmt.Errorf("could not check existing grant: %w", err)
	}

	// Record the grant
	grantQuery := `
		INSERT INTO sviniya_monthly_grants (year, month, user_id, granted_at)
		VALUES (?, ?, ?, ?)
	`
	_, err = tx.ExecContext(ctx, grantQuery, year, int(month), userID, time.Now().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("could not record sviniya monthly grant: %w", err)
	}

	// Add the balance
	balanceQuery := `
		INSERT INTO sviniya_balances (user_id, balance)
		VALUES (?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			balance = balance + ?
	`
	_, err = tx.ExecContext(ctx, balanceQuery, userID, 1, 1)
	if err != nil {
		return fmt.Errorf("could not add sviniya balance: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("could not commit transaction: %w", err)
	}
	return nil
}
