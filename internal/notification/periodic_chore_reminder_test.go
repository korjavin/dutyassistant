package notification

import (
	"context"
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
	// Set initial time to 12:00, next run should be 3-6 hours later
	now := time.Date(2026, 3, 14, 12, 0, 0, 0, loc)

	for i := 0; i < 100; i++ {
		next := nextReminderTime(now, loc)
		diff := next.Sub(now)

		// Next time should be between 3 and 6 hours from now
		assert.True(t, diff >= 3*time.Hour && diff <= 6*time.Hour)

		// The new time should be either within 11:00-18:00 window, or if it crosses 18:00, it wraps
		hour := next.Hour()
		if hour < 11 || hour >= 18 {
			t.Errorf("Next time %v should be within 11:00-18:00 window", next)
		}
	}
}

func TestNextReminderTime_BeforeWindow(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Berlin")
	// Set initial time to 06:00, next run should be advanced to 11:00 same day
	now := time.Date(2026, 3, 14, 6, 0, 0, 0, loc)

	next := nextReminderTime(now, loc)
	assert.Equal(t, 2026, next.Year())
	assert.Equal(t, time.Month(3), next.Month())
	assert.Equal(t, 14, next.Day())
	assert.Equal(t, 11, next.Hour())
	assert.Equal(t, 0, next.Minute())
}

func TestNextReminderTime_AfterWindow(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Berlin")
	// Set initial time to 17:00, next run (+3-6h) will be after 18:00
	now := time.Date(2026, 3, 14, 17, 0, 0, 0, loc)

	for i := 0; i < 100; i++ {
		next := nextReminderTime(now, loc)

		// It should wrap to next day
		assert.Equal(t, 2026, next.Year())
		assert.Equal(t, time.Month(3), next.Month())
		assert.Equal(t, 15, next.Day())
		assert.Equal(t, 11, next.Hour())

		// Minutes should be 0-30
		assert.True(t, next.Minute() >= 0 && next.Minute() < 30)
	}
}

func TestSendChoreReminders_OutsideWindow(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Berlin")
	bot := new(MockBotSender)
	storeMock := new(mocks.MockStore)
	ctx := context.Background()

	// Before 11:00
	now := time.Date(2026, 3, 14, 10, 59, 0, 0, loc)
	sendChoreReminders(ctx, now, bot, storeMock, loc)

	// After 18:00
	now = time.Date(2026, 3, 14, 18, 0, 0, 0, loc)
	sendChoreReminders(ctx, now, bot, storeMock, loc)

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
	now := time.Date(2026, 3, 14, 12, 0, 0, 0, loc)

	storeMock.On("GetActiveChores", ctx).Return([]*store.Chore{}, nil)

	sendChoreReminders(ctx, now, bot, storeMock, loc)

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
	now := time.Date(2026, 3, 14, 12, 0, 0, 0, loc)

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

	sendChoreReminders(ctx, now, bot, storeMock, loc)

	storeMock.AssertCalled(t, "GetActiveChores", ctx)
	bot.AssertCalled(t, "SendMessageHTML", int64(123), mock.AnythingOfType("string"))
	bot.AssertCalled(t, "SendMessageHTML", int64(456), mock.AnythingOfType("string"))
}
