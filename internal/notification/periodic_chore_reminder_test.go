package notification

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/korjavin/dutyassistant/internal/store/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockBotSender struct {
	mock.Mock
}

func (m *MockBotSender) SendMessageHTML(chatID int64, text string) error {
	args := m.Called(chatID, text)
	return args.Error(0)
}

func TestNextReminderTime_WithinWindow(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Berlin")
	// Set initial time to 13:00, next run should be 3-6 hours later
	now := time.Date(2026, 3, 14, 13, 0, 0, 0, loc)

	for i := 0; i < 100; i++ {
		next := nextReminderTime(now, loc)
		diff := next.Sub(now)

		// Next time should be between 3 and 6 hours from now
		assert.True(t, diff >= 3*time.Hour && diff <= 6*time.Hour)

		// The new time should be either within 16:00-19:00 window, or if it crosses 19:00, it wraps
		hour := next.Hour()
		if hour < 16 || hour >= 19 {
			t.Errorf("Next time %v should be within 16:00-19:00 window", next)
		}
	}
}

func TestNextReminderTime_BeforeWindow(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Berlin")
	// Set initial time to 10:00, next run (+3-6h=13:00-16:00) should be advanced to 16:00 same day if it falls before 16:00
	now := time.Date(2026, 3, 14, 10, 0, 0, 0, loc)

	for i := 0; i < 100; i++ {
		next := nextReminderTime(now, loc)
		assert.Equal(t, 2026, next.Year())
		assert.Equal(t, time.Month(3), next.Month())
		assert.Equal(t, 14, next.Day())
		assert.Equal(t, 16, next.Hour())
		assert.True(t, next.Minute() >= 0 && next.Minute() < 30)
	}
}

func TestNextReminderTime_AfterWindow(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Berlin")
	// Set initial time to 17:00, next run (+3-6h) will be after 19:00
	now := time.Date(2026, 3, 14, 17, 0, 0, 0, loc)

	for i := 0; i < 100; i++ {
		next := nextReminderTime(now, loc)

		// It should wrap to next day
		assert.Equal(t, 2026, next.Year())
		assert.Equal(t, time.Month(3), next.Month())
		assert.Equal(t, 15, next.Day())
		assert.Equal(t, 16, next.Hour())

		// Minutes should be 0-30
		assert.True(t, next.Minute() >= 0 && next.Minute() < 30)
	}
}

func TestSendChoreReminders_OutsideWindow(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Berlin")
	bot := new(MockBotSender)
	storeMock := new(mocks.MockStore)
	ctx := context.Background()

	// Before 16:00
	now := time.Date(2026, 3, 14, 15, 59, 0, 0, loc)
	sendChoreReminders(ctx, now, bot, storeMock, nil, loc)

	// After 19:00
	now = time.Date(2026, 3, 14, 19, 0, 0, 0, loc)
	sendChoreReminders(ctx, now, bot, storeMock, nil, loc)

	// Verify bot and store were not called
	bot.AssertNotCalled(t, "SendMessageHTML")
	storeMock.AssertNotCalled(t, "GetActiveChores")
}

func TestSendChoreReminders_InsideWindow_NoChores(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Berlin")
	bot := new(MockBotSender)
	storeMock := new(mocks.MockStore)
	ctx := context.Background()

	// Inside window
	now := time.Date(2026, 3, 14, 17, 0, 0, 0, loc)

	storeMock.On("GetActiveChores", ctx).Return([]*store.Chore{}, nil)

	sendChoreReminders(ctx, now, bot, storeMock, nil, loc)

	// Verify store was called but bot was not
	storeMock.AssertCalled(t, "GetActiveChores", ctx)
	bot.AssertNotCalled(t, "SendMessageHTML")
}

func TestSendChoreReminders_InsideWindow_WithChores(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Berlin")
	bot := new(MockBotSender)
	storeMock := new(mocks.MockStore)
	ctx := context.Background()

	// Inside window
	now := time.Date(2026, 3, 14, 17, 0, 0, 0, loc)

	chores := []*store.Chore{
		{
			ID: 1, UserID: 10, Description: "Chore 1",
			User: &store.User{ID: 10, TelegramUserID: 123},
		},
		{
			ID: 2, UserID: 10, Description: "Chore 2",
			User: &store.User{ID: 10, TelegramUserID: 123},
		},
		{
			ID: 3, UserID: 20, Description: "Chore 3",
			User: &store.User{ID: 20, TelegramUserID: 456},
		},
	}

	storeMock.On("GetActiveChores", ctx).Return(chores, nil)

	bot.On("SendMessageHTML", int64(123), mock.AnythingOfType("string")).Return(nil)
	bot.On("SendMessageHTML", int64(456), mock.AnythingOfType("string")).Return(nil)

	sendChoreReminders(ctx, now, bot, storeMock, nil, loc)

	storeMock.AssertCalled(t, "GetActiveChores", ctx)
	bot.AssertCalled(t, "SendMessageHTML", int64(123), mock.AnythingOfType("string"))
	bot.AssertCalled(t, "SendMessageHTML", int64(456), mock.AnythingOfType("string"))
}

type MockChoreNudger struct {
	mock.Mock
}

func (m *MockChoreNudger) ChoreNudge(ctx context.Context, userName, choresContext string) (string, error) {
	args := m.Called(ctx, userName, choresContext)
	return args.String(0), args.Error(1)
}

func TestSendChoreReminders_WithNudger(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Berlin")
	bot := new(MockBotSender)
	storeMock := new(mocks.MockStore)
	nudger := new(MockChoreNudger)
	ctx := context.Background()

	now := time.Date(2026, 3, 14, 17, 0, 0, 0, loc)
	chores := []*store.Chore{
		{
			ID: 1, UserID: 10, Description: "Take out the trash",
			AssignedAt: now.Add(-72 * time.Hour),
			DeadlineAt: now.Add(-48 * time.Hour),
			User:       &store.User{ID: 10, TelegramUserID: 123, FirstName: "Anna"},
		},
	}
	storeMock.On("GetActiveChores", ctx).Return(chores, nil)

	nudger.On("ChoreNudge", ctx, "Anna", mock.MatchedBy(func(c string) bool {
		return strings.Contains(c, "Take out the trash") &&
			strings.Contains(c, "assigned 3 days ago") &&
			strings.Contains(c, "OVERDUE by 2 days")
	})).Return("🗑️ Anna! Trash first, treats later — or the admin hears about it 👀", nil)

	var sent string
	bot.On("SendMessageHTML", int64(123), mock.AnythingOfType("string")).
		Run(func(args mock.Arguments) { sent = args.String(1) }).
		Return(nil)

	sendChoreReminders(ctx, now, bot, storeMock, nudger, loc)

	nudger.AssertExpectations(t)
	assert.Contains(t, sent, "Trash first, treats later")
	assert.Contains(t, sent, "1. <b>Take out the trash</b>")
	assert.NotContains(t, sent, "Just a gentle reminder")
}

func TestSendChoreReminders_NudgerFailureFallsBack(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Berlin")
	bot := new(MockBotSender)
	storeMock := new(mocks.MockStore)
	nudger := new(MockChoreNudger)
	ctx := context.Background()

	now := time.Date(2026, 3, 14, 17, 0, 0, 0, loc)
	chores := []*store.Chore{
		{
			ID: 1, UserID: 10, Description: "Wash the dishes",
			User: &store.User{ID: 10, TelegramUserID: 123, FirstName: "Anna"},
		},
	}
	storeMock.On("GetActiveChores", ctx).Return(chores, nil)
	nudger.On("ChoreNudge", ctx, "Anna", mock.AnythingOfType("string")).Return("", errors.New("llm down"))

	var sent string
	bot.On("SendMessageHTML", int64(123), mock.AnythingOfType("string")).
		Run(func(args mock.Arguments) { sent = args.String(1) }).
		Return(nil)

	sendChoreReminders(ctx, now, bot, storeMock, nudger, loc)

	assert.Contains(t, sent, "Just a gentle reminder")
	assert.Contains(t, sent, "1. <b>Wash the dishes</b>")
}
