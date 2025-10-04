package mocks

import (
	"context"

	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/stretchr/testify/mock"
)

// MockStore is a mock implementation of the store.Store interface,
// to be used in unit tests.
type MockStore struct {
	mock.Mock
}

// GetUserByTelegramID mocks the GetUserByTelegramID method.
func (m *MockStore) GetUserByTelegramID(ctx context.Context, id int64) (*store.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*store.User), args.Error(1)
}

// ListActiveUsers mocks the ListActiveUsers method.
func (m *MockStore) ListActiveUsers(ctx context.Context) ([]*store.User, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*store.User), args.Error(1)
}

// ListAllUsers mocks the ListAllUsers method.
func (m *MockStore) ListAllUsers(ctx context.Context) ([]*store.User, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*store.User), args.Error(1)
}

// CreateUser mocks the CreateUser method.
func (m *MockStore) CreateUser(ctx context.Context, user *store.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

// UpdateUser mocks the UpdateUser method.
func (m *MockStore) UpdateUser(ctx context.Context, user *store.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

// CreateDuty mocks the CreateDuty method.
func (m *MockStore) CreateDuty(ctx context.Context, duty *store.Duty) error {
	args := m.Called(ctx, duty)
	return args.Error(0)
}

// GetDutyByDate mocks the GetDutyByDate method.
func (m *MockStore) GetDutyByDate(ctx context.Context, date string) (*store.Duty, error) {
	args := m.Called(ctx, date)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*store.Duty), args.Error(1)
}

// GetDutiesByMonth mocks the GetDutiesByMonth method.
func (m *MockStore) GetDutiesByMonth(ctx context.Context, year, month int) ([]*store.Duty, error) {
	args := m.Called(ctx, year, month)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*store.Duty), args.Error(1)
}

// UpdateDuty mocks the UpdateDuty method.
func (m *MockStore) UpdateDuty(ctx context.Context, duty *store.Duty) error {
	args := m.Called(ctx, duty)
	return args.Error(0)
}

// DeleteDuty mocks the DeleteDuty method.
func (m *MockStore) DeleteDuty(ctx context.Context, date string) error {
	args := m.Called(ctx, date)
	return args.Error(0)
}

// GetNextRoundRobinUser mocks the GetNextRoundRobinUser method.
func (m *MockStore) GetNextRoundRobinUser(ctx context.Context) (*store.User, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*store.User), args.Error(1)
}

// IncrementAssignmentCount mocks the IncrementAssignmentCount method.
func (m *MockStore) IncrementAssignmentCount(ctx context.Context, userID int64) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}