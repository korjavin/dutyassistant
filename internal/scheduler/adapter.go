package scheduler

import (
	"context"
	"time"

	"github.com/korjavin/dutyassistant/internal/store"
)

// SchedulerInterface defines the interface for the core business logic of assigning duties.
// This is used by the Telegram bot handlers.
type SchedulerInterface interface {
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

// Verify that Scheduler implements SchedulerInterface
var _ SchedulerInterface = (*Scheduler)(nil)

// AssignDuty implements the SchedulerInterface by calling AssignDutyAdmin.
func (s *Scheduler) AssignDuty(ctx context.Context, user *store.User, date time.Time) error {
	_, err := s.AssignDutyAdmin(ctx, user, date)
	return err
}

// VolunteerForDuty implements the SchedulerInterface by calling AssignDutyVoluntary.
func (s *Scheduler) VolunteerForDuty(ctx context.Context, user *store.User, date time.Time) error {
	_, err := s.AssignDutyVoluntary(ctx, user, date)
	return err
}

// AutoAssignDuty implements the SchedulerInterface by calling AssignDutyRoundRobin.
func (s *Scheduler) AutoAssignDuty(ctx context.Context, date time.Time) (*store.Duty, error) {
	return s.AssignDutyRoundRobin(ctx, date)
}
