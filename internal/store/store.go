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
type UserStats struct {
	TotalDuties     int
	DutiesThisMonth int
	NextDutyDate    string // YYYY-MM-DD, or empty if none
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

	// Round-robin methods
	GetNextRoundRobinUser(ctx context.Context) (*User, error)
	IncrementAssignmentCount(ctx context.Context, userID int64, lastAssigned time.Time) error
}
