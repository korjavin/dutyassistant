package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/korjavin/dutyassistant/internal/store/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestExplainLastAssignment_NoDuties(t *testing.T) {
	mockStore := new(mocks.MockStore)
	s := NewScheduler(mockStore)
	ctx := context.Background()

	mockStore.On("GetLastDuty", ctx).Return(nil, nil)

	explanation, err := s.ExplainLastAssignment(ctx)

	assert.NoError(t, err)
	assert.Equal(t, "Нет данных о последних назначениях.", explanation)
	mockStore.AssertExpectations(t)
}

func TestExplainLastAssignment_RoundRobin(t *testing.T) {
	mockStore := new(mocks.MockStore)
	s := NewScheduler(mockStore)
	ctx := context.Background()

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	u1 := &store.User{ID: 1, FirstName: "alex", IsActive: true}
	u2 := &store.User{ID: 2, FirstName: "maria", IsActive: true}
	u3 := &store.User{ID: 3, FirstName: "den", IsActive: true}

	lastDuty := &store.Duty{
		ID:             1,
		UserID:         1,
		User:           u1,
		DutyDate:       today,
		AssignmentType: store.AssignmentTypeRoundRobin,
	}

	duties := []*store.Duty{
		{UserID: 3, AssignmentType: store.AssignmentTypeRoundRobin, DutyDate: today.AddDate(0, 0, -1)},
	}

	mockStore.On("GetLastDuty", ctx).Return(lastDuty, nil)
	mockStore.On("ListActiveUsers", ctx).Return([]*store.User{u1, u2, u3}, nil)
	mockStore.On("GetCompletedDutiesInRange", ctx, mock.Anything, mock.Anything).Return(duties, nil)

	// User off duty checks
	mockStore.On("IsUserOffDuty", ctx, int64(1), today).Return(false, nil)
	mockStore.On("IsUserOffDuty", ctx, int64(2), today).Return(true, nil) // Maria is off duty
	mockStore.On("IsUserOffDuty", ctx, int64(3), today).Return(false, nil)

    // For calculating volunteer / admin available candidates
    mockStore.On("GetUsersWithVolunteerQueue", ctx).Return([]*store.User{}, nil)
    mockStore.On("GetUsersWithAdminQueue", ctx).Return([]*store.User{}, nil)

	explanation, err := s.ExplainLastAssignment(ctx)

	assert.NoError(t, err)
	assert.Contains(t, explanation, "Последнее назначение: @alex")
	assert.Contains(t, explanation, "Кандидаты: @alex, @den, @maria")
	assert.Contains(t, explanation, "@maria — отсутствует по расписанию")
	assert.Contains(t, explanation, "@den — 1 дежурств за последние 14 дней (минимум 0)")
	assert.Contains(t, explanation, "Оставшиеся кандидаты: @alex")
	assert.Contains(t, explanation, "Итог: назначен @alex, так как имел наименьшее число дежурств за 14 дней (tie-break случайный при равенстве).")
}

func TestExplainLastAssignment_Volunteer(t *testing.T) {
	mockStore := new(mocks.MockStore)
	s := NewScheduler(mockStore)
	ctx := context.Background()

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	u1 := &store.User{ID: 1, FirstName: "alex", IsActive: true, VolunteerQueueDays: 1}
	u2 := &store.User{ID: 2, FirstName: "maria", IsActive: true}

	lastDuty := &store.Duty{
		ID:             1,
		UserID:         1,
		User:           u1,
		DutyDate:       today,
		AssignmentType: store.AssignmentTypeVoluntary,
	}

	mockStore.On("GetLastDuty", ctx).Return(lastDuty, nil)
	mockStore.On("ListActiveUsers", ctx).Return([]*store.User{u1, u2}, nil)
	mockStore.On("GetCompletedDutiesInRange", ctx, mock.Anything, mock.Anything).Return([]*store.Duty{}, nil)

	mockStore.On("IsUserOffDuty", ctx, int64(1), today).Return(false, nil)
	mockStore.On("IsUserOffDuty", ctx, int64(2), today).Return(false, nil)

	explanation, err := s.ExplainLastAssignment(ctx)

	assert.NoError(t, err)
	assert.Contains(t, explanation, "Последнее назначение: @alex")
	assert.Contains(t, explanation, "Кандидаты: @alex, @maria")
	assert.Contains(t, explanation, "Оставшиеся кандидаты: @alex")
	assert.Contains(t, explanation, "доброволец с наивысшим приоритетом")
}

func TestExplainLastAssignment_Admin(t *testing.T) {
	mockStore := new(mocks.MockStore)
	s := NewScheduler(mockStore)
	ctx := context.Background()

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	u1 := &store.User{ID: 1, FirstName: "alex", IsActive: true, AdminQueueDays: 1}
	u2 := &store.User{ID: 2, FirstName: "maria", IsActive: true}

	lastDuty := &store.Duty{
		ID:             1,
		UserID:         1,
		User:           u1,
		DutyDate:       today,
		AssignmentType: store.AssignmentTypeAdmin,
	}

	mockStore.On("GetLastDuty", ctx).Return(lastDuty, nil)
	mockStore.On("ListActiveUsers", ctx).Return([]*store.User{u1, u2}, nil)
	mockStore.On("GetCompletedDutiesInRange", ctx, mock.Anything, mock.Anything).Return([]*store.Duty{}, nil)

	mockStore.On("IsUserOffDuty", ctx, int64(1), today).Return(false, nil)
	mockStore.On("IsUserOffDuty", ctx, int64(2), today).Return(false, nil)

	explanation, err := s.ExplainLastAssignment(ctx)

	assert.NoError(t, err)
	assert.Contains(t, explanation, "Последнее назначение: @alex")
	assert.Contains(t, explanation, "Кандидаты: @alex, @maria")
	assert.Contains(t, explanation, "Оставшиеся кандидаты: @alex")
	assert.Contains(t, explanation, "назначен администратором")
}
