package service

import (
	"context"
	"testing"
	"time"

	"github.com/korjavin/dutyassistant/internal/domain"
)

type mockRatingRepo struct {
	domain.Repository
	ratingsSaved bool
	savedRatings []*domain.ParticipantDailyRating
}

func (m *mockRatingRepo) GetUserByTelegramID(ctx context.Context, id int64) (*domain.User, error) {
	return &domain.User{ID: id, FirstName: "TestUser"}, nil
}

func (m *mockRatingRepo) SaveDailyParticipantRatings(ctx context.Context, date time.Time, ratings []*domain.ParticipantDailyRating) error {
	m.ratingsSaved = true
	m.savedRatings = ratings
	return nil
}

func TestRatingService_SubmitDailyRatings(t *testing.T) {
	repo := &mockRatingRepo{}
	service := NewRatingService(repo)

	scores := map[int64]int{
		123: 5,
	}

	err := service.SubmitDailyRatings(context.Background(), time.Now(), scores)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !repo.ratingsSaved {
		t.Error("expected ratings to be saved in repo")
	}

	if len(repo.savedRatings) != 1 {
		t.Errorf("expected 1 rating to be saved, got %d", len(repo.savedRatings))
	}

	if repo.savedRatings[0].Score != 5 {
		t.Errorf("expected score 5, got %d", repo.savedRatings[0].Score)
	}
}
