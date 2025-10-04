package mocks

import (
	"context"
	"time"

	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/stretchr/testify/mock"
)

// MockStore is a mock implementation of the store.Store interface.
type MockStore struct {
	mock.Mock
}

func (m *MockStore) CreateUser(ctx context.Context, user *store.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockStore) GetUserByTelegramID(ctx context.Context, telegramID int64) (*store.User, error) {
	args := m.Called(ctx, telegramID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*store.User), args.Error(1)
}

func (m *MockStore) GetUserByName(ctx context.Context, name string) (*store.User, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*store.User), args.Error(1)
}

func (m *MockStore) ListActiveUsers(ctx context.Context) ([]*store.User, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*store.User), args.Error(1)
}

func (m *MockStore) ListAllUsers(ctx context.Context) ([]*store.User, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*store.User), args.Error(1)
}

func (m *MockStore) UpdateUser(ctx context.Context, user *store.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockStore) GetUserStats(ctx context.Context, userID int64) (*store.UserStats, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*store.UserStats), args.Error(1)
}

func (m *MockStore) CreateDuty(ctx context.Context, duty *store.Duty) error {
	args := m.Called(ctx, duty)
	return args.Error(0)
}

func (m *MockStore) GetDutyByDate(ctx context.Context, date time.Time) (*store.Duty, error) {
	args := m.Called(ctx, date)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*store.Duty), args.Error(1)
}

func (m *MockStore) GetDutiesByMonth(ctx context.Context, year int, month time.Month) ([]*store.Duty, error) {
	args := m.Called(ctx, year, month)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*store.Duty), args.Error(1)
}

func (m *MockStore) UpdateDuty(ctx context.Context, duty *store.Duty) error {
	args := m.Called(ctx, duty)
	return args.Error(0)
}

func (m *MockStore) DeleteDuty(ctx context.Context, date time.Time) error {
	args := m.Called(ctx, date)
	return args.Error(0)
}

func (m *MockStore) GetNextRoundRobinUser(ctx context.Context) (*store.User, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*store.User), args.Error(1)
}

func (m *MockStore) IncrementAssignmentCount(ctx context.Context, userID int64, lastAssigned time.Time) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}