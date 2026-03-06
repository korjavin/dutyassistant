package mocks

import (
	"context"
	"time"

	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/stretchr/testify/mock"
)

type MockScheduler struct {
	mock.Mock
}

func (m *MockScheduler) AssignDuty(ctx context.Context, user *store.User, days int) error {
	args := m.Called(ctx, user, days)
	return args.Error(0)
}

func (m *MockScheduler) UnassignDuty(ctx context.Context, user *store.User, days int) error {
	args := m.Called(ctx, user, days)
	return args.Error(0)
}

func (m *MockScheduler) VolunteerForDuty(ctx context.Context, user *store.User, days int) error {
	args := m.Called(ctx, user, days)
	return args.Error(0)
}

func (m *MockScheduler) AutoAssignDuty(ctx context.Context, date time.Time) (*store.Duty, error) {
	args := m.Called(ctx, date)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*store.Duty), args.Error(1)
}

func (m *MockScheduler) ChangeDutyUser(ctx context.Context, date time.Time, newUserID int64) (*store.Duty, error) {
	args := m.Called(ctx, date, newUserID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*store.Duty), args.Error(1)
}

func (m *MockScheduler) SetOffDuty(ctx context.Context, userID int64, start, end time.Time) error {
	args := m.Called(ctx, userID, start, end)
	return args.Error(0)
}

func (m *MockScheduler) SetVacationMode(ctx context.Context, enabled bool) error {
	args := m.Called(ctx, enabled)
	return args.Error(0)
}

func (m *MockScheduler) IsVacationMode(ctx context.Context) (bool, error) {
	args := m.Called(ctx)
	return args.Bool(0), args.Error(1)
}

func (m *MockScheduler) ExplainLastAssignment(ctx context.Context) (string, error) {
	args := m.Called(ctx)
	return args.String(0), args.Error(1)
}
