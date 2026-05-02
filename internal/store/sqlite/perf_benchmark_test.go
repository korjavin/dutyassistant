package sqlite

import (
	"context"
	"fmt"
	"testing"

	"github.com/korjavin/dutyassistant/internal/store"
)

// ListAllUsers is the original implementation in handlers
func benchmarkListAllUsers(b *testing.B, s store.Store, targetID int64) {
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		users, _ := s.ListAllUsers(ctx)
		var user *store.User
		for _, u := range users {
			if u.ID == targetID {
				user = u
				break
			}
		}
		_ = user
	}
}

// GetUserByID is the proposed optimized implementation
func benchmarkGetUserByID(b *testing.B, s store.Store, targetID int64) {
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var user *store.User
		query := `SELECT id, telegram_user_id, first_name, is_admin, is_active, volunteer_queue_days, admin_queue_days, off_duty_start, off_duty_end
	          FROM users WHERE id = ?`
		row := s.(*SQLiteStore).db.QueryRowContext(ctx, query, targetID)
		u, _ := scanUser(row)
		user = u
		_ = user
	}
}

func BenchmarkUserRetrieval(b *testing.B) {
	// Instead of full setupTestDB, we can use in memory DB
	db, err := New(context.Background(), "file:bench123?mode=memory&cache=shared")
	if err != nil {
		b.Fatalf("Failed to open DB: %v", err)
	}
	s := db

	// Create some dummy users
	for i := 1; i <= 100; i++ {
		err := s.CreateUser(context.Background(), &store.User{
			TelegramUserID: int64(1000 + i),
			FirstName:      fmt.Sprintf("User%d", i),
		})
		if err != nil {
			b.Fatalf("Failed to create user: %v", err)
		}
	}

	targetID := int64(50)

	b.Run("ListAllUsers_N1", func(b *testing.B) {
		benchmarkListAllUsers(b, s, targetID)
	})

	b.Run("GetUserByID_Optimized", func(b *testing.B) {
		benchmarkGetUserByID(b, s, targetID)
	})
}
