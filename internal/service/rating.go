package service

import (
	"context"
	"time"

	"github.com/korjavin/dutyassistant/internal/domain"
)

type RatingServiceImpl struct {
	repo domain.Repository
}

func NewRatingService(repo domain.Repository) *RatingServiceImpl {
	return &RatingServiceImpl{repo: repo}
}

func (s *RatingServiceImpl) SubmitDailyRatings(ctx context.Context, date time.Time, scores map[int64]int) error {
	var ratings []*domain.ParticipantDailyRating
	for userID, score := range scores {
		user, err := s.repo.GetUserByTelegramID(ctx, userID)
		if err != nil {
			return err
		}
		ratings = append(ratings, &domain.ParticipantDailyRating{
			ParticipantID:   user.ID,
			ParticipantName: user.FirstName,
			RatingDate:      date,
			Score:           score,
		})
	}
	return s.repo.SaveDailyParticipantRatings(ctx, date, ratings)
}

func (s *RatingServiceImpl) GetMonthlyRatingsCalendar(ctx context.Context, year int, month time.Month) ([]*domain.ParticipantDailyRating, error) {
	now := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	return s.repo.GetCurrentMonthParticipantRatings(ctx, now)
}

func (s *RatingServiceImpl) GetMonthlyWinners(ctx context.Context, year int, month time.Month) ([]*domain.ParticipantMonthlyTotal, error) {
	return s.repo.GetMonthlyParticipantTotals(ctx, year, month)
}
