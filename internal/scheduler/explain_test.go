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
	mockStore.On("GetOffDutyUsers", ctx, today).Return([]*store.User{u2}, nil)

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

	mockStore.On("GetOffDutyUsers", ctx, today).Return([]*store.User{}, nil)
	mockStore.On("GetOffDutyUsers", ctx, today).Return([]*store.User{}, nil)

	explanation, err := s.ExplainLastAssignment(ctx)

	assert.NoError(t, err)
	assert.Contains(t, explanation, "Последнее назначение: @alex")
	assert.Contains(t, explanation, "Кандидаты: @alex, @maria")
	assert.Contains(t, explanation, "Оставшиеся кандидаты: @alex")
	assert.Contains(t, explanation, "доброволец с наибольшим количеством дней (tie-break случайный при равенстве).")
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

	mockStore.On("GetOffDutyUsers", ctx, today).Return([]*store.User{}, nil)
	mockStore.On("GetOffDutyUsers", ctx, today).Return([]*store.User{}, nil)

	explanation, err := s.ExplainLastAssignment(ctx)

	assert.NoError(t, err)
	assert.Contains(t, explanation, "Последнее назначение: @alex")
	assert.Contains(t, explanation, "Кандидаты: @alex, @maria")
	assert.Contains(t, explanation, "Оставшиеся кандидаты: @alex")
	assert.Contains(t, explanation, "назначен администратором с наибольшим количеством дней в очереди (tie-break случайный при равенстве).")
}

func TestExplainLastAssignment_PostDecrementZero(t *testing.T) {
	mockStore := new(mocks.MockStore)
	s := NewScheduler(mockStore)
	ctx := context.Background()

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	// User alex now has 0 days because it was decremented during assignment
	u1 := &store.User{ID: 1, FirstName: "alex", IsActive: true, VolunteerQueueDays: 0}
	u2 := &store.User{ID: 2, FirstName: "maria", IsActive: true, VolunteerQueueDays: 0}

	lastDuty := &store.Duty{
		ID:             1,
		UserID:         1, // assigned to alex
		User:           u1,
		DutyDate:       today,
		AssignmentType: store.AssignmentTypeVoluntary,
	}

	mockStore.On("GetLastDuty", ctx).Return(lastDuty, nil)
	mockStore.On("ListActiveUsers", ctx).Return([]*store.User{u1, u2}, nil)
	mockStore.On("GetCompletedDutiesInRange", ctx, mock.Anything, mock.Anything).Return([]*store.Duty{}, nil)

	mockStore.On("GetOffDutyUsers", ctx, today).Return([]*store.User{}, nil)

	explanation, err := s.ExplainLastAssignment(ctx)

	assert.NoError(t, err)
	assert.Contains(t, explanation, "Последнее назначение: @alex")
	assert.Contains(t, explanation, "Кандидаты: @alex, @maria")

	// Maria was excluded because she had 0
	assert.Contains(t, explanation, "@maria — нет дней в очереди")

	// Alex was left in remaining candidates even though current DB count is 0
	assert.Contains(t, explanation, "Оставшиеся кандидаты: @alex")
}

func TestExplainLastAssignment_OffDutyMaxQueue(t *testing.T) {
	mockStore := new(mocks.MockStore)
	s := NewScheduler(mockStore)
	ctx := context.Background()

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	// alex has 1 and is active
	u1 := &store.User{ID: 1, FirstName: "alex", IsActive: true, VolunteerQueueDays: 0}

	// maria has 5, but she is off duty
	u2 := &store.User{ID: 2, FirstName: "maria", IsActive: true, VolunteerQueueDays: 5}

	lastDuty := &store.Duty{
		ID:             1,
		UserID:         1, // assigned to alex
		User:           u1,
		DutyDate:       today,
		AssignmentType: store.AssignmentTypeVoluntary,
	}

	mockStore.On("GetLastDuty", ctx).Return(lastDuty, nil)
	mockStore.On("ListActiveUsers", ctx).Return([]*store.User{u1, u2}, nil)
	mockStore.On("GetCompletedDutiesInRange", ctx, mock.Anything, mock.Anything).Return([]*store.Duty{}, nil)

	mockStore.On("GetOffDutyUsers", ctx, today).Return([]*store.User{u2}, nil)

	explanation, err := s.ExplainLastAssignment(ctx)

	assert.NoError(t, err)
	assert.Contains(t, explanation, "Последнее назначение: @alex")
	assert.Contains(t, explanation, "Кандидаты: @alex, @maria")

	// Maria was excluded because she is off duty
	assert.Contains(t, explanation, "@maria — отсутствует по расписанию")

	// Since maria is off-duty, she doesn't count towards the maxQueueCount (which should be 1 for alex).
	// Alex was left in remaining candidates
	assert.Contains(t, explanation, "Оставшиеся кандидаты: @alex")
}
