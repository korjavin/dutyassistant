package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/korjavin/dutyassistant/internal/store"
)

const dailyReportColumns = `
	r.id, r.user_id, u.first_name, u.telegram_user_id, r.report_date, r.text, r.parts,
	r.status, r.proposed_score, r.analysis
	FROM daily_reports r
	JOIN users u ON u.id = r.user_id`

// SaveParticipantRating upserts one participant's score for one day, leaving
// the other participants' scores for that day untouched.
func (s *SQLiteStore) SaveParticipantRating(ctx context.Context, rating *store.ParticipantDailyRating) error {
	if rating == nil || rating.ParticipantID == 0 {
		return fmt.Errorf("participant rating must include participant id")
	}
	if rating.Score < 1 || rating.Score > 5 {
		return fmt.Errorf("participant rating score must be between 1 and 5")
	}
	if rating.HasEar && rating.Score != 5 {
		return fmt.Errorf("ear modifier can only be used with score 5, got score %d", rating.Score)
	}
	hasEar := 0
	if rating.HasEar {
		hasEar = 1
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO participant_ratings (participant_id, rating_date, score, has_ear, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(participant_id, rating_date) DO UPDATE SET
			score = excluded.score,
			has_ear = excluded.has_ear,
			updated_at = excluded.updated_at
	`, rating.ParticipantID, rating.RatingDate.UTC().Format(sqliteDateLayout), rating.Score, hasEar, now, now)
	if err != nil {
		return fmt.Errorf("could not save participant rating for participant %d: %w", rating.ParticipantID, err)
	}
	return nil
}

// EnsureDailyReport returns the user's report for date, creating an empty
// collecting one if there is none yet.
func (s *SQLiteStore) EnsureDailyReport(ctx context.Context, userID int64, date time.Time) (*store.DailyReport, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO daily_reports (user_id, report_date, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(user_id, report_date) DO NOTHING
	`, userID, date.UTC().Format(sqliteDateLayout), store.DailyReportCollecting, now, now)
	if err != nil {
		return nil, fmt.Errorf("could not create daily report: %w", err)
	}
	report, err := s.GetDailyReport(ctx, userID, date)
	if err != nil {
		return nil, err
	}
	if report == nil {
		return nil, fmt.Errorf("daily report for user %d not found after insert", userID)
	}
	return report, nil
}

// GetDailyReport returns the user's report for date, or nil if none.
func (s *SQLiteStore) GetDailyReport(ctx context.Context, userID int64, date time.Time) (*store.DailyReport, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+dailyReportColumns+`
		WHERE r.user_id = ? AND r.report_date = ?`, userID, date.UTC().Format(sqliteDateLayout))
	return scanDailyReport(row)
}

// GetDailyReportByID returns a report by ID, or nil if none.
func (s *SQLiteStore) GetDailyReportByID(ctx context.Context, id int64) (*store.DailyReport, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+dailyReportColumns+` WHERE r.id = ?`, id)
	return scanDailyReport(row)
}

// ListDailyReports returns all reports for date, ordered by participant name.
func (s *SQLiteStore) ListDailyReports(ctx context.Context, date time.Time) ([]*store.DailyReport, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+dailyReportColumns+`
		WHERE r.report_date = ?
		ORDER BY u.first_name COLLATE NOCASE ASC, r.id ASC`, date.UTC().Format(sqliteDateLayout))
	if err != nil {
		return nil, fmt.Errorf("could not query daily reports: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var reports []*store.DailyReport
	for rows.Next() {
		report, err := scanDailyReport(rows)
		if err != nil {
			return nil, err
		}
		reports = append(reports, report)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("could not iterate daily reports: %w", err)
	}
	return reports, nil
}

// AppendDailyReport adds a part to a report that is still collecting.
// It returns false when the report is not collecting (or does not exist).
func (s *SQLiteStore) AppendDailyReport(ctx context.Context, id int64, text string) (bool, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE daily_reports SET
			text = CASE WHEN text = '' THEN ? ELSE text || char(10) || ? END,
			parts = parts + 1,
			updated_at = ?
		WHERE id = ? AND status = ?
	`, text, text, time.Now().UTC().Format(time.RFC3339), id, store.DailyReportCollecting)
	if err != nil {
		return false, fmt.Errorf("could not append to daily report: %w", err)
	}
	return rowsChanged(res)
}

// SetDailyReportStatus moves a report from status from to status to, so that
// concurrent button presses act only once. It returns false if the report was
// not in status from.
func (s *SQLiteStore) SetDailyReportStatus(ctx context.Context, id int64, from, to string) (bool, error) {
	res, err := s.db.ExecContext(ctx, `UPDATE daily_reports SET status = ?, updated_at = ? WHERE id = ? AND status = ?`,
		to, time.Now().UTC().Format(time.RFC3339), id, from)
	if err != nil {
		return false, fmt.Errorf("could not update daily report status: %w", err)
	}
	return rowsChanged(res)
}

// SaveDailyReportProposal stores the LLM's proposed score and analysis and
// marks the report as proposed.
func (s *SQLiteStore) SaveDailyReportProposal(ctx context.Context, id int64, score int, analysis string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE daily_reports SET proposed_score = ?, analysis = ?, status = ?, updated_at = ? WHERE id = ?
	`, score, analysis, store.DailyReportProposed, time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return fmt.Errorf("could not save daily report proposal: %w", err)
	}
	return nil
}

// GetChoresCompletedBetween returns the user's chores completed in [start, end).
func (s *SQLiteStore) GetChoresCompletedBetween(ctx context.Context, userID int64, start, end time.Time) ([]*store.Chore, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.user_id, c.description, c.assigned_at, c.deadline_at, c.completed_at, c.cancelled_at, c.reminder_id,
		       u.id, u.telegram_user_id, u.first_name, u.is_admin, u.is_active
		FROM chores c
		JOIN users u ON c.user_id = u.id
		WHERE c.user_id = ? AND c.completed_at >= ? AND c.completed_at < ?
		ORDER BY c.completed_at ASC
	`, userID, start.UTC().Format(time.RFC3339), end.UTC().Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("could not query completed chores for user: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanChoreRowsWithUser(rows)
}

func scanDailyReport(row rowScanner) (*store.DailyReport, error) {
	report := &store.DailyReport{}
	var dateStr string
	err := row.Scan(&report.ID, &report.UserID, &report.UserName, &report.TelegramUserID, &dateStr,
		&report.Text, &report.Parts, &report.Status, &report.ProposedScore, &report.Analysis)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("could not scan daily report: %w", err)
	}
	date, err := time.Parse(sqliteDateLayout, dateStr)
	if err != nil {
		return nil, fmt.Errorf("could not parse daily report date: %w", err)
	}
	report.ReportDate = date
	return report, nil
}

func rowsChanged(res sql.Result) (bool, error) {
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
