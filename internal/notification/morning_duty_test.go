package notification

import (
	"testing"
	"time"

	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/korjavin/dutyassistant/internal/store/mocks"
	"github.com/stretchr/testify/require"
)

func TestReadMorningDutyContextCountsDistinctFutureDates(t *testing.T) {
	ctx := t.Context()
	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)
	// SQLite represents calendar labels as UTC midnight.
	today := time.Date(2026, time.March, 30, 0, 0, 0, 0, time.UTC)
	duty := &store.Duty{UserID: 1, DutyDate: today, AssignmentType: store.AssignmentTypeAdmin}
	reader := &mocks.MockStore{}
	reader.On("GetUserByID", ctx, int64(1)).Return(&store.User{FirstName: "Ana", VolunteerQueueDays: 3, AdminQueueDays: 2}, nil).Once()
	reader.On("GetDutyByDate", ctx, time.Date(2026, time.March, 29, 0, 0, 0, 0, berlin)).Return(&store.Duty{UserID: 1}, nil).Once()
	reader.On("GetDutyByDate", ctx, time.Date(2026, time.March, 28, 0, 0, 0, 0, berlin)).Return(&store.Duty{UserID: 2}, nil).Once()
	reader.On("GetFutureDuties", ctx, time.Date(2026, time.March, 31, 0, 0, 0, 0, berlin)).Return([]*store.Duty{
		duty,
		{UserID: 1, DutyDate: today.AddDate(0, 0, -1)},
		{UserID: 1, DutyDate: today.AddDate(0, 0, 1), AssignmentType: store.AssignmentTypeAdmin},
		{UserID: 1, DutyDate: today.AddDate(0, 0, 1), AssignmentType: store.AssignmentTypeVoluntary},
		{UserID: 2, DutyDate: today.AddDate(0, 0, 2)},
		{UserID: 1, DutyDate: today.AddDate(0, 0, 3), AssignmentType: store.AssignmentTypeRoundRobin},
	}, nil).Once()

	facts, err := ReadMorningDutyContext(ctx, reader, duty, berlin)
	require.NoError(t, err)
	require.Equal(t, &MorningDutyContext{Name: "Ana", Reason: store.AssignmentTypeAdmin, DaysBehind: 1, DaysAhead: 7}, facts)
	reader.AssertExpectations(t)
}
