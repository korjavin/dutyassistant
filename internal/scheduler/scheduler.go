package scheduler

import (
	"context"
	"github.com/korjavin/dutyassistant/internal/store"
)

// Scheduler defines the interface for the core business logic of assigning duties.
type Scheduler interface {
	AssignDuty(ctx context.Context, user *store.User, date string) error
	VolunteerForDuty(ctx context.Context, user *store.User, date string) error
	AutoAssignDuty(ctx context.Context, date string) (*store.Duty, error)
}