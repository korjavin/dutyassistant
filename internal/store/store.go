package store

import (
	"context"
	"time"
)

// User represents a user in the system.
type User struct {
	ID             int64
	TelegramUserID int64
	FirstName      string
	IsAdmin        bool
	IsActive       bool
}

// Duty represents a duty assignment.
type Duty struct {
	ID             int64
	UserID         int64
	User           *User // Used for joined queries
	DutyDate       time.Time
	AssignmentType string
	CreatedAt      time.Time
}

// UserStats holds aggregated statistics for a user.
type UserStats struct {
	TotalDuties     int
	DutiesThisMonth int
	NextDutyDate    string // YYYY-MM-DD, or empty if none
}

// Store defines the interface for interacting with the database.
// This interface is based on the feedback to be compatible with the existing data layer.
type Store interface {
	// User methods
	CreateUser(ctx context.Context, user *User) error
	GetUserByTelegramID(ctx context.Context, telegramID int64) (*User, error)
	GetUserByName(ctx context.Context, name string) (*User, error)
	ListActiveUsers(ctx context.Context) ([]*User, error)
	ListAllUsers(ctx context.Context) ([]*User, error)
	UpdateUser(ctx context.Context, user *User) error
	GetUserStats(ctx context.Context, userID int64) (*UserStats, error)

	// Duty methods
	CreateDuty(ctx context.Context, duty *Duty) error
	GetDutyByDate(ctx context.Context, date time.Time) (*Duty, error)
	GetDutiesByMonth(ctx context.Context, year int, month time.Month) ([]*Duty, error)
	UpdateDuty(ctx context.Context, duty *Duty) error
	DeleteDuty(ctx context.Context, date time.Time) error

	// Round Robin methods
	GetNextRoundRobinUser(ctx context.Context) (*User, error)
	IncrementAssignmentCount(ctx context.Context, userID int64) error
}