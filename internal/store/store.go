package store

import (
	"context"
	"time"
)

// User represents a user in the system.
type User struct {
	ID             int64  `json:"id"`
	TelegramUserID int64  `json:"telegram_user_id"`
	FirstName      string `json:"first_name"`
	IsAdmin        bool   `json:"is_admin"`
	IsActive       bool   `json:"is_active"`
}

// Duty represents a duty assignment in the system.
type Duty struct {
	ID             int64     `json:"id"`
	UserID         int64     `json:"user_id"`
	DutyDate       string    `json:"duty_date"` // YYYY-MM-DD
	AssignmentType string    `json:"assignment_type"`
	CreatedAt      time.Time `json:"created_at"`
}

// Store defines the interface for database operations.
// This is the "Repository" pattern described in the technical specification.
type Store interface {
	// User methods
	GetUserByTelegramID(ctx context.Context, id int64) (*User, error)
	ListActiveUsers(ctx context.Context) ([]*User, error)
	ListAllUsers(ctx context.Context) ([]*User, error)
	CreateUser(ctx context.Context, user *User) error
	UpdateUser(ctx context.Context, user *User) error

	// Duty methods
	CreateDuty(ctx context.Context, duty *Duty) error
	GetDutyByDate(ctx context.Context, date string) (*Duty, error)
	GetDutiesByMonth(ctx context.Context, year, month int) ([]*Duty, error)
	UpdateDuty(ctx context.Context, duty *Duty) error
	DeleteDuty(ctx context.Context, date string) error

	// Round Robin methods
	GetNextRoundRobinUser(ctx context.Context) (*User, error)
	IncrementAssignmentCount(ctx context.Context, userID int64) error
}