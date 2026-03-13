// /internal/store/store.go
package store

import (
	"context"
	"time"
)

// AssignmentType defines the type of duty assignment.
type AssignmentType string

const (
	// AssignmentTypeRoundRobin is for duties assigned automatically by fair round-robin.
	AssignmentTypeRoundRobin AssignmentType = "round_robin"
	// AssignmentTypeVoluntary is for duties taken voluntarily by a user.
	AssignmentTypeVoluntary AssignmentType = "voluntary"
	// AssignmentTypeAdmin is for duties assigned by an administrator.
	AssignmentTypeAdmin AssignmentType = "admin"
)

// User represents a user in the system.
type User struct {
	ID                 int64
	TelegramUserID     int64
	FirstName          string
	IsAdmin            bool
	IsActive           bool
	VolunteerQueueDays int
	AdminQueueDays     int
	OffDutyStart       *time.Time
	OffDutyEnd         *time.Time
}

// Duty represents a duty assignment in the system.
// Chore represents a chore assignment in the system.
// ChoreStat holds aggregated statistics for overdue chores.
type ChoreStat struct {
	Description string
	Count       int
}

type Chore struct {
	ID          int64
	UserID      int64
	Description string
	AssignedAt  time.Time
	DeadlineAt  time.Time
	CompletedAt *time.Time
	CancelledAt *time.Time
	ReminderID  string
	User        *User
}

type Duty struct {
	ID             int64
	UserID         int64
	DutyDate       time.Time
	AssignmentType AssignmentType
	CreatedAt      time.Time
	CompletedAt    *time.Time
	User           *User // Used to join user data
}

// RoundRobinState represents the state of the round-robin algorithm for a user.
type RoundRobinState struct {
	UserID                int64
	AssignmentCount       int
	LastAssignedTimestamp time.Time
}

// UserStats holds aggregated statistics for a user.
// UserChoreStat holds aggregated statistics for a user's chores.
type UserChoreStat struct {
	Name  string
	Count int
}

// UserWeeklyStats holds aggregated weekly statistics for a user's completed chores.
type UserWeeklyStats struct {
	Name           string
	CompletedCount int
	AvgExecSeconds float64
	AvgLateSeconds float64
}

type UserStats struct {
	TotalDuties     int
	DutiesThisMonth int
	NextDutyDate    string // YYYY-MM-DD, or empty if none
}

// ParticipantDailyRating represents one participant's score for one calendar day.
type ParticipantDailyRating struct {
	ParticipantID   int64
	ParticipantName string
	RatingDate      time.Time
	Score           int
}

// ParticipantMonthlyTotal represents a participant's aggregate score for one month.
type ParticipantMonthlyTotal struct {
	ParticipantID   int64
	ParticipantName string
	TotalScore      int
	DaysRated       int
}

// RecurringChore represents a scheduled periodic chore.
type RecurringChore struct {
	ID          int64
	Description string
	Interval    int       // Interval in days
	NextRunAt   time.Time // Time when the chore is next due
	IsActive    bool
	CreatedAt   time.Time
}

// Store defines the interface for all data operations.
type Store interface {
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
