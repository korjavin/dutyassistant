package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"
)

// mockStore is a mock implementation of the Store interface for testing.
type mockStore struct {
	duties               map[string]*Duty
	users                []*User
	nextRoundRobinUser   *User
	roundRobinError      error
	assignmentCountError error
	findUserError        error
}

// newMockStore initializes a new mockStore.
func newMockStore() *mockStore {
	return &mockStore{
		duties: make(map[string]*Duty),
		users: []*User{
			{ID: 1, FirstName: "Alice", IsActive: true},
			{ID: 2, FirstName: "Bob", IsActive: true, IsAdmin: true},
			{ID: 3, FirstName: "Charlie", IsActive: false},
		},
	}
}

func (m *mockStore) GetDutyByDate(ctx context.Context, date time.Time) (*Duty, error) {
	key := date.Format("2006-01-02")
	duty, exists := m.duties[key]
	if !exists {
		return nil, errors.New("not found") // Simulate not found error
	}
	return duty, nil
}

func (m *mockStore) GetNextRoundRobinUser(ctx context.Context) (*User, error) {
	if m.roundRobinError != nil {
		return nil, m.roundRobinError
	}
	return m.nextRoundRobinUser, nil
}

func (m *mockStore) CreateDuty(ctx context.Context, duty *Duty) error {
	key := duty.DutyDate.Format("2006-01-02")
	if _, exists := m.duties[key]; exists {
		return errors.New("duty already exists on this date")
	}
	duty.ID = len(m.duties) + 1
	m.duties[key] = duty
	return nil
}

func (m *mockStore) UpdateDuty(ctx context.Context, duty *Duty) error {
	key := duty.DutyDate.Format("2006-01-02")
	m.duties[key] = duty
	return nil
}

func (m *mockStore) FindUserByName(ctx context.Context, name string) (*User, error) {
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

func (m *mockStore) IncrementAssignmentCount(ctx context.Context, userID int) error {
	return m.assignmentCountError
}

func TestScheduler_AssignDutyAdmin(t *testing.T) {
	store := newMockStore()
	scheduler := NewScheduler(store)
	ctx := context.Background()

	user := &User{ID: 1, FirstName: "Alice"}
	date := time.Now().UTC().Truncate(24 * time.Hour)

	// Admin assigns a new duty
	duty, err := scheduler.AssignDutyAdmin(ctx, user, date)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if duty.UserID != user.ID || duty.AssignmentType != AssignmentTypeAdmin {
		t.Errorf("Duty was not assigned correctly. Got %+v", duty)
	}

	// Admin overrides a voluntary assignment
	store.duties[date.Format("2006-01-02")] = &Duty{ID: 1, UserID: 2, DutyDate: date, AssignmentType: AssignmentTypeVoluntary}
	duty, err = scheduler.AssignDutyAdmin(ctx, user, date)
	if err != nil {
		t.Fatalf("Expected no error on override, got %v", err)
	}
	if duty.UserID != user.ID || duty.AssignmentType != AssignmentTypeAdmin {
		t.Errorf("Duty was not overridden correctly. Got %+v", duty)
	}
}

func TestScheduler_AssignDutyVoluntary(t *testing.T) {
	store := newMockStore()
	scheduler := NewScheduler(store)
	ctx := context.Background()

	user := &User{ID: 1, FirstName: "Alice"}
	date := time.Now().UTC().Truncate(24 * time.Hour)

	// Volunteer for a free date
	duty, err := scheduler.AssignDutyVoluntary(ctx, user, date)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if duty.UserID != user.ID || duty.AssignmentType != AssignmentTypeVoluntary {
		t.Errorf("Duty was not assigned correctly. Got %+v", duty)
	}

	// Attempt to override an admin assignment (should fail)
	adminDuty := &Duty{ID: 2, UserID: 99, DutyDate: date, AssignmentType: AssignmentTypeAdmin}
	store.duties[date.Format("2006-01-02")] = adminDuty
	_, err = scheduler.AssignDutyVoluntary(ctx, user, date)
	if err == nil {
		t.Fatal("Expected an error when trying to override an admin assignment, but got none")
	}
}

func TestScheduler_AssignDutyRoundRobin(t *testing.T) {
	store := newMockStore()
	scheduler := NewScheduler(store)
	ctx := context.Background()

	nextUser := &User{ID: 3, FirstName: "Charlie"}
	store.nextRoundRobinUser = nextUser
	date := time.Now().UTC().Truncate(24 * time.Hour)

	// Assign duty for a free date
	duty, err := scheduler.AssignDutyRoundRobin(ctx, date)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if duty.UserID != nextUser.ID || duty.AssignmentType != AssignmentTypeRoundRobin {
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
	store.assignmentCountError = errors.New("db error")
	date = date.Add(24 * time.Hour) // New date
	_, err = scheduler.AssignDutyRoundRobin(ctx, date)
	if err == nil {
		t.Fatal("Expected an error when incrementing count fails, but got none")
	}
}