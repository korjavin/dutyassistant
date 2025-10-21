package mocks

import (
	"context"
	"time"

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
func (m *MockStore) GetDutyByDate(ctx context.Context, date time.Time) (*store.Duty, error) {
	args := m.Called(ctx, date)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*store.Duty), args.Error(1)
}

// GetDutiesByMonth mocks the GetDutiesByMonth method.
func (m *MockStore) GetDutiesByMonth(ctx context.Context, year int, month time.Month) ([]*store.Duty, error) {
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
func (m *MockStore) DeleteDuty(ctx context.Context, date time.Time) error {
	args := m.Called(ctx, date)
	return args.Error(0)
}

// GetUserByName mocks the GetUserByName method.
func (m *MockStore) GetUserByName(ctx context.Context, name string) (*store.User, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*store.User), args.Error(1)
}

// GetUserStats mocks the GetUserStats method.
func (m *MockStore) GetUserStats(ctx context.Context, userID int64) (*store.UserStats, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*store.UserStats), args.Error(1)
}

// CompleteDuty mocks the CompleteDuty method.
func (m *MockStore) CompleteDuty(ctx context.Context, date time.Time) error {
	args := m.Called(ctx, date)
	return args.Error(0)
}

// GetTodaysDuty mocks the GetTodaysDuty method.
func (m *MockStore) GetTodaysDuty(ctx context.Context) (*store.Duty, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*store.Duty), args.Error(1)
}

// GetCompletedDutiesInRange mocks the GetCompletedDutiesInRange method.
func (m *MockStore) GetCompletedDutiesInRange(ctx context.Context, start, end time.Time) ([]*store.Duty, error) {
	args := m.Called(ctx, start, end)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*store.Duty), args.Error(1)
}

// AddToVolunteerQueue mocks the AddToVolunteerQueue method.
func (m *MockStore) AddToVolunteerQueue(ctx context.Context, userID int64, days int) error {
	args := m.Called(ctx, userID, days)
	return args.Error(0)
}

// AddToAdminQueue mocks the AddToAdminQueue method.
func (m *MockStore) AddToAdminQueue(ctx context.Context, userID int64, days int) error {
	args := m.Called(ctx, userID, days)
	return args.Error(0)
}

// DecrementVolunteerQueue mocks the DecrementVolunteerQueue method.
func (m *MockStore) DecrementVolunteerQueue(ctx context.Context, userID int64) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

// DecrementAdminQueue mocks the DecrementAdminQueue method.
func (m *MockStore) DecrementAdminQueue(ctx context.Context, userID int64) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

// GetUsersWithVolunteerQueue mocks the GetUsersWithVolunteerQueue method.
func (m *MockStore) GetUsersWithVolunteerQueue(ctx context.Context) ([]*store.User, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*store.User), args.Error(1)
}

// GetUsersWithAdminQueue mocks the GetUsersWithAdminQueue method.
func (m *MockStore) GetUsersWithAdminQueue(ctx context.Context) ([]*store.User, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*store.User), args.Error(1)
}

// SetOffDuty mocks the SetOffDuty method.
func (m *MockStore) SetOffDuty(ctx context.Context, userID int64, start, end time.Time) error {
	args := m.Called(ctx, userID, start, end)
	return args.Error(0)
}

// ClearOffDuty mocks the ClearOffDuty method.
func (m *MockStore) ClearOffDuty(ctx context.Context, userID int64) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

// IsUserOffDuty mocks the IsUserOffDuty method.
func (m *MockStore) IsUserOffDuty(ctx context.Context, userID int64, date time.Time) (bool, error) {
	args := m.Called(ctx, userID, date)
	return args.Bool(0), args.Error(1)
}

// GetOffDutyUsers mocks the GetOffDutyUsers method.
func (m *MockStore) GetOffDutyUsers(ctx context.Context, date time.Time) ([]*store.User, error) {
	args := m.Called(ctx, date)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*store.User), args.Error(1)
}

// SetVacationMode mocks the SetVacationMode method.
func (m *MockStore) SetVacationMode(ctx context.Context, enabled bool) error {
	args := m.Called(ctx, enabled)
	return args.Error(0)
}

// IsVacationMode mocks the IsVacationMode method.
func (m *MockStore) IsVacationMode(ctx context.Context) (bool, error) {
	args := m.Called(ctx)
	return args.Bool(0), args.Error(1)
}