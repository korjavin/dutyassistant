package mocks

import (
	"context"

	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/stretchr/testify/mock"
)

// MockScheduler is a mock implementation of the scheduler.Scheduler interface.
type MockScheduler struct {
	mock.Mock
}

func (m *MockScheduler) AssignDuty(ctx context.Context, user *store.User, date string) error {
	args := m.Called(ctx, user, date)
	return args.Error(0)
}

func (m *MockScheduler) VolunteerForDuty(ctx context.Context, user *store.User, date string) error {
	args := m.Called(ctx, user, date)
	return args.Error(0)
}

func (m *MockScheduler) AutoAssignDuty(ctx context.Context, date string) (*store.Duty, error) {
	args := m.Called(ctx, date)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*store.Duty), args.Error(1)
}