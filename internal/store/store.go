// /internal/store/store.go
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

// Duty represents a duty assignment in the system.
type Duty struct {
	ID             int64
	UserID         int64
	DutyDate       time.Time
	AssignmentType string
	CreatedAt      time.Time
	User           *User // Used to join user data
}

// RoundRobinState represents the state of the round-robin algorithm for a user.
type RoundRobinState struct {
	UserID                int64
	AssignmentCount       int
	LastAssignedTimestamp time.Time
}

// Store defines the interface for all data operations.
type Store interface {
	// User methods
	GetUserByTelegramID(ctx context.Context, id int64) (*User, error)
	ListActiveUsers(ctx context.Context) ([]*User, error)
	CreateUser(ctx context.Context, user *User) error
	UpdateUser(ctx context.Context, user *User) error

	// Duty methods
	CreateDuty(ctx context.Context, duty *Duty) error
	GetDutyByDate(ctx context.Context, date time.Time) (*Duty, error)
	UpdateDuty(ctx context.Context, duty *Duty) error
	DeleteDuty(ctx context.Context, date time.Time) error
	GetDutiesByMonth(ctx context.Context, year int, month time.Month) ([]*Duty, error)

	// Round-robin methods
	GetNextRoundRobinUser(ctx context.Context) (*User, error)
	IncrementAssignmentCount(ctx context.Context, userID int64, lastAssigned time.Time) error
}