package domain

import (
	"context"
	"time"
)

// Repository defines the interface for all data access needs.
type Repository interface {
	// User methods
	GetUserByTelegramID(ctx context.Context, id int64) (*User, error)
	GetUserByName(ctx context.Context, name string) (*User, error)
	ListActiveUsers(ctx context.Context) ([]*User, error)
	ListAllUsers(ctx context.Context) ([]*User, error)
	CreateUser(ctx context.Context, user *User) error
	UpdateUser(ctx context.Context, user *User) error
	GetUserStats(ctx context.Context, userID int64) (*UserStats, error)

	// Duty methods
	CreateDuty(ctx context.Context, duty *Duty) error
	GetDutyByDate(ctx context.Context, date time.Time) (*Duty, error)
	UpdateDuty(ctx context.Context, duty *Duty) error
	DeleteDuty(ctx context.Context, date time.Time) error
	GetDutiesByMonth(ctx context.Context, year int, month time.Month) ([]*Duty, error)
	CompleteDuty(ctx context.Context, date time.Time) error
	GetTodaysDuty(ctx context.Context) (*Duty, error)
	GetCompletedDutiesInRange(ctx context.Context, start, end time.Time) ([]*Duty, error)
	GetLastDuty(ctx context.Context) (*Duty, error)

	// Chore methods
	CreateChore(ctx context.Context, chore *Chore) error
	GetChoreByReminderID(ctx context.Context, reminderID string) (*Chore, error)
	GetActiveChores(ctx context.Context) ([]*Chore, error)
	GetActiveChoresByUserID(ctx context.Context, userID int64) ([]*Chore, error)
	GetOverdueChores(ctx context.Context) ([]*Chore, error)
	CompleteChoreByReminderID(ctx context.Context, reminderID string) error
	CancelChore(ctx context.Context, id int64) (*Chore, error)
	ListActiveChores(ctx context.Context) ([]*Chore, error)
	GetTopOverdueChores(ctx context.Context, limit int) ([]*ChoreStat, error)
	GetTopCompletedChoresUsers(ctx context.Context, limit int) ([]*UserChoreStat, error)
	GetUserWeeklyStats(ctx context.Context, since time.Time) ([]*UserWeeklyStats, error)
	GetLastChoreDigestDate(ctx context.Context) (string, error)
	SetLastChoreDigestDate(ctx context.Context, date string) error

	// Participant rating methods
	SaveDailyParticipantRatings(ctx context.Context, date time.Time, ratings []*ParticipantDailyRating) error
	GetParticipantsForRating(ctx context.Context) ([]*User, error)
	GetCurrentMonthParticipantRatings(ctx context.Context, now time.Time) ([]*ParticipantDailyRating, error)
	GetMonthlyParticipantTotals(ctx context.Context, year int, month time.Month) ([]*ParticipantMonthlyTotal, error)

	// Queue management methods
	AddToVolunteerQueue(ctx context.Context, userID int64, days int) error
	AddToAdminQueue(ctx context.Context, userID int64, days int) error
	DecrementVolunteerQueue(ctx context.Context, userID int64) error
	DecrementAdminQueue(ctx context.Context, userID int64) error
	ReduceAdminQueue(ctx context.Context, userID int64, days int) error
	GetUsersWithVolunteerQueue(ctx context.Context) ([]*User, error)
	GetUsersWithAdminQueue(ctx context.Context) ([]*User, error)

	// Off-duty management methods
	SetOffDuty(ctx context.Context, userID int64, start, end time.Time) error
	ClearOffDuty(ctx context.Context, userID int64) error
	IsUserOffDuty(ctx context.Context, userID int64, date time.Time) (bool, error)
	GetOffDutyUsers(ctx context.Context, date time.Time) ([]*User, error)

	// Vacation mode methods
	SetVacationMode(ctx context.Context, enabled bool) error
	IsVacationMode(ctx context.Context) (bool, error)

	// Recurring Chore methods
	CreateRecurringChore(ctx context.Context, chore *RecurringChore) error
	GetRecurringChore(ctx context.Context, id int64) (*RecurringChore, error)
	GetActiveRecurringChores(ctx context.Context) ([]*RecurringChore, error)
	GetDueRecurringChores(ctx context.Context, before time.Time) ([]*RecurringChore, error)
	UpdateRecurringChoreNextRun(ctx context.Context, id int64, nextRun time.Time) error
	UpdateRecurringChoreDescription(ctx context.Context, id int64, description string) error
	CancelRecurringChore(ctx context.Context, id int64) error
}

// DutyService handles duty-related business logic.
type DutyService interface {
	AssignDuty(ctx context.Context, date time.Time, userID int64, assignmentType AssignmentType) error
	AutoAssignDuty(ctx context.Context, date time.Time) (*Duty, error)
	CompleteTodaysDuty(ctx context.Context) error
	GetSchedule(ctx context.Context, year int, month time.Month) ([]*Duty, error)
	ChangeDutyUser(ctx context.Context, date time.Time, newUserID int64) (*Duty, error)
}

// ChoreService handles chore-related business logic.
type ChoreService interface {
	CreateChore(ctx context.Context, description string, duration time.Duration) (*Chore, error)
	AssignChore(ctx context.Context, choreID int64, userID int64) error
	CompleteChore(ctx context.Context, reminderID string) error
	CancelChore(ctx context.Context, id int64) error
	ProcessRecurringChores(ctx context.Context) ([]*Chore, error)
}

// RatingService handles participant rating logic.
type RatingService interface {
	SubmitDailyRatings(ctx context.Context, date time.Time, scores map[int64]int) error
	GetMonthlyRatingsCalendar(ctx context.Context, year int, month time.Month) ([]*ParticipantDailyRating, error)
	GetMonthlyWinners(ctx context.Context, year int, month time.Month) ([]*ParticipantMonthlyTotal, error)
}
