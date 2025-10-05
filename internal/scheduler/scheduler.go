package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/korjavin/dutyassistant/internal/store"
)

// Scheduler handles the business logic for duty assignments.
type Scheduler struct {
	store store.Store
}

// NewScheduler creates a new Scheduler with the given data store.
func NewScheduler(s store.Store) *Scheduler {
	return &Scheduler{store: s}
}

// AddToVolunteerQueue adds days to a user's volunteer queue.
func (s *Scheduler) AddToVolunteerQueue(ctx context.Context, userID int64, days int) error {
	if days <= 0 {
		return fmt.Errorf("days must be positive")
	}
	return s.store.AddToVolunteerQueue(ctx, userID, days)
}

// AddToAdminQueue adds days to a user's admin assignment queue.
func (s *Scheduler) AddToAdminQueue(ctx context.Context, userID int64, days int) error {
	if days <= 0 {
		return fmt.Errorf("days must be positive")
	}
	return s.store.AddToAdminQueue(ctx, userID, days)
}

// SetOffDuty sets a user's off-duty period.
func (s *Scheduler) SetOffDuty(ctx context.Context, userID int64, start, end time.Time) error {
	// Validate dates
	if end.Before(start) {
		return fmt.Errorf("end date must be after start date")
	}
	return s.store.SetOffDuty(ctx, userID, start, end)
}

// ClearOffDuty clears a user's off-duty period.
func (s *Scheduler) ClearOffDuty(ctx context.Context, userID int64) error {
	return s.store.ClearOffDuty(ctx, userID)
}

// AssignTodaysDuty performs the daily assignment at 11:00 AM Berlin time.
// Priority: Volunteer queue > Admin queue > Round-robin (with balancing).
func (s *Scheduler) AssignTodaysDuty(ctx context.Context) (*store.Duty, error) {
	now := time.Now()
	berlinLoc, _ := time.LoadLocation("Europe/Berlin")
	berlinNow := now.In(berlinLoc)

	// Check if it's past 11 AM in Berlin
	if berlinNow.Hour() < 11 {
		return nil, fmt.Errorf("too early to assign today's duty (before 11:00 AM Berlin time)")
	}

	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	// Check if already assigned
	existingDuty, err := s.store.GetDutyByDate(ctx, today)
	if err == nil && existingDuty != nil {
		return existingDuty, nil
	}

	// 1. Try volunteer queue first
	volunteers, err := s.store.GetUsersWithVolunteerQueue(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get volunteers: %w", err)
	}

	// Filter out off-duty users
	volunteers = s.filterOffDutyUsers(ctx, volunteers, today)

	if len(volunteers) > 0 {
		// If multiple volunteers with same queue count, use round-robin to balance
		user := s.selectUserWithBalancing(ctx, volunteers)
		duty, err := s.assignDuty(ctx, user, today, store.AssignmentTypeVoluntary)
		if err != nil {
			return nil, err
		}
		// Decrement volunteer queue
		s.store.DecrementVolunteerQueue(ctx, user.ID)
		return duty, nil
	}

	// 2. Try admin queue
	adminAssigned, err := s.store.GetUsersWithAdminQueue(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get admin-assigned users: %w", err)
	}

	// Filter out off-duty users
	adminAssigned = s.filterOffDutyUsers(ctx, adminAssigned, today)

	if len(adminAssigned) > 0 {
		// If multiple with same queue count, use round-robin to balance
		user := s.selectUserWithBalancing(ctx, adminAssigned)
		duty, err := s.assignDuty(ctx, user, today, store.AssignmentTypeAdmin)
		if err != nil {
			return nil, err
		}
		// Decrement admin queue
		s.store.DecrementAdminQueue(ctx, user.ID)
		return duty, nil
	}

	// 3. Fall back to round-robin
	allUsers, err := s.store.ListActiveUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get active users: %w", err)
	}

	// Filter out off-duty users
	allUsers = s.filterOffDutyUsers(ctx, allUsers, today)

	if len(allUsers) == 0 {
		return nil, fmt.Errorf("no available users for duty")
	}

	// Select user with least duties in last 14 days (excluding admin assignments)
	user := s.selectRoundRobinUser(ctx, allUsers)
	duty, err := s.assignDuty(ctx, user, today, store.AssignmentTypeRoundRobin)
	if err != nil {
		return nil, err
	}

	return duty, nil
}

// filterOffDutyUsers removes users who are off-duty on the given date.
func (s *Scheduler) filterOffDutyUsers(ctx context.Context, users []*store.User, date time.Time) []*store.User {
	var available []*store.User
	for _, user := range users {
		offDuty, _ := s.store.IsUserOffDuty(ctx, user.ID, date)
		if !offDuty {
			available = append(available, user)
		}
	}
	return available
}

// selectUserWithBalancing selects a user from those with the highest queue count.
// If multiple users have the same highest count, it uses round-robin balancing.
func (s *Scheduler) selectUserWithBalancing(ctx context.Context, users []*store.User) *store.User {
	if len(users) == 0 {
		return nil
	}

	// Find the maximum queue count
	maxQueue := 0
	for _, user := range users {
		queue := user.VolunteerQueueDays
		if user.AdminQueueDays > queue {
			queue = user.AdminQueueDays
		}
		if queue > maxQueue {
			maxQueue = queue
		}
	}

	// Get users with max queue count
	var maxQueueUsers []*store.User
	for _, user := range users {
		queue := user.VolunteerQueueDays
		if user.AdminQueueDays > queue {
			queue = user.AdminQueueDays
		}
		if queue == maxQueue {
			maxQueueUsers = append(maxQueueUsers, user)
		}
	}

	// If only one user, return it
	if len(maxQueueUsers) == 1 {
		return maxQueueUsers[0]
	}

	// Use round-robin balancing for multiple users
	return s.selectRoundRobinUser(ctx, maxQueueUsers)
}

// selectRoundRobinUser selects the user with the least completed duties in the last 14 days.
func (s *Scheduler) selectRoundRobinUser(ctx context.Context, users []*store.User) *store.User {
	if len(users) == 0 {
		return nil
	}

	// Calculate last 14 days
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	start := today.AddDate(0, 0, -14)

	// Get completed duties in the last 14 days (excluding admin assignments)
	duties, err := s.store.GetCompletedDutiesInRange(ctx, start, today)
	if err != nil {
		// If error, just return first user
		return users[0]
	}

	// Count duties per user (excluding admin assignments)
	dutyCounts := make(map[int64]int)
	for _, duty := range duties {
		if duty.AssignmentType != store.AssignmentTypeAdmin {
			dutyCounts[duty.UserID]++
		}
	}

	// Find user with minimum duty count
	var selectedUser *store.User
	minCount := int(^uint(0) >> 1) // max int

	for _, user := range users {
		count := dutyCounts[user.ID]
		if count < minCount {
			minCount = count
			selectedUser = user
		}
	}

	if selectedUser == nil {
		return users[0]
	}

	return selectedUser
}

// assignDuty creates a new duty assignment.
func (s *Scheduler) assignDuty(ctx context.Context, user *store.User, date time.Time, assignType store.AssignmentType) (*store.Duty, error) {
	newDuty := &store.Duty{
		UserID:         user.ID,
		DutyDate:       date,
		AssignmentType: assignType,
		CreatedAt:      time.Now().UTC(),
	}

	err := s.store.CreateDuty(ctx, newDuty)
	if err != nil {
		return nil, fmt.Errorf("failed to create duty: %w", err)
	}

	return newDuty, nil
}

// CompleteTodaysDuty marks today's duty as completed (runs at 21:00 PM Berlin time).
func (s *Scheduler) CompleteTodaysDuty(ctx context.Context) error {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	return s.store.CompleteDuty(ctx, today)
}

// ChangeDutyUser allows admin to change today's or future duty to a different user.
func (s *Scheduler) ChangeDutyUser(ctx context.Context, date time.Time, newUserID int64) (*store.Duty, error) {
	// Don't allow changing past duties
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	dutyDate := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)

	if dutyDate.Before(today) {
		return nil, fmt.Errorf("cannot change past duties")
	}

	existingDuty, err := s.store.GetDutyByDate(ctx, date)
	if err != nil || existingDuty == nil {
		return nil, fmt.Errorf("no duty found for this date")
	}

	// Update the duty
	existingDuty.UserID = newUserID
	err = s.store.UpdateDuty(ctx, existingDuty)
	if err != nil {
		return nil, fmt.Errorf("failed to update duty: %w", err)
	}

	return existingDuty, nil
}
