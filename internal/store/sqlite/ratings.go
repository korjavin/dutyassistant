package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/korjavin/dutyassistant/internal/store"
)

const sqliteDateLayout = "2006-01-02"

// SaveDailyParticipantRatings upserts one day's ratings for the provided participants.
func (s *SQLiteStore) SaveDailyParticipantRatings(ctx context.Context, date time.Time, ratings []*store.ParticipantDailyRating) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("could not start participant ratings transaction: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().UTC().Format(time.RFC3339)
	dateStr := date.UTC().Format(sqliteDateLayout)

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO participant_ratings (participant_id, rating_date, score, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(participant_id, rating_date) DO UPDATE SET
			score = excluded.score,
			updated_at = excluded.updated_at
	`)
	if err != nil {
		return fmt.Errorf("could not prepare participant ratings statement: %w", err)
	}
	defer stmt.Close()

	for _, rating := range ratings {
		if rating == nil {
			return fmt.Errorf("participant rating must not be nil")
		}
		if rating.ParticipantID == 0 {
			return fmt.Errorf("participant rating must include participant id")
		}
		if rating.Score < 1 || rating.Score > 5 {
			return fmt.Errorf("participant rating score must be between 1 and 5")
		}

		if _, err := stmt.ExecContext(ctx, rating.ParticipantID, dateStr, rating.Score, now, now); err != nil {
			return fmt.Errorf("could not save participant rating for participant %d: %w", rating.ParticipantID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("could not commit participant ratings transaction: %w", err)
	}
	return nil
}

// GetParticipantsForRating returns active non-admin participants in a deterministic order.
func (s *SQLiteStore) GetParticipantsForRating(ctx context.Context) ([]*store.User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, telegram_user_id, first_name, is_admin, is_active,
		       volunteer_queue_days, admin_queue_days, off_duty_start, off_duty_end
		FROM users
		WHERE is_active = 1 AND is_admin = 0
		ORDER BY first_name COLLATE NOCASE ASC, id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("could not query participants for rating: %w", err)
	}
	defer rows.Close()

	var users []*store.User
	for rows.Next() {
		user, err := scanUserRows(rows)
		if err != nil {
			return nil, fmt.Errorf("could not scan participant for rating: %w", err)
		}
		users = append(users, user)
	}

	return users, nil
}

// GetCurrentMonthParticipantRatings returns all participant ratings for the month containing now.
func (s *SQLiteStore) GetCurrentMonthParticipantRatings(ctx context.Context, now time.Time) ([]*store.ParticipantDailyRating, error) {
	start, end := monthBounds(now)
	return s.getParticipantRatingsBetween(ctx, start, end)
}

// GetMonthlyParticipantTotals returns ranked monthly totals with deterministic tie ordering.
func (s *SQLiteStore) GetMonthlyParticipantTotals(ctx context.Context, year int, month time.Month) ([]*store.ParticipantMonthlyTotal, error) {
	start := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)

	rows, err := s.db.QueryContext(ctx, `
		SELECT pr.participant_id, u.first_name, SUM(pr.score) AS total_score, COUNT(*) AS days_rated
		FROM participant_ratings pr
		JOIN users u ON u.id = pr.participant_id
		WHERE pr.rating_date >= ? AND pr.rating_date < ?
		GROUP BY pr.participant_id, u.first_name
		ORDER BY total_score DESC, u.first_name COLLATE NOCASE ASC, pr.participant_id ASC
	`, start.Format(sqliteDateLayout), end.Format(sqliteDateLayout))
	if err != nil {
		return nil, fmt.Errorf("could not query monthly participant totals: %w", err)
	}
	defer rows.Close()

	var totals []*store.ParticipantMonthlyTotal
	for rows.Next() {
		total := &store.ParticipantMonthlyTotal{}
		if err := rows.Scan(&total.ParticipantID, &total.ParticipantName, &total.TotalScore, &total.DaysRated); err != nil {
			return nil, fmt.Errorf("could not scan monthly participant total: %w", err)
		}
		totals = append(totals, total)
	}

	return totals, nil
}

func (s *SQLiteStore) getParticipantRatingsBetween(ctx context.Context, start time.Time, end time.Time) ([]*store.ParticipantDailyRating, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT pr.participant_id, u.first_name, pr.rating_date, pr.score
		FROM participant_ratings pr
		JOIN users u ON u.id = pr.participant_id
		WHERE pr.rating_date >= ? AND pr.rating_date < ?
		ORDER BY pr.rating_date ASC, u.first_name COLLATE NOCASE ASC, pr.participant_id ASC
	`, start.Format(sqliteDateLayout), end.Format(sqliteDateLayout))
	if err != nil {
		return nil, fmt.Errorf("could not query participant ratings: %w", err)
	}
	defer rows.Close()

	var ratings []*store.ParticipantDailyRating
	for rows.Next() {
		rating, err := scanParticipantDailyRating(rows)
		if err != nil {
			return nil, err
		}
		ratings = append(ratings, rating)
	}

	return ratings, nil
}

func scanParticipantDailyRating(rows *sql.Rows) (*store.ParticipantDailyRating, error) {
	rating := &store.ParticipantDailyRating{}
	var dateStr string
	if err := rows.Scan(&rating.ParticipantID, &rating.ParticipantName, &dateStr, &rating.Score); err != nil {
		return nil, fmt.Errorf("could not scan participant rating row: %w", err)
	}

	parsedDate, err := time.Parse(sqliteDateLayout, dateStr)
	if err != nil {
		return nil, fmt.Errorf("could not parse participant rating date: %w", err)
	}
	rating.RatingDate = parsedDate

	return rating, nil
}

func monthBounds(now time.Time) (time.Time, time.Time) {
	start := time.Date(now.UTC().Year(), now.UTC().Month(), 1, 0, 0, 0, 0, time.UTC)
	return start, start.AddDate(0, 1, 0)
}
