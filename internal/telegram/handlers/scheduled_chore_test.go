package handlers_test

import (
	"context"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/mocks"
	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/korjavin/dutyassistant/internal/telegram/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// choreCommand builds an admin /chore message with the given arguments.
func choreCommand(args string) *tgbotapi.Message {
	return &tgbotapi.Message{
		Chat:     &tgbotapi.Chat{ID: 789},
		From:     &tgbotapi.User{ID: 123},
		Text:     "/chore " + args,
		Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 6}},
	}
}

// scheduledChoreSetup freezes the clock at 2026-10-20 14:00 Berlin (inside the
// 10:00-18:00 window, so a plain recurring chore would be assigned at once).
func scheduledChoreSetup(t *testing.T) (*handlers.Handlers, *mocks.MockStore, *time.Location) {
	t.Helper()
	t.Setenv("CHORE_TIMEZONE", "Europe/Berlin")
	berlin, _ := time.LoadLocation("Europe/Berlin")
	handlers.TimeNow = func() time.Time { return time.Date(2026, time.October, 20, 14, 0, 0, 0, berlin) }
	t.Cleanup(func() { handlers.TimeNow = time.Now })

	st := new(mocks.MockStore)
	admin := &store.User{ID: 1, TelegramUserID: 123, FirstName: "Admin", IsAdmin: true, IsActive: true}
	vladimir := &store.User{ID: 7, TelegramUserID: 777, FirstName: "Vladimir", IsActive: true}
	st.On("GetUserByTelegramID", mock.Anything, int64(123)).Return(admin, nil)
	st.On("ListActiveUsers", mock.Anything).Return([]*store.User{admin, vladimir}, nil)
	return handlers.New(st, nil, 0, nil), st, berlin
}

func TestHandleChore_ScheduledOneOff(t *testing.T) {
	h, st, berlin := scheduledChoreSetup(t)

	st.On("CreateRecurringChore", mock.Anything, mock.MatchedBy(func(c *store.RecurringChore) bool {
		return c.Description == "Air the cellar" && c.Interval == 0 && c.AssigneeUserID == 7 && c.EndsAt == nil &&
			c.NextRunAt.Equal(time.Date(2026, time.October, 21, 12, 0, 0, 0, berlin))
	})).Return(nil).Run(func(args mock.Arguments) { args.Get(1).(*store.RecurringChore).ID = 5 })

	resp, err := h.HandleChore(choreCommand("Air the cellar /Vladimir /from=2026-10-21_12:00"))
	assert.NoError(t, err)

	text := resp.(tgbotapi.MessageConfig).Text
	assert.Contains(t, text, "Chore scheduled")
	assert.Contains(t, text, "Vladimir")
	assert.Contains(t, text, "2026-10-21 12:00")
	assert.NotContains(t, text, "Assigned chore to")
	// Nothing is assigned now
	st.AssertNotCalled(t, "CreateChore", mock.Anything, mock.Anything)
	st.AssertExpectations(t)
}

func TestHandleChore_RecurringWithStartAndEnd(t *testing.T) {
	h, st, berlin := scheduledChoreSetup(t)

	st.On("CreateRecurringChore", mock.Anything, mock.MatchedBy(func(c *store.RecurringChore) bool {
		return c.Interval == 1 && c.AssigneeUserID == 7 &&
			c.NextRunAt.Equal(time.Date(2026, time.October, 21, 12, 0, 0, 0, berlin)) &&
			c.EndsAt != nil && c.EndsAt.Equal(time.Date(2026, time.October, 30, 23, 59, 59, 0, berlin))
	})).Return(nil).Run(func(args mock.Arguments) { args.Get(1).(*store.RecurringChore).ID = 6 })

	resp, err := h.HandleChore(choreCommand("Air the cellar /Vladimir /1d /from=2026-10-21_12:00 /until=2026-10-30"))
	assert.NoError(t, err)

	text := resp.(tgbotapi.MessageConfig).Text
	assert.Contains(t, text, "Recurring chore scheduled")
	assert.Contains(t, text, "every 1 days")
	assert.Contains(t, text, "2026-10-21 12:00")
	assert.Contains(t, text, "Until:</b> 2026-10-30 23:59 (10 more run(s))")
	st.AssertNotCalled(t, "CreateChore", mock.Anything, mock.Anything)
	st.AssertExpectations(t)
}

func TestHandleChore_ScheduleErrors(t *testing.T) {
	cases := []struct {
		args, want string
	}{
		{"Air /from=2026-10-20_13:00", "in the past"},
		{"Air /from=21.10.2026", "Invalid /from="},
		{"Air /until=2026-10-30", "only works for recurring chores"},
		{"Air /1d /until=30.10.2026", "Invalid /until="},
		{"Air /1d /from=2026-10-25_12:00 /until=2026-10-24", "before the first run"},
		{"Air /1d /until=2026-10-19", "before the first run"},
		{"Air /Nobody /from=2026-10-21_12:00", "No active user named"},
	}
	for _, c := range cases {
		t.Run(c.args, func(t *testing.T) {
			h, st, _ := scheduledChoreSetup(t)
			resp, err := h.HandleChore(choreCommand(c.args))
			assert.NoError(t, err)
			assert.Contains(t, resp.(tgbotapi.MessageConfig).Text, c.want)
			st.AssertNotCalled(t, "CreateRecurringChore", mock.Anything, mock.Anything)
		})
	}
}

func TestHandleChore_RecurringEndsToday_AssignsOnlyNow(t *testing.T) {
	h, st, _ := scheduledChoreSetup(t)
	st.On("CreateChore", mock.Anything, mock.Anything).Return(nil)

	resp, err := h.HandleChore(choreCommand("Air /Vladimir /1d /until=2026-10-20"))
	assert.NoError(t, err)

	text := resp.(tgbotapi.MessageConfig).Text
	assert.Contains(t, text, "Assigned chore to")
	assert.Contains(t, text, "nothing was scheduled")
	st.AssertNotCalled(t, "CreateRecurringChore", mock.Anything, mock.Anything)
}

// processSetup runs ProcessRecurringChores with one due chore and one active
// user, and returns the store to check what happened.
func processSetup(t *testing.T, chore *store.RecurringChore) *mocks.MockStore {
	t.Helper()
	st := new(mocks.MockStore)
	st.On("GetDueRecurringChores", mock.Anything, mock.Anything).Return([]*store.RecurringChore{chore}, nil)
	st.On("ListActiveUsers", mock.Anything).Return([]*store.User{{ID: 7, TelegramUserID: 0, FirstName: "Vladimir", IsActive: true}}, nil)
	st.On("GetOffDutyUsers", mock.Anything, mock.Anything).Return([]*store.User{}, nil)
	st.On("CreateChore", mock.Anything, mock.Anything).Return(nil)
	st.On("CancelRecurringChore", mock.Anything, chore.ID).Return(nil)
	st.On("UpdateRecurringChoreNextRun", mock.Anything, chore.ID, mock.Anything).Return(nil)
	assert.NoError(t, handlers.New(st, nil, 0, nil).ProcessRecurringChores(context.Background()))
	return st
}

func TestProcessRecurringChores_OneOffRunsOnce(t *testing.T) {
	st := processSetup(t, &store.RecurringChore{ID: 5, Description: "Air", Interval: 0, NextRunAt: time.Now().Add(-time.Minute), AssigneeUserID: 7, IsActive: true})

	st.AssertCalled(t, "CreateChore", mock.Anything, mock.MatchedBy(func(c *store.Chore) bool { return c.UserID == 7 && c.Description == "Air" }))
	st.AssertCalled(t, "CancelRecurringChore", mock.Anything, int64(5))
	st.AssertNotCalled(t, "UpdateRecurringChoreNextRun", mock.Anything, mock.Anything, mock.Anything)
}

func TestProcessRecurringChores_LastRunBeforeEnd(t *testing.T) {
	ends := time.Now().Add(12 * time.Hour)
	st := processSetup(t, &store.RecurringChore{ID: 6, Description: "Air", Interval: 1, NextRunAt: time.Now().Add(-time.Minute), EndsAt: &ends, IsActive: true})

	st.AssertCalled(t, "CreateChore", mock.Anything, mock.Anything)
	st.AssertCalled(t, "CancelRecurringChore", mock.Anything, int64(6))
	st.AssertNotCalled(t, "UpdateRecurringChoreNextRun", mock.Anything, mock.Anything, mock.Anything)
}

func TestProcessRecurringChores_MoreRunsLeft(t *testing.T) {
	ends := time.Now().Add(72 * time.Hour)
	st := processSetup(t, &store.RecurringChore{ID: 6, Description: "Air", Interval: 1, NextRunAt: time.Now().Add(-time.Minute), EndsAt: &ends, IsActive: true})

	st.AssertCalled(t, "CreateChore", mock.Anything, mock.Anything)
	st.AssertCalled(t, "UpdateRecurringChoreNextRun", mock.Anything, int64(6), mock.Anything)
	st.AssertNotCalled(t, "CancelRecurringChore", mock.Anything, mock.Anything)
}

func TestProcessRecurringChores_EndPassedWhileDown(t *testing.T) {
	ends := time.Now().Add(-time.Hour)
	st := processSetup(t, &store.RecurringChore{ID: 6, Description: "Air", Interval: 1, NextRunAt: time.Now().Add(-time.Minute), EndsAt: &ends, IsActive: true})

	st.AssertNotCalled(t, "CreateChore", mock.Anything, mock.Anything)
	st.AssertCalled(t, "CancelRecurringChore", mock.Anything, int64(6))
}
