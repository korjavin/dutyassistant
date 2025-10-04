package scheduler

import (
	"context"
	"github.com/korjavin/dutyassistant/internal/store"
	"time"
)

// Scheduler defines the interface for the core business logic of assigning duties.
type Scheduler interface {
	// AssignDuty assigns a user to a duty on a specific date.
	// This is an administrative action and should overwrite any existing duty.
	AssignDuty(ctx context.Context, user *store.User, date time.Time) error

	// VolunteerForDuty allows a user to volunteer for a duty on a specific date.
	// This should fail if the date is already taken by an admin assignment.
	VolunteerForDuty(ctx context.Context, user *store.User, date time.Time) error

	// AutoAssignDuty automatically assigns the next user in the round-robin sequence to a duty.
	// This is typically called by a background job.
	AutoAssignDuty(ctx context.Context, date time.Time) (*store.Duty, error)
}