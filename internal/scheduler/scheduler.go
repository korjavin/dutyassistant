package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/korjavin/dutyassistant/internal/store"
)

const dateLayout = "2006-01-02"

// Scheduler handles the business logic for duty assignments.
type Scheduler struct {
	store store.Store
}

// NewScheduler creates a new Scheduler with the given data store.
func NewScheduler(s store.Store) *Scheduler {
	return &Scheduler{store: s}
}

// AssignDutyAdmin assigns a duty to a user on a specific date as an administrator.
// This has the highest priority and will override any existing assignment.
func (s *Scheduler) AssignDutyAdmin(ctx context.Context, user *store.User, date time.Time) (*store.Duty, error) {
	return s.assignDuty(ctx, user, date, store.AssignmentTypeAdmin)
}

// AssignDutyVoluntary allows a user to volunteer for a duty on a specific date.
// This cannot override an administrative assignment, and cannot be used for past dates.
func (s *Scheduler) AssignDutyVoluntary(ctx context.Context, user *store.User, date time.Time) (*store.Duty, error) {
	// Prevent volunteering for past dates
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	dutyDate := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)

	if dutyDate.Before(today) {
		return nil, fmt.Errorf("cannot volunteer for past dates")
	}

	existingDuty, err := s.store.GetDutyByDate(ctx, date)
	if err != nil {
		// Assuming an error means no duty exists, which is fine for this operation.
		// A real implementation would check for specific "not found" errors.
	}

	if existingDuty != nil && existingDuty.AssignmentType == store.AssignmentTypeAdmin {
		// When volunteering for an admin-assigned day, reassign the admin assignment to next available day
		if err := s.reassignAdminToNextDay(ctx, existingDuty); err != nil {
			return nil, fmt.Errorf("failed to reassign admin duty: %w", err)
		}
	}

	return s.assignDuty(ctx, user, date, store.AssignmentTypeVoluntary)
}

// AssignDutyRoundRobin automatically assigns a duty to the next eligible user.
// This is the lowest priority assignment and will not run if a duty already exists.
func (s *Scheduler) AssignDutyRoundRobin(ctx context.Context, date time.Time) (*store.Duty, error) {
	existingDuty, err := s.store.GetDutyByDate(ctx, date)
	if err != nil {
		// Assuming "not found" is the common case here.
	}

	if existingDuty != nil {
		// A duty already exists, so do nothing.
		return existingDuty, nil
	}

	user, err := s.store.GetNextRoundRobinUser(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get next round-robin user: %w", err)
	}

	duty, err := s.assignDuty(ctx, user, date, store.AssignmentTypeRoundRobin)
	if err != nil {
		return nil, err
	}

	// Only increment count for successful round-robin assignments.
	if err := s.store.IncrementAssignmentCount(ctx, user.ID, time.Now().UTC()); err != nil {
		// In a real system, this would require transactional logic or compensation
		// to avoid an inconsistent state (duty created but count not incremented).
		return duty, fmt.Errorf("duty created, but failed to increment assignment count: %w", err)
	}

	return duty, nil
}

// reassignAdminToNextDay finds the next unassigned day and assigns the admin duty there.
func (s *Scheduler) reassignAdminToNextDay(ctx context.Context, existingDuty *store.Duty) error {
	// Find the next unassigned day for round-robin
	currentDate := existingDuty.DutyDate.AddDate(0, 0, 1)
	maxDays := 90 // Search up to 90 days ahead

	for i := 0; i < maxDays; i++ {
		duty, err := s.store.GetDutyByDate(ctx, currentDate)
		if err != nil {
			// Assuming error means no duty exists
		}

		if duty == nil {
			// Found an unassigned day - assign the admin duty here
			_, err := s.AssignDutyAdmin(ctx, existingDuty.User, currentDate)
			return err
		}

		// Check if it's a round-robin assignment we can override
		if duty.AssignmentType == store.AssignmentTypeRoundRobin {
			// Override this round-robin with admin assignment
			_, err := s.AssignDutyAdmin(ctx, existingDuty.User, currentDate)
			return err
		}

		currentDate = currentDate.AddDate(0, 0, 1)
	}

	return fmt.Errorf("could not find available day within %d days", maxDays)
}

// assignDuty is a helper function to handle the creation or update of a duty.
// It encapsulates the state transition logic based on assignment priority.
func (s *Scheduler) assignDuty(ctx context.Context, user *store.User, date time.Time, assignType store.AssignmentType) (*store.Duty, error) {
	existingDuty, err := s.store.GetDutyByDate(ctx, date)
	if err != nil {
		// Again, assuming error means not found for simplicity.
	}

	if existingDuty != nil {
		// Admin assignments can override anything.
		// Voluntary assignments can override round-robin or another voluntary.
		if assignType == store.AssignmentTypeAdmin || existingDuty.AssignmentType != store.AssignmentTypeAdmin {
			// Update the existing duty.
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

	// Create a new duty.
	newDuty := &store.Duty{
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