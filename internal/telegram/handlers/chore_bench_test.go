package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/korjavin/dutyassistant/internal/store/mocks"
)

// BenchmarkOffDutyFiltering compares the N+1 query method vs the batch optimized method
func BenchmarkOffDutyFiltering(b *testing.B) {
	mockStore := new(mocks.MockStore)
	now := time.Now()
	checkDate := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, now.Location())

	// Setup mock data
	var users []*store.User
	for i := 0; i < 1000; i++ {
		users = append(users, &store.User{ID: int64(i)})
	}

	// Fast version mock setup
	offDutyUsers := []*store.User{
		{ID: 10}, {ID: 50}, {ID: 100},
	}
	mockStore.On("GetOffDutyUsers", context.Background(), checkDate).Return(offDutyUsers, nil)

	// Slow version mock setup
	for i := 0; i < 1000; i++ {
		isOff := false
		if i == 10 || i == 50 || i == 100 {
			isOff = true
		}
		mockStore.On("IsUserOffDuty", context.Background(), int64(i), checkDate).Return(isOff, nil)
	}

	b.Run("Optimized_Batch", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			offDutyUsers, _ := mockStore.GetOffDutyUsers(context.Background(), checkDate)
			offDutyMap := make(map[int64]bool)
			for _, u := range offDutyUsers {
				offDutyMap[u.ID] = true
			}

			var candidates []*store.User
			for _, u := range users {
				if !offDutyMap[u.ID] {
					candidates = append(candidates, u)
				}
			}
			_ = candidates
		}
	})

	b.Run("Unoptimized_NPlus1", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var candidates []*store.User
			for _, u := range users {
				isOff, _ := mockStore.IsUserOffDuty(context.Background(), u.ID, checkDate)
				if !isOff {
					candidates = append(candidates, u)
				}
			}
			_ = candidates
		}
	})
}
