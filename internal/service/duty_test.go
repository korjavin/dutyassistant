package service

import (
	"context"
	"testing"
	"time"

	"github.com/korjavin/dutyassistant/internal/domain"
)

type mockDutyRepo struct {
	domain.Repository
	dutyCreated bool
	duty        *domain.Duty
}

func (m *mockDutyRepo) CreateDuty(ctx context.Context, duty *domain.Duty) error {
	m.dutyCreated = true
	m.duty = duty
	return nil
}

func (m *mockDutyRepo) IsVacationMode(ctx context.Context) (bool, error) {
	return false, nil
}

func (m *mockDutyRepo) GetDutyByDate(ctx context.Context, date time.Time) (*domain.Duty, error) {
	return nil, nil // No existing duty
}

func (m *mockDutyRepo) GetUsersWithVolunteerQueue(ctx context.Context) ([]*domain.User, error) {
	return []*domain.User{{ID: 1, VolunteerQueueDays: 1}}, nil
}

func (m *mockDutyRepo) GetOffDutyUsers(ctx context.Context, date time.Time) ([]*domain.User, error) {
	return nil, nil
}

func (m *mockDutyRepo) DecrementVolunteerQueue(ctx context.Context, userID int64) error {
	return nil
}

func TestDutyService_AutoAssignDuty(t *testing.T) {
	repo := &mockDutyRepo{}
	service := NewDutyService(repo)

	duty, err := service.AutoAssignDuty(context.Background(), time.Now())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if duty == nil {
		t.Fatal("expected duty to be returned")
	}

	if !repo.dutyCreated {
		t.Error("expected duty to be created in repo")
	}
}
