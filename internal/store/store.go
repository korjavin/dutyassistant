package store

import "context"

// User represents a user in the system, based on the schema in Agent.md.
type User struct {
	ID             int
	TelegramUserID int64
	FirstName      string
	IsAdmin        bool
	IsActive       bool
}

// Duty represents a duty assignment, based on the schema in Agent.md.
type Duty struct {
	ID             int
	UserID         int
	DutyDate       string // YYYY-MM-DD
	AssignmentType string // 'round_robin', 'voluntary', 'admin'
	CreatedAt      string // ISO 8601
}

// UserStats holds aggregated statistics for a user.
type UserStats struct {
	TotalDuties     int
	DutiesThisMonth int
	NextDutyDate    string // YYYY-MM-DD, or empty if none
}

// Store defines the interface for interacting with the database.
// This is the "Repository" pattern described in Agent.md.
type Store interface {
	// User methods
	GetUserByTelegramID(ctx context.Context, id int64) (*User, error)
	GetUserByName(ctx context.Context, name string) (*User, error)
	ListAllUsers(ctx context.Context) ([]*User, error)
	ListActiveUsers(ctx context.Context) ([]*User, error)
	UpdateUser(ctx context.Context, user *User) error
	GetUserStats(ctx context.Context, userID int) (*UserStats, error)

	// Duty methods
	CreateDuty(ctx context.Context, duty *Duty) error
	GetDutyByDate(ctx context.Context, date string) (*Duty, error)
	UpdateDuty(ctx context.Context, duty *Duty) error

	// Round Robin methods
	GetNextRoundRobinUser(ctx context.Context) (*User, error)
	IncrementAssignmentCount(ctx context.Context, userID int) error
}