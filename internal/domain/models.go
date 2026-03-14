package domain

import "time"

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
type Duty struct {
	ID             int64
	UserID         int64
	DutyDate       time.Time
	AssignmentType AssignmentType
	CreatedAt      time.Time
	CompletedAt    *time.Time
	User           *User // Used to join user data
}

// Chore represents a chore assignment in the system.
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

// ChoreStat holds aggregated statistics for overdue chores.
type ChoreStat struct {
	Description string
	Count       int
}

// UserStats holds aggregated statistics for a user.
type UserStats struct {
	TotalDuties     int
	DutiesThisMonth int
	NextDutyDate    string // YYYY-MM-DD, or empty if none
}

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
