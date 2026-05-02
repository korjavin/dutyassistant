package sqlite_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/korjavin/dutyassistant/internal/store/sqlite"
)

func BenchmarkGetUserByID_Vs_ListAllUsers(b *testing.B) {
	// Setup a uniquely named in-memory database URI to avoid shared cache collisions
	dbURI := fmt.Sprintf("file:bench_%d?mode=memory&cache=shared", time.Now().UnixNano())
	store_db, err := sqlite.New(context.Background(), dbURI)
	if err != nil {
		b.Fatalf("failed to create sqlite store: %v", err)
	}

	ctx := context.Background()

	// Insert 100 users
	for i := 1; i <= 100; i++ {
		user := &store.User{
			TelegramUserID: int64(1000 + i),
			FirstName:      fmt.Sprintf("User%d", i),
			IsActive:       true,
		}
		if err := store_db.CreateUser(ctx, user); err != nil {
			b.Fatalf("failed to insert user: %v", err)
		}
	}

	b.Run("ListAllUsers_Loop", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Simulate the current behavior: find user with ID 50
			users, err := store_db.ListAllUsers(ctx)
			if err != nil {
				b.Fatalf("ListAllUsers failed: %v", err)
			}
			var user *store.User
			for _, u := range users {
				if u.ID == 50 {
					user = u
					break
				}
			}
			if user == nil {
				b.Fatalf("user not found")
			}
		}
	})
}

func BenchmarkGetUserByID_Vs_ListAllUsers_New(b *testing.B) {
	// Setup a uniquely named in-memory database URI to avoid shared cache collisions
	dbURI := fmt.Sprintf("file:bench_new_%d?mode=memory&cache=shared", time.Now().UnixNano())
	store_db, err := sqlite.New(context.Background(), dbURI)
	if err != nil {
		b.Fatalf("failed to create sqlite store: %v", err)
	}

	ctx := context.Background()

	// Insert 100 users
	for i := 1; i <= 100; i++ {
		user := &store.User{
			TelegramUserID: int64(1000 + i),
			FirstName:      fmt.Sprintf("User%d", i),
			IsActive:       true,
		}
		if err := store_db.CreateUser(ctx, user); err != nil {
			b.Fatalf("failed to insert user: %v", err)
		}
	}

	b.Run("GetUserByID", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Expected new behavior
			user, err := store_db.GetUserByID(ctx, 50)
			if err != nil {
				b.Fatalf("GetUserByID failed: %v", err)
			}
			if user == nil {
				b.Fatalf("user not found")
			}
		}
	})
}
