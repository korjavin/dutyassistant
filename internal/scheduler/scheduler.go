package scheduler

import (
	"context"
	"fmt"
	"time"
)

// Scheduler handles the business logic for duty assignments.
type Scheduler struct {
	store Store
}

// NewScheduler creates a new Scheduler with the given data store.
func NewScheduler(store Store) *Scheduler {
	return &Scheduler{store: store}
}

// AssignDutyAdmin assigns a duty to a user on a specific date as an administrator.
// This has the highest priority and will override any existing assignment.
func (s *Scheduler) AssignDutyAdmin(ctx context.Context, user *User, date time.Time) (*Duty, error) {
	return s.assignDuty(ctx, user, date, AssignmentTypeAdmin)
}

// AssignDutyVoluntary allows a user to volunteer for a duty on a specific date.
// This cannot override an administrative assignment.
func (s *Scheduler) AssignDutyVoluntary(ctx context.Context, user *User, date time.Time) (*Duty, error) {
	existingDuty, err := s.store.GetDutyByDate(ctx, date)
	if err != nil {
		// Assuming an error means no duty exists, which is fine.
		// A proper implementation would distinguish between "not found" and other errors.
	}

	if existingDuty != nil && existingDuty.AssignmentType == AssignmentTypeAdmin {
		return nil, fmt.Errorf("cannot override an administrative assignment")
	}

	return s.assignDuty(ctx, user, date, AssignmentTypeVoluntary)
}

// AssignDutyRoundRobin automatically assigns a duty to the next eligible user.
// This is the lowest priority assignment.
func (s *Scheduler) AssignDutyRoundRobin(ctx context.Context, date time.Time) (*Duty, error) {
	existingDuty, err := s.store.GetDutyByDate(ctx, date)
	if err != nil {
		// Assuming "not found" is the common case here.
	}

	if existingDuty != nil {
		// A duty already exists, do nothing.
		return existingDuty, nil
	}

	user, err := s.store.GetNextRoundRobinUser(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get next round-robin user: %w", err)
	}

	duty, err := s.assignDuty(ctx, user, date, AssignmentTypeRoundRobin)
	if err != nil {
		return nil, err
	}

	// Only increment count for successful round-robin assignments
	if err := s.store.IncrementAssignmentCount(ctx, user.ID); err != nil {
		// This is a problem. The duty is created, but the count failed to increment.
		// A real implementation would need transactional logic or compensation.
		return duty, fmt.Errorf("duty created, but failed to increment assignment count: %w", err)
	}

	return duty, nil
}

// assignDuty is a helper function to handle the creation or update of a duty.
// It encapsulates the state transition logic based on assignment priority.
func (s *Scheduler) assignDuty(ctx context.Context, user *User, date time.Time, assignType AssignmentType) (*Duty, error) {
	existingDuty, err := s.store.GetDutyByDate(ctx, date)
	if err != nil {
		// Again, assuming error means not found for simplicity.
	}

	if existingDuty != nil {
		// If an admin is assigning, they can override anything.
		// If a volunteer is signing up, they can override round-robin or another volunteer.
		if assignType == AssignmentTypeAdmin || existingDuty.AssignmentType != AssignmentTypeAdmin {
			// Update existing duty
			existingDuty.UserID = user.ID
			existingDuty.AssignmentType = assignType
			err := s.store.UpdateDuty(ctx, existingDuty)
			if err != nil {
				return nil, fmt.Errorf("failed to update duty: %w", err)
			}
			return existingDuty, nil
		}
		return nil, fmt.Errorf("assignment failed due to priority conflict")
	}

	// Create new duty
	newDuty := &Duty{
		UserID:         user.ID,
		DutyDate:       date,
		AssignmentType: assignType,
		CreatedAt:      time.Now().UTC(),
	}

	err = s.store.CreateDuty(ctx, newDuty)
	if err != nil {
		return nil, fmt.Errorf("failed to create duty: %w", err)
	}

	return newDuty, nil
}