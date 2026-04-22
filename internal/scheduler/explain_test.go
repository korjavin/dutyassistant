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
	assert.Contains(t, explanation, "@den — нагрузка 1 за год (минимум 0)")
	assert.Contains(t, explanation, "Оставшиеся кандидаты: @alex")
	assert.Contains(t, explanation, "Итог: назначен @alex, так как имел наименьшую нагрузку дежурств за год (добровольные дни × 1.1, админ-назначения не учитываются; tie-break случайный при равенстве).")
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

// Verifies the weighted round-robin fairness rules: admin assignments are
// excluded entirely, voluntary days weigh 1.1× round-robin days, and the
// lookback window covers a full year.
func TestExplainLastAssignment_WeightedLoad(t *testing.T) {
	mockStore := new(mocks.MockStore)
	s := NewScheduler(mockStore)
	ctx := context.Background()

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	alex := &store.User{ID: 1, FirstName: "alex", IsActive: true}
	maria := &store.User{ID: 2, FirstName: "maria", IsActive: true}
	den := &store.User{ID: 3, FirstName: "den", IsActive: true}

	// alex was just assigned (round-robin). We construct a duty history that
	// exercises all three weight cases within the 365-day lookback:
	//   - maria has 10 voluntary days  -> load = 10 * 11 = 110 (11.0)
	//   - den has 11 round-robin days  -> load = 11 * 10 = 110 (11.0)
	//   - alex has 12 round-robin days -> load = 12 * 10 = 120 (12.0)
	//   - den also has 50 admin days   -> load contribution = 0 (ignored)
	// So alex is excluded (max load), and both maria and den tie at 11.0,
	// which must be shown in the explanation.
	var duties []*store.Duty
	for i := 0; i < 10; i++ {
		duties = append(duties, &store.Duty{
			UserID:         maria.ID,
			AssignmentType: store.AssignmentTypeVoluntary,
			DutyDate:       today.AddDate(0, 0, -(i + 1)),
		})
	}
	for i := 0; i < 11; i++ {
		duties = append(duties, &store.Duty{
			UserID:         den.ID,
			AssignmentType: store.AssignmentTypeRoundRobin,
			DutyDate:       today.AddDate(0, 0, -(i + 20)),
		})
	}
	for i := 0; i < 12; i++ {
		duties = append(duties, &store.Duty{
			UserID:         alex.ID,
			AssignmentType: store.AssignmentTypeRoundRobin,
			DutyDate:       today.AddDate(0, 0, -(i + 40)),
		})
	}
	for i := 0; i < 50; i++ {
		duties = append(duties, &store.Duty{
			UserID:         den.ID,
			AssignmentType: store.AssignmentTypeAdmin,
			DutyDate:       today.AddDate(0, 0, -(i + 60)),
		})
	}
	lastDuty := &store.Duty{
		ID:             1,
		UserID:         alex.ID,
		User:           alex,
		DutyDate:       today,
		AssignmentType: store.AssignmentTypeRoundRobin,
	}

	mockStore.On("GetLastDuty", ctx).Return(lastDuty, nil)
	mockStore.On("ListActiveUsers", ctx).Return([]*store.User{alex, maria, den}, nil)

	// Verify the lookback window is ~365 days, independently of what we return.
	mockStore.On("GetCompletedDutiesInRange", ctx, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			start := args.Get(1).(time.Time)
			end := args.Get(2).(time.Time)
			if days := int(end.Sub(start).Hours() / 24); days < 364 || days > 366 {
				t.Errorf("expected ~365 day lookback, got %d", days)
			}
		}).
		Return(duties, nil)
	mockStore.On("GetOffDutyUsers", ctx, today).Return([]*store.User{}, nil)

	explanation, err := s.ExplainLastAssignment(ctx)

	assert.NoError(t, err)
	// alex's 12 round-robin days = load 12.0, above the minimum of 11.0.
	assert.Contains(t, explanation, "@alex — нагрузка 12 за год (минимум 11)")
	// maria and den tie at the minimum load of 11.0 — both remain as candidates.
	assert.Contains(t, explanation, "Оставшиеся кандидаты: @den, @maria")
}
