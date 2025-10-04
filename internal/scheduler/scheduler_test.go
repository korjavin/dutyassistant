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

func (m *mockStore) GetDutyByDate(ctx context.Context, date string) (*store.Duty, error) {
	duty, exists := m.duties[date]
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
	if _, exists := m.duties[duty.DutyDate]; exists {
		return errors.New("duty already exists on this date")
	}
	duty.ID = int64(len(m.duties) + 1)
	m.duties[duty.DutyDate] = duty
	return nil
}

func (m *mockStore) UpdateDuty(ctx context.Context, duty *store.Duty) error {
	m.duties[duty.DutyDate] = duty
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

func (m *mockStore) IncrementAssignmentCount(ctx context.Context, userID int64) error {
	return m.assignmentCountError
}

func TestScheduler_AssignDutyAdmin(t *testing.T) {
	mock := newMockStore()
	scheduler := NewScheduler(mock)
	ctx := context.Background()

	user := &store.User{ID: 1, FirstName: "Alice"}
	date := time.Now().UTC().Truncate(24 * time.Hour)
	dateStr := date.Format(dateLayout)

	// Admin assigns a new duty
	duty, err := scheduler.AssignDutyAdmin(ctx, user, date)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if duty.UserID != user.ID || duty.AssignmentType != store.AssignmentTypeAdmin {
		t.Errorf("Duty was not assigned correctly. Got %+v", duty)
	}

	// Admin overrides a voluntary assignment
	mock.duties[dateStr] = &store.Duty{ID: 1, UserID: 2, DutyDate: dateStr, AssignmentType: store.AssignmentTypeVoluntary}
	duty, err = scheduler.AssignDutyAdmin(ctx, user, date)
	if err != nil {
		t.Fatalf("Expected no error on override, got %v", err)
	}
	if duty.UserID != user.ID || duty.AssignmentType != store.AssignmentTypeAdmin {
		t.Errorf("Duty was not overridden correctly. Got %+v", duty)
	}
}

func TestScheduler_AssignDutyVoluntary(t *testing.T) {
	mock := newMockStore()
	scheduler := NewScheduler(mock)
	ctx := context.Background()

	user := &store.User{ID: 1, FirstName: "Alice"}
	date := time.Now().UTC().Truncate(24 * time.Hour)
	dateStr := date.Format(dateLayout)

	// Volunteer for a free date
	duty, err := scheduler.AssignDutyVoluntary(ctx, user, date)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if duty.UserID != user.ID || duty.AssignmentType != store.AssignmentTypeVoluntary {
		t.Errorf("Duty was not assigned correctly. Got %+v", duty)
	}

	// Attempt to override an admin assignment (should fail)
	adminDuty := &store.Duty{ID: 2, UserID: 99, DutyDate: dateStr, AssignmentType: store.AssignmentTypeAdmin}
	mock.duties[dateStr] = adminDuty
	_, err = scheduler.AssignDutyVoluntary(ctx, user, date)
	if err == nil {
		t.Fatal("Expected an error when trying to override an admin assignment, but got none")
	}
}

func TestScheduler_AssignDutyRoundRobin(t *testing.T) {
	mock := newMockStore()
	scheduler := NewScheduler(mock)
	ctx := context.Background()

	nextUser := &store.User{ID: 3, FirstName: "Charlie"}
	mock.nextRoundRobinUser = nextUser
	date := time.Now().UTC().Truncate(24 * time.Hour)

	// Assign duty for a free date
	duty, err := scheduler.AssignDutyRoundRobin(ctx, date)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if duty.UserID != nextUser.ID || duty.AssignmentType != store.AssignmentTypeRoundRobin {
		t.Errorf("Duty was not assigned correctly. Got %+v", duty)
	}

	// Try to assign again on the same date (should do nothing)
	duty2, err := scheduler.AssignDutyRoundRobin(ctx, date)
	if err != nil {
		t.Fatalf("Expected no error on second call, got %v", err)
	}
	if duty.ID != duty2.ID {
		t.Errorf("Expected the same duty to be returned. Got %+v, want %+v", duty2, duty)
	}

	// Test failure to increment count
	mock.assignmentCountError = errors.New("db error")
	date = date.Add(24 * time.Hour) // New date
	_, err = scheduler.AssignDutyRoundRobin(ctx, date)
	if err == nil {
		t.Fatal("Expected an error when incrementing count fails, but got none")
	}
}