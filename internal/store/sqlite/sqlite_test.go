package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/korjavin/dutyassistant/internal/store"
)

// setupTestDB creates a new in-memory SQLite database for testing.
func setupTestDB(t *testing.T) *SQLiteStore {
	t.Helper()
	ctx := context.Background()
	// Using ":memory:" creates a temporary, in-memory database.
	// Using "?_pragma=foreign_keys(1)" ensures foreign key constraints are enforced.
	db, err := New(ctx, ":memory:?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	return db
}

func TestUserLifecycle(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	// 1. Create User
	user := &store.User{
		TelegramUserID: 12345,
		FirstName:      "John Doe",
		IsAdmin:        false,
		IsActive:       true,
	}
	err := s.CreateUser(ctx, user)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	if user.ID == 0 {
		t.Fatal("Expected user ID to be set, but it was 0")
	}

	// 2. Get User
	retrievedUser, err := s.GetUserByTelegramID(ctx, 12345)
	if err != nil {
		t.Fatalf("GetUserByTelegramID failed: %v", err)
	}
	if retrievedUser == nil {
		t.Fatal("Expected to retrieve a user, but got nil")
	}
	if retrievedUser.FirstName != "John Doe" {
		t.Errorf("Expected user first name to be 'John Doe', got '%s'", retrievedUser.FirstName)
	}

	// 3. Update User
	retrievedUser.IsActive = false
	retrievedUser.FirstName = "John D."
	err = s.UpdateUser(ctx, retrievedUser)
	if err != nil {
		t.Fatalf("UpdateUser failed: %v", err)
	}

	updatedUser, _ := s.GetUserByTelegramID(ctx, 12345)
	if updatedUser.IsActive != false {
		t.Error("Expected user to be inactive, but they are active")
	}
	if updatedUser.FirstName != "John D." {
		t.Errorf("Expected user first name to be 'John D.', got '%s'", updatedUser.FirstName)
	}

	// 4. List Active Users
	activeUsers, err := s.ListActiveUsers(ctx)
	if err != nil {
		t.Fatalf("ListActiveUsers failed: %v", err)
	}
	if len(activeUsers) != 0 {
		t.Errorf("Expected 0 active users, but got %d", len(activeUsers))
	}

	// Make user active again and check list
	updatedUser.IsActive = true
	s.UpdateUser(ctx, updatedUser)
	activeUsers, _ = s.ListActiveUsers(ctx)
	if len(activeUsers) != 1 {
		t.Errorf("Expected 1 active user, but got %d", len(activeUsers))
	}
}

func TestDutyLifecycle(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	// Prerequisite: Create a user
	user := &store.User{TelegramUserID: 54321, FirstName: "Jane Doe", IsActive: true}
	if err := s.CreateUser(ctx, user); err != nil {
		t.Fatalf("Failed to create user for duty test: %v", err)
	}

	dutyDate := time.Date(2023, 10, 27, 0, 0, 0, 0, time.UTC)
	createdAt := time.Now()

	// 1. Create Duty
	duty := &store.Duty{
		UserID:         user.ID,
		DutyDate:       dutyDate,
		AssignmentType: "voluntary",
		CreatedAt:      createdAt,
	}
	err := s.CreateDuty(ctx, duty)
	if err != nil {
		t.Fatalf("CreateDuty failed: %v", err)
	}
	if duty.ID == 0 {
		t.Fatal("Expected duty ID to be set, but it was 0")
	}

	// 2. Get Duty
	retrievedDuty, err := s.GetDutyByDate(ctx, dutyDate)
	if err != nil {
		t.Fatalf("GetDutyByDate failed: %v", err)
	}
	if retrievedDuty == nil {
		t.Fatal("Expected to retrieve a duty, but got nil")
	}
	if retrievedDuty.AssignmentType != "voluntary" {
		t.Errorf("Expected duty type to be 'voluntary', got '%s'", retrievedDuty.AssignmentType)
	}
	if retrievedDuty.User == nil || retrievedDuty.User.FirstName != "Jane Doe" {
		t.Errorf("Expected duty user to be 'Jane Doe', got '%v'", retrievedDuty.User)
	}

	// 3. Update Duty
	retrievedDuty.AssignmentType = "admin"
	err = s.UpdateDuty(ctx, retrievedDuty)
	if err != nil {
		t.Fatalf("UpdateDuty failed: %v", err)
	}
	updatedDuty, _ := s.GetDutyByDate(ctx, dutyDate)
	if updatedDuty.AssignmentType != "admin" {
		t.Errorf("Expected updated duty type to be 'admin', got '%s'", updatedDuty.AssignmentType)
	}

	// 4. Get Duties By Month
	duties, err := s.GetDutiesByMonth(ctx, 2023, time.October)
	if err != nil {
		t.Fatalf("GetDutiesByMonth failed: %v", err)
	}
	if len(duties) != 1 {
		t.Errorf("Expected 1 duty in October, got %d", len(duties))
	}

	// 5. Delete Duty
	err = s.DeleteDuty(ctx, dutyDate)
	if err != nil {
		t.Fatalf("DeleteDuty failed: %v", err)
	}
	deletedDuty, _ := s.GetDutyByDate(ctx, dutyDate)
	if deletedDuty != nil {
		t.Error("Expected duty to be deleted, but it was found")
	}
}

func TestGetDutiesByMonth_OrphanDutyRow(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	// Simulate legacy/orphan data where duty row exists but linked user is missing.
	if _, err := s.db.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
		t.Fatalf("Failed to disable foreign keys: %v", err)
	}

	createdAt := time.Now().UTC().Format(time.RFC3339)
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO duties (user_id, duty_date, assignment_type, created_at, completed_at) VALUES (?, ?, ?, ?, ?)`,
		int64(999999), "2026-02-14", "round_robin", createdAt, nil,
	); err != nil {
		t.Fatalf("Failed to insert orphan duty: %v", err)
	}

	if _, err := s.db.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
		t.Fatalf("Failed to re-enable foreign keys: %v", err)
	}

	duties, err := s.GetDutiesByMonth(ctx, 2026, time.February)
	if err != nil {
		t.Fatalf("GetDutiesByMonth failed for orphan duty row: %v", err)
	}
	if len(duties) != 1 {
		t.Fatalf("Expected 1 duty in February, got %d", len(duties))
	}
	if duties[0].User != nil {
		t.Fatalf("Expected duty user to be nil for orphan row, got %+v", duties[0].User)
	}
}

func TestRoundRobin(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	// Create users
	user1 := &store.User{TelegramUserID: 1, FirstName: "User1", IsActive: true}
	user2 := &store.User{TelegramUserID: 2, FirstName: "User2", IsActive: true}
	user3 := &store.User{TelegramUserID: 3, FirstName: "User3", IsActive: false} // Inactive
	s.CreateUser(ctx, user1)
	s.CreateUser(ctx, user2)
	s.CreateUser(ctx, user3)

	// Test the new queue-based system
	// 1. Add users to volunteer queue
	err := s.AddToVolunteerQueue(ctx, user1.ID, 2)
	if err != nil {
		t.Fatalf("AddToVolunteerQueue failed: %v", err)
	}

	// 2. Get users with volunteer queue
	volunteers, err := s.GetUsersWithVolunteerQueue(ctx)
	if err != nil {
		t.Fatalf("GetUsersWithVolunteerQueue failed: %v", err)
	}
	if len(volunteers) != 1 || volunteers[0].ID != user1.ID {
		t.Errorf("Expected 1 volunteer (user1), got %d volunteers", len(volunteers))
	}

	// 3. Decrement volunteer queue
	err = s.DecrementVolunteerQueue(ctx, user1.ID)
	if err != nil {
		t.Fatalf("DecrementVolunteerQueue failed: %v", err)
	}

	// 4. Add to admin queue
	err = s.AddToAdminQueue(ctx, user2.ID, 1)
	if err != nil {
		t.Fatalf("AddToAdminQueue failed: %v", err)
	}

	// 5. Get users with admin queue
	admins, err := s.GetUsersWithAdminQueue(ctx)
	if err != nil {
		t.Fatalf("GetUsersWithAdminQueue failed: %v", err)
	}
	if len(admins) != 1 || admins[0].ID != user2.ID {
		t.Errorf("Expected 1 admin queue user (user2), got %d users", len(admins))
	}
}

func TestChoreCancellation(t *testing.T) {
	s := setupTestDB(t)
	ctx := context.Background()

	// Prerequisite: Create a user
	user := &store.User{TelegramUserID: 112233, FirstName: "Chore Worker", IsActive: true}
	if err := s.CreateUser(ctx, user); err != nil {
		t.Fatalf("Failed to create user for chore test: %v", err)
	}

	// 1. Create a Chore
	chore := &store.Chore{
		UserID:      user.ID,
		Description: "Take out the trash",
		AssignedAt:  time.Now(),
		DeadlineAt:  time.Now().Add(24 * time.Hour),
		ReminderID:  "reminder-123",
	}
	err := s.CreateChore(ctx, chore)
	if err != nil {
		t.Fatalf("CreateChore failed: %v", err)
	}
	if chore.ID == 0 {
		t.Fatal("Expected chore ID to be set")
	}

	// 2. List Active Chores
	activeChores, err := s.ListActiveChores(ctx)
	if err != nil {
		t.Fatalf("ListActiveChores failed: %v", err)
	}
	if len(activeChores) != 1 {
		t.Errorf("Expected 1 active chore, got %d", len(activeChores))
	}

	// 3. Cancel the Chore
	err = s.CancelChore(ctx, chore.ID)
	if err != nil {
		t.Fatalf("CancelChore failed: %v", err)
	}

	// 4. Verify it's no longer active
	activeChoresAfter, err := s.ListActiveChores(ctx)
	if err != nil {
		t.Fatalf("ListActiveChores after cancel failed: %v", err)
	}
	if len(activeChoresAfter) != 0 {
		t.Errorf("Expected 0 active chores after cancellation, got %d", len(activeChoresAfter))
	}

	// 5. Try to cancel again (should fail)
	err = s.CancelChore(ctx, chore.ID)
	if err == nil {
		t.Errorf("Expected an error when cancelling an already cancelled chore")
	}
}
