package scheduler

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
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

// MaxVolunteerQueueDays caps how many days a user may hold in the volunteer queue.
const MaxVolunteerQueueDays = 7

// AddToVolunteerQueue adds days to a user's volunteer queue, capped at
// MaxVolunteerQueueDays in total (not per call).
func (s *Scheduler) AddToVolunteerQueue(ctx context.Context, userID int64, days int) error {
	if days <= 0 {
		return fmt.Errorf("days must be positive")
	}
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to load user: %w", err)
	}
	if user == nil {
		return fmt.Errorf("user not found")
	}
	if user.VolunteerQueueDays+days > MaxVolunteerQueueDays {
		return fmt.Errorf("volunteer queue is capped at %d days (you already have %d)",
			MaxVolunteerQueueDays, user.VolunteerQueueDays)
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

// ReduceAdminQueue reduces days from a user's admin assignment queue.
func (s *Scheduler) ReduceAdminQueue(ctx context.Context, userID int64, days int) error {
	if days <= 0 {
		return fmt.Errorf("days must be positive")
	}
	return s.store.ReduceAdminQueue(ctx, userID, days)
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
	// Check if vacation mode is enabled
	isVacation, err := s.store.IsVacationMode(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check vacation mode: %w", err)
	}
	if isVacation {
		return nil, fmt.Errorf("system is in vacation mode - no duties will be assigned")
	}

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
		_ = s.store.DecrementVolunteerQueue(ctx, user.ID)
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
		_ = s.store.DecrementAdminQueue(ctx, user.ID)
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

	// Select user with least weighted duty load in last year (excluding admin assignments)
	user := s.selectRoundRobinUser(ctx, allUsers)
	duty, err := s.assignDuty(ctx, user, today, store.AssignmentTypeRoundRobin)
	if err != nil {
		return nil, err
	}

	return duty, nil
}

// filterOffDutyUsers removes users who are off-duty on the given date.
func (s *Scheduler) filterOffDutyUsers(ctx context.Context, users []*store.User, date time.Time) []*store.User {
	offDutyUsers, err := s.store.GetOffDutyUsers(ctx, date) // Fix N+1 query
	if err != nil {
		return users // Return all users on error to be safe
	}
	if len(offDutyUsers) == 0 {
		return users
	}
	offDutyMap := make(map[int64]struct{}, len(offDutyUsers)) // memory optimization
	for _, u := range offDutyUsers {
		offDutyMap[u.ID] = struct{}{}
	}
	available := make([]*store.User, 0, len(users)) // capacity pre-allocation
	for _, user := range users {
		if _, off := offDutyMap[user.ID]; !off {
			available = append(available, user)
		}
	}
	return available
}

// selectUserWithBalancing selects a user from those with the highest queue count.
// If multiple users have the same highest count, one is randomly selected.
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

	// Randomly select from users with max queue count for fairness
	idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(maxQueueUsers))))
	if err != nil {
		return maxQueueUsers[0]
	}
	return maxQueueUsers[idx.Int64()]
}

// Round-robin weighting (scaled ×10 for integer math):
//   - Admin-assigned days are ignored (they are not a favour to the assignee).
//   - Round-robin days count as 10.
//   - Voluntary days count as 12 (a 20% bonus, so a volunteer doing 10 days
//     is "fair" against someone assigned 12 round-robin days).
const (
	roundRobinLookbackDays = 365
	weightRoundRobin       = 10
	weightVoluntary        = 12
)

// dutyLoadWeight returns the scaled round-robin weight contributed by a duty.
// Returns 0 for duty types that should be excluded from fairness calculations.
func dutyLoadWeight(t store.AssignmentType) int {
	switch t {
	case store.AssignmentTypeRoundRobin:
		return weightRoundRobin
	case store.AssignmentTypeVoluntary:
		return weightVoluntary
	default:
		return 0
	}
}

// selectRoundRobinUser selects the user with the least weighted duty load over
// the last year. Voluntary days are weighted 1.2× versus round-robin days,
// and admin-assigned days are excluded.
// If multiple users tie on the minimum, one is randomly selected for fairness.
func (s *Scheduler) selectRoundRobinUser(ctx context.Context, users []*store.User) *store.User {
	if len(users) == 0 {
		return nil
	}

	// If only one user, return it immediately
	if len(users) == 1 {
		return users[0]
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	start := today.AddDate(0, 0, -roundRobinLookbackDays)

	duties, err := s.store.GetCompletedDutiesInRange(ctx, start, today)
	if err != nil {
		// If error, randomize selection among all available users
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(users))))
		if err != nil {
			return users[0]
		}
		return users[idx.Int64()]
	}

	// Sum weighted load per user (admin days contribute 0).
	dutyLoad := make(map[int64]int)
	for _, duty := range duties {
		dutyLoad[duty.UserID] += dutyLoadWeight(duty.AssignmentType)
	}

	// Find minimum load
	minLoad := int(^uint(0) >> 1) // max int
	for _, user := range users {
		load := dutyLoad[user.ID]
		if load < minLoad {
			minLoad = load
		}
	}

	// Collect all users with minimum load
	var candidateUsers []*store.User
	for _, user := range users {
		if dutyLoad[user.ID] == minLoad {
			candidateUsers = append(candidateUsers, user)
		}
	}

	// Randomly select from candidates for fairness
	if len(candidateUsers) == 0 {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(users))))
		if err != nil {
			return users[0]
		}
		return users[idx.Int64()]
	}

	idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(candidateUsers))))
	if err != nil {
		return candidateUsers[0]
	}
	return candidateUsers[idx.Int64()]
}

// assignDuty creates a new duty assignment.
func (s *Scheduler) assignDuty(ctx context.Context, user *store.User, date time.Time, assignType store.AssignmentType) (*store.Duty, error) {
	newDuty := &store.Duty{
		UserID:         user.ID,
		User:           user, // Populate the User field so notifications can be sent
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

// SetVacationMode sets the system vacation mode state.
func (s *Scheduler) SetVacationMode(ctx context.Context, enabled bool) error {
	return s.store.SetVacationMode(ctx, enabled)
}

// IsVacationMode checks if the system is in vacation mode.
func (s *Scheduler) IsVacationMode(ctx context.Context) (bool, error) {
	return s.store.IsVacationMode(ctx)
}
