package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/korjavin/dutyassistant/internal/store"
)

// mockStore is a mock implementation of the store.Store interface for testing.
type mockStore struct {
	duties               map[string]*store.Duty
	users                []*store.User
	nextRoundRobinUser   *store.User
	roundRobinError      error
	assignmentCountError error
	findUserError        error
}

// newMockStore initializes a new mockStore with some default data.
func newMockStore() *mockStore {
	return &mockStore{
		duties: make(map[string]*store.Duty),
		users: []*store.User{
			{ID: 1, FirstName: "Alice", IsActive: true},
			{ID: 2, FirstName: "Bob", IsActive: true, IsAdmin: true},
			{ID: 3, FirstName: "Charlie", IsActive: false},
		},
	}
}

func (m *mockStore) GetDutyByDate(ctx context.Context, date time.Time) (*store.Duty, error) {
	key := date.Format("2006-01-02")
	duty, exists := m.duties[key]
	if !exists {
		return nil, errors.New("not found")
	}
	return duty, nil
}

func (m *mockStore) GetNextRoundRobinUser(ctx context.Context) (*store.User, error) {
	if m.roundRobinError != nil {
		return nil, m.roundRobinError
	}
	return m.nextRoundRobinUser, nil
}

func (m *mockStore) CreateDuty(ctx context.Context, duty *store.Duty) error {
	key := duty.DutyDate.Format("2006-01-02")
	if _, exists := m.duties[key]; exists {
		return errors.New("duty already exists on this date")
	}
	duty.ID = int64(len(m.duties) + 1)
	m.duties[key] = duty
	return nil
}

func (m *mockStore) UpdateDuty(ctx context.Context, duty *store.Duty) error {
	key := duty.DutyDate.Format("2006-01-02")
	m.duties[key] = duty
	return nil
}

func (m *mockStore) FindUserByName(ctx context.Context, name string) (*store.User, error) {
	if m.findUserError != nil {
		return nil, m.findUserError
	}
	for _, u := range m.users {
		if u.FirstName == name {
			return u, nil
		}
	}
	return nil, errors.New("user not found")
}

func (m *mockStore) IncrementAssignmentCount(ctx context.Context, userID int64, lastAssigned time.Time) error {
	return m.assignmentCountError
}

// Stub implementations for remaining Store interface methods
func (m *mockStore) GetUserByTelegramID(ctx context.Context, id int64) (*store.User, error) {
	for _, u := range m.users {
		if u.TelegramUserID == id {
			return u, nil
		}
	}
	return nil, errors.New("user not found")
}

func (m *mockStore) ListActiveUsers(ctx context.Context) ([]*store.User, error) {
	var active []*store.User
	for _, u := range m.users {
		if u.IsActive {
			active = append(active, u)
		}
	}
	return active, nil
}

func (m *mockStore) CreateUser(ctx context.Context, user *store.User) error {
	user.ID = int64(len(m.users) + 1)
	m.users = append(m.users, user)
	return nil
}

func (m *mockStore) GetUserByName(ctx context.Context, name string) (*store.User, error) {
	for _, u := range m.users {
		if u.FirstName == name {
			return u, nil
		}
	}
	return nil, errors.New("user not found")
}

func (m *mockStore) ListAllUsers(ctx context.Context) ([]*store.User, error) {
	return m.users, nil
}

func (m *mockStore) UpdateUser(ctx context.Context, user *store.User) error {
	return nil
}

func (m *mockStore) GetUserStats(ctx context.Context, userID int64) (*store.UserStats, error) {
	return &store.UserStats{}, nil
}

func (m *mockStore) DeleteDuty(ctx context.Context, date time.Time) error {
	key := date.Format("2006-01-02")
	delete(m.duties, key)
	return nil
}

func (m *mockStore) GetDutiesByMonth(ctx context.Context, year int, month time.Month) ([]*store.Duty, error) {
	var result []*store.Duty
	for _, d := range m.duties {
		result = append(result, d)
	}
	return result, nil
}

// Stub implementations for new queue and off-duty methods
func (m *mockStore) CompleteDuty(ctx context.Context, date time.Time) error {
	key := date.Format("2006-01-02")
	if duty, exists := m.duties[key]; exists {
		now := time.Now()
		duty.CompletedAt = &now
	}
	return nil
}

func (m *mockStore) GetTodaysDuty(ctx context.Context) (*store.Duty, error) {
	today := time.Now().Truncate(24 * time.Hour)
	return m.GetDutyByDate(ctx, today)
}

func (m *mockStore) GetCompletedDutiesInRange(ctx context.Context, start, end time.Time) ([]*store.Duty, error) {
	return []*store.Duty{}, nil
}

func (m *mockStore) GetLastDuty(ctx context.Context) (*store.Duty, error) {
	return nil, nil // not fully mocked
}

func (m *mockStore) AddToVolunteerQueue(ctx context.Context, userID int64, days int) error {
	for _, u := range m.users {
		if u.ID == userID {
			u.VolunteerQueueDays += days
			return nil
		}
	}
	return errors.New("user not found")
}

func (m *mockStore) AddToAdminQueue(ctx context.Context, userID int64, days int) error {
	for _, u := range m.users {
		if u.ID == userID {
			u.AdminQueueDays += days
			return nil
		}
	}
	return errors.New("user not found")
}

func (m *mockStore) DecrementVolunteerQueue(ctx context.Context, userID int64) error {
	for _, u := range m.users {
		if u.ID == userID && u.VolunteerQueueDays > 0 {
			u.VolunteerQueueDays--
			return nil
		}
	}
	return nil
}

func (m *mockStore) DecrementAdminQueue(ctx context.Context, userID int64) error {
	return m.ReduceAdminQueue(ctx, userID, 1)
}

func (m *mockStore) ReduceAdminQueue(ctx context.Context, userID int64, days int) error {
	for _, u := range m.users {
		if u.ID == userID {
			if u.AdminQueueDays > days {
				u.AdminQueueDays -= days
			} else {
				u.AdminQueueDays = 0
			}
			return nil
		}
	}
	return nil
}

func (m *mockStore) GetUsersWithVolunteerQueue(ctx context.Context) ([]*store.User, error) {
	var result []*store.User
	for _, u := range m.users {
		if u.VolunteerQueueDays > 0 {
			result = append(result, u)
		}
	}
	return result, nil
}

func (m *mockStore) GetUsersWithAdminQueue(ctx context.Context) ([]*store.User, error) {
	var result []*store.User
	for _, u := range m.users {
		if u.AdminQueueDays > 0 {
			result = append(result, u)
		}
	}
	return result, nil
}

func (m *mockStore) SetOffDuty(ctx context.Context, userID int64, start, end time.Time) error {
	for _, u := range m.users {
		if u.ID == userID {
			u.OffDutyStart = &start
			u.OffDutyEnd = &end
			return nil
		}
	}
	return errors.New("user not found")
}

func (m *mockStore) ClearOffDuty(ctx context.Context, userID int64) error {
	for _, u := range m.users {
		if u.ID == userID {
			u.OffDutyStart = nil
			u.OffDutyEnd = nil
			return nil
		}
	}
	return errors.New("user not found")
}

func (m *mockStore) IsUserOffDuty(ctx context.Context, userID int64, date time.Time) (bool, error) {
	for _, u := range m.users {
		if u.ID == userID && u.OffDutyStart != nil && u.OffDutyEnd != nil {
			return !date.Before(*u.OffDutyStart) && !date.After(*u.OffDutyEnd), nil
		}
	}
	return false, nil
}

func (m *mockStore) GetOffDutyUsers(ctx context.Context, date time.Time) ([]*store.User, error) {
	var result []*store.User
	for _, u := range m.users {
		if u.OffDutyStart != nil && u.OffDutyEnd != nil {
			if !date.Before(*u.OffDutyStart) && !date.After(*u.OffDutyEnd) {
				result = append(result, u)
			}
		}
	}
	return result, nil
}

func (m *mockStore) SetVacationMode(ctx context.Context, enabled bool) error {
	return nil
}

func (m *mockStore) IsVacationMode(ctx context.Context) (bool, error) {
	return false, nil
}

func TestScheduler_AddToVolunteerQueue(t *testing.T) {
	mock := newMockStore()
	scheduler := NewScheduler(mock)
	ctx := context.Background()

	err := scheduler.AddToVolunteerQueue(ctx, 1, 3)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify the queue was updated
	if mock.users[0].VolunteerQueueDays != 3 {
		t.Errorf("Expected 3 volunteer queue days, got %d", mock.users[0].VolunteerQueueDays)
	}
}

func TestScheduler_AddToAdminQueue(t *testing.T) {
	mock := newMockStore()
	scheduler := NewScheduler(mock)
	ctx := context.Background()

	err := scheduler.AddToAdminQueue(ctx, 1, 2)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify the queue was updated
	if mock.users[0].AdminQueueDays != 2 {
		t.Errorf("Expected 2 admin queue days, got %d", mock.users[0].AdminQueueDays)
	}
}

func TestScheduler_ReduceAdminQueue(t *testing.T) {
	mock := newMockStore()
	scheduler := NewScheduler(mock)
	ctx := context.Background()

	// Initial setup
	mock.users[0].AdminQueueDays = 5

	// Reduce by 2
	err := scheduler.ReduceAdminQueue(ctx, 1, 2)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify the queue was updated
	if mock.users[0].AdminQueueDays != 3 {
		t.Errorf("Expected 3 admin queue days, got %d", mock.users[0].AdminQueueDays)
	}

	// Reduce by 10 (should go to 0)
	err = scheduler.ReduceAdminQueue(ctx, 1, 10)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify the queue is 0
	if mock.users[0].AdminQueueDays != 0 {
		t.Errorf("Expected 0 admin queue days, got %d", mock.users[0].AdminQueueDays)
	}
}

func TestScheduler_SetOffDuty(t *testing.T) {
	mock := newMockStore()
	scheduler := NewScheduler(mock)
	ctx := context.Background()

	start := time.Now()
	end := start.Add(7 * 24 * time.Hour)

	err := scheduler.SetOffDuty(ctx, 1, start, end)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify off-duty was set
	if mock.users[0].OffDutyStart == nil || mock.users[0].OffDutyEnd == nil {
		t.Error("Expected off-duty dates to be set")
	}
}

func TestScheduler_SetOffDuty_InvalidDates(t *testing.T) {
	mock := newMockStore()
	scheduler := NewScheduler(mock)
	ctx := context.Background()

	start := time.Now()
	end := start.Add(-7 * 24 * time.Hour) // End before start

	err := scheduler.SetOffDuty(ctx, 1, start, end)
	if err == nil {
		t.Fatal("Expected error for invalid date range, got nil")
	}
}

func TestScheduler_ClearOffDuty(t *testing.T) {
	mock := newMockStore()
	scheduler := NewScheduler(mock)
	ctx := context.Background()

	// First set off-duty
	start := time.Now()
	end := start.Add(7 * 24 * time.Hour)
	scheduler.SetOffDuty(ctx, 1, start, end)

	// Then clear it
	err := scheduler.ClearOffDuty(ctx, 1)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify off-duty was cleared
	if mock.users[0].OffDutyStart != nil || mock.users[0].OffDutyEnd != nil {
		t.Error("Expected off-duty dates to be cleared")
	}
}

func TestScheduler_ChangeDutyUser(t *testing.T) {
	mock := newMockStore()
	scheduler := NewScheduler(mock)
	ctx := context.Background()

	// Create a duty for tomorrow
	tomorrow := time.Now().Add(24 * time.Hour).Truncate(24 * time.Hour)
	existingDuty := &store.Duty{
		ID:             1,
		UserID:         1,
		DutyDate:       tomorrow,
		AssignmentType: store.AssignmentTypeRoundRobin,
	}
	mock.duties[tomorrow.Format("2006-01-02")] = existingDuty

	// Change to user 2
	updatedDuty, err := scheduler.ChangeDutyUser(ctx, tomorrow, 2)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if updatedDuty.UserID != 2 {
		t.Errorf("Expected UserID to be 2, got %d", updatedDuty.UserID)
	}
}

func TestScheduler_ChangeDutyUser_PastDate(t *testing.T) {
	mock := newMockStore()
	scheduler := NewScheduler(mock)
	ctx := context.Background()

	// Try to change a past duty
	yesterday := time.Now().Add(-24 * time.Hour).Truncate(24 * time.Hour)
	existingDuty := &store.Duty{
		ID:             1,
		UserID:         1,
		DutyDate:       yesterday,
		AssignmentType: store.AssignmentTypeRoundRobin,
	}
	mock.duties[yesterday.Format("2006-01-02")] = existingDuty

	_, err := scheduler.ChangeDutyUser(ctx, yesterday, 2)
	if err == nil {
		t.Fatal("Expected error when changing past duty, got nil")
	}
}