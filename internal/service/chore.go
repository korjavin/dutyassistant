package service

import (
	"context"
	"time"

	"github.com/korjavin/dutyassistant/internal/domain"
)

type ChoreServiceImpl struct {
	repo domain.Repository
}

func NewChoreService(repo domain.Repository) *ChoreServiceImpl {
	return &ChoreServiceImpl{repo: repo}
}

func (s *ChoreServiceImpl) CreateChore(ctx context.Context, description string, duration time.Duration) (*domain.Chore, error) {
	now := time.Now()
	chore := &domain.Chore{
		Description: description,
		AssignedAt:  now,
		DeadlineAt:  now.Add(duration),
	}

	err := s.repo.CreateChore(ctx, chore)
	if err != nil {
		return nil, err
	}

	return chore, nil
}

func (s *ChoreServiceImpl) AssignChore(ctx context.Context, choreID int64, userID int64) error {
	// Need to get chore first then update UserID
	// Implementation will depend on repository interface mapping
	return nil
}

func (s *ChoreServiceImpl) CompleteChore(ctx context.Context, reminderID string) error {
	return s.repo.CompleteChoreByReminderID(ctx, reminderID)
}

func (s *ChoreServiceImpl) CancelChore(ctx context.Context, id int64) error {
	_, err := s.repo.CancelChore(ctx, id)
	return err
}

func (s *ChoreServiceImpl) ProcessRecurringChores(ctx context.Context) ([]*domain.Chore, error) {
	return nil, nil
}
