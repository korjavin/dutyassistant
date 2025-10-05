package scheduler

import (
	"context"
	"time"

	"github.com/korjavin/dutyassistant/internal/store"
)

// SchedulerInterface defines the interface for the core business logic of assigning duties.
// This is used by the Telegram bot handlers.
type SchedulerInterface interface {
	// AssignDuty adds days to a user's admin queue.
	AssignDuty(ctx context.Context, user *store.User, days int) error

	// VolunteerForDuty adds days to a user's volunteer queue.
	VolunteerForDuty(ctx context.Context, user *store.User, days int) error

	// AutoAssignDuty automatically assigns today's duty (runs at 11AM).
	AutoAssignDuty(ctx context.Context, date time.Time) (*store.Duty, error)

	// ChangeDutyUser changes the assigned user for today or a future duty.
	ChangeDutyUser(ctx context.Context, date time.Time, newUserID int64) (*store.Duty, error)

	// SetOffDuty sets a user's off-duty period.
	SetOffDuty(ctx context.Context, userID int64, start, end time.Time) error
}

// Verify that Scheduler implements SchedulerInterface
var _ SchedulerInterface = (*Scheduler)(nil)

// AssignDuty implements the SchedulerInterface by adding days to admin queue.
func (s *Scheduler) AssignDuty(ctx context.Context, user *store.User, days int) error {
	return s.AddToAdminQueue(ctx, user.ID, days)
}

// VolunteerForDuty implements the SchedulerInterface by adding days to volunteer queue.
func (s *Scheduler) VolunteerForDuty(ctx context.Context, user *store.User, days int) error {
	return s.AddToVolunteerQueue(ctx, user.ID, days)
}

// AutoAssignDuty implements the SchedulerInterface by assigning today's duty.
func (s *Scheduler) AutoAssignDuty(ctx context.Context, date time.Time) (*store.Duty, error) {
	return s.AssignTodaysDuty(ctx)
}
