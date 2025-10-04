package scheduler

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
	ID             int
	TelegramUserID int64
	FirstName      string
	IsAdmin        bool
	IsActive       bool
}

// Duty represents a duty assignment for a specific date.
type Duty struct {
	ID             int
	UserID         int
	DutyDate       time.Time
	AssignmentType AssignmentType
	CreatedAt      time.Time
}

// Store defines the interface for data persistence that the scheduler relies on.
// This allows the scheduler to be tested independently of the database implementation.
type Store interface {
	// GetDutyByDate retrieves a duty for a specific date.
	GetDutyByDate(ctx context.Context, date time.Time) (*Duty, error)
	// GetNextRoundRobinUser retrieves the next user for a round-robin assignment.
	// The implementation should handle the logic of finding the user with the
	// minimum assignment_count and use last_assigned_timestamp as a tie-breaker.
	GetNextRoundRobinUser(ctx context.Context) (*User, error)
	// CreateDuty creates a new duty assignment.
	CreateDuty(ctx context.Context, duty *Duty) error
	// UpdateDuty updates an existing duty assignment.
	UpdateDuty(ctx context.Context, duty *Duty) error
	// FindUserByName retrieves a user by their first name.
	FindUserByName(ctx context.Context, name string) (*User, error)
	// IncrementAssignmentCount increments the round-robin assignment count for a user.
	IncrementAssignmentCount(ctx context.Context, userID int) error
}