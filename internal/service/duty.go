package service

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/korjavin/dutyassistant/internal/domain"
)

type DutyServiceImpl struct {
	repo domain.Repository
}

func NewDutyService(repo domain.Repository) *DutyServiceImpl {
	return &DutyServiceImpl{repo: repo}
}

func (s *DutyServiceImpl) AssignDuty(ctx context.Context, date time.Time, userID int64, assignmentType domain.AssignmentType) error {
	duty := &domain.Duty{
		UserID:         userID,
		DutyDate:       date,
		AssignmentType: assignmentType,
		CreatedAt:      time.Now(),
	}
	return s.repo.CreateDuty(ctx, duty)
}

func (s *DutyServiceImpl) AutoAssignDuty(ctx context.Context, date time.Time) (*domain.Duty, error) {
	isVacation, err := s.repo.IsVacationMode(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check vacation mode: %w", err)
	}
	if isVacation {
		return nil, fmt.Errorf("system is in vacation mode - no duties will be assigned")
	}

	existingDuty, err := s.repo.GetDutyByDate(ctx, date)
	if err == nil && existingDuty != nil {
		return existingDuty, nil
	}

	volunteers, err := s.repo.GetUsersWithVolunteerQueue(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get volunteers: %w", err)
	}
	volunteers = s.filterOffDutyUsers(ctx, volunteers, date)
	if len(volunteers) > 0 {
		user := s.selectUserWithBalancing(volunteers)
		duty := &domain.Duty{
			UserID:         user.ID,
			DutyDate:       date,
			User:           user,
			AssignmentType: domain.AssignmentTypeVoluntary,
			CreatedAt:      time.Now().UTC(),
		}
		if err := s.repo.CreateDuty(ctx, duty); err != nil {
			return nil, err
		}
		s.repo.DecrementVolunteerQueue(ctx, user.ID)
		return duty, nil
	}

	adminAssigned, err := s.repo.GetUsersWithAdminQueue(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get admin-assigned users: %w", err)
	}
	adminAssigned = s.filterOffDutyUsers(ctx, adminAssigned, date)
	if len(adminAssigned) > 0 {
		user := s.selectUserWithBalancing(adminAssigned)
		duty := &domain.Duty{
			UserID:         user.ID,
			DutyDate:       date,
			User:           user,
			AssignmentType: domain.AssignmentTypeAdmin,
			CreatedAt:      time.Now().UTC(),
		}
		if err := s.repo.CreateDuty(ctx, duty); err != nil {
			return nil, err
		}
		s.repo.DecrementAdminQueue(ctx, user.ID)
		return duty, nil
	}

	allUsers, err := s.repo.ListActiveUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get active users: %w", err)
	}
	allUsers = s.filterOffDutyUsers(ctx, allUsers, date)
	if len(allUsers) == 0 {
		return nil, fmt.Errorf("no available users for duty")
	}

	user := s.selectRoundRobinUser(ctx, allUsers, date)
	duty := &domain.Duty{
		UserID:         user.ID,
		DutyDate:       date,
		User:           user,
		AssignmentType: domain.AssignmentTypeRoundRobin,
		CreatedAt:      time.Now().UTC(),
	}
	if err := s.repo.CreateDuty(ctx, duty); err != nil {
		return nil, err
	}
	return duty, nil
}

func (s *DutyServiceImpl) filterOffDutyUsers(ctx context.Context, users []*domain.User, date time.Time) []*domain.User {
	offDutyUsers, err := s.repo.GetOffDutyUsers(ctx, date)
	if err != nil {
		return users
	}
	offDutyMap := make(map[int64]bool)
	for _, u := range offDutyUsers {
		offDutyMap[u.ID] = true
	}

	var available []*domain.User
	for _, user := range users {
		if !offDutyMap[user.ID] {
			available = append(available, user)
		}
	}
	return available
}

func (s *DutyServiceImpl) selectUserWithBalancing(users []*domain.User) *domain.User {
	if len(users) == 0 {
		return nil
	}
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
	var maxQueueUsers []*domain.User
	for _, user := range users {
		queue := user.VolunteerQueueDays
		if user.AdminQueueDays > queue {
			queue = user.AdminQueueDays
		}
		if queue == maxQueue {
			maxQueueUsers = append(maxQueueUsers, user)
		}
	}
	return maxQueueUsers[rand.Intn(len(maxQueueUsers))]
}

func (s *DutyServiceImpl) selectRoundRobinUser(ctx context.Context, users []*domain.User, today time.Time) *domain.User {
	if len(users) == 0 {
		return nil
	}
	if len(users) == 1 {
		return users[0]
	}
	start := today.AddDate(0, 0, -14)
	duties, err := s.repo.GetCompletedDutiesInRange(ctx, start, today)
	if err != nil {
		return users[rand.Intn(len(users))]
	}

	dutyCounts := make(map[int64]int)
	for _, duty := range duties {
		if duty.AssignmentType != domain.AssignmentTypeAdmin {
			dutyCounts[duty.UserID]++
		}
	}

	minCount := int(^uint(0) >> 1)
	for _, user := range users {
		count := dutyCounts[user.ID]
		if count < minCount {
			minCount = count
		}
	}

	var candidateUsers []*domain.User
	for _, user := range users {
		count := dutyCounts[user.ID]
		if count == minCount {
			candidateUsers = append(candidateUsers, user)
		}
	}

	if len(candidateUsers) == 0 {
		return users[rand.Intn(len(users))]
	}
	return candidateUsers[rand.Intn(len(candidateUsers))]
}

func (s *DutyServiceImpl) CompleteTodaysDuty(ctx context.Context) error {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return s.repo.CompleteDuty(ctx, today)
}

func (s *DutyServiceImpl) GetSchedule(ctx context.Context, year int, month time.Month) ([]*domain.Duty, error) {
	return s.repo.GetDutiesByMonth(ctx, year, month)
}

func (s *DutyServiceImpl) ChangeDutyUser(ctx context.Context, date time.Time, newUserID int64) (*domain.Duty, error) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	dutyDate := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)

	if dutyDate.Before(today) {
		return nil, fmt.Errorf("cannot change past duties")
	}

	existingDuty, err := s.repo.GetDutyByDate(ctx, date)
	if err != nil || existingDuty == nil {
		return nil, fmt.Errorf("no duty found for this date")
	}

	existingDuty.UserID = newUserID
	err = s.repo.UpdateDuty(ctx, existingDuty)
	if err != nil {
		return nil, fmt.Errorf("failed to update duty: %w", err)
	}

	return existingDuty, nil
}
