package sqlite

import (
	"context"
	"database/sql"
	"fmt"

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

// DecrementSviniyaBalance decrements a user's sviniya balance by 1 (minimum 0).
func (s *SQLiteStore) DecrementSviniyaBalance(ctx context.Context, userID int64) error {
	query := `UPDATE sviniya_balances SET balance = MAX(0, balance - 1) WHERE user_id = ?`
	_, err := s.db.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("could not decrement sviniya balance: %w", err)
	}
	return nil
}
