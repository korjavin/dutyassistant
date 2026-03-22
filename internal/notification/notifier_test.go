package notification

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockStore is a mock implementation of the store.Store interface.
type MockStore struct {
	mock.Mock
}

func (m *MockStore) GetDutyByDate(ctx context.Context, date time.Time) (*store.Duty, error) {
	args := m.Called(ctx, date)
	duty, _ := args.Get(0).(*store.Duty)
	return duty, args.Error(1)
}

// Implement other store.Store methods as needed for tests, returning nil or zero values.
func (m *MockStore) GetUserByTelegramID(ctx context.Context, id int64) (*store.User, error) {
	return nil, nil
}

func (m *MockStore) GetUserByName(ctx context.Context, name string) (*store.User, error) {
	return nil, nil
}
func (m *MockStore) ListActiveUsers(ctx context.Context) ([]*store.User, error) { return nil, nil }
func (m *MockStore) ListAllUsers(ctx context.Context) ([]*store.User, error)    { return nil, nil }
func (m *MockStore) CreateUser(ctx context.Context, user *store.User) error     { return nil }
func (m *MockStore) UpdateUser(ctx context.Context, user *store.User) error     { return nil }
func (m *MockStore) GetUserStats(ctx context.Context, userID int64) (*store.UserStats, error) {
	return nil, nil
}
func (m *MockStore) CreateDuty(ctx context.Context, duty *store.Duty) error { return nil }
func (m *MockStore) UpdateDuty(ctx context.Context, duty *store.Duty) error { return nil }
func (m *MockStore) DeleteDuty(ctx context.Context, date time.Time) error   { return nil }
func (m *MockStore) GetDutiesByMonth(ctx context.Context, year int, month time.Month) ([]*store.Duty, error) {
	return nil, nil
}
func (m *MockStore) CompleteDuty(ctx context.Context, date time.Time) error { return nil }
func (m *MockStore) GetTodaysDuty(ctx context.Context) (*store.Duty, error) { return nil, nil }
func (m *MockStore) GetLastDuty(ctx context.Context) (*store.Duty, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*store.Duty), args.Error(1)
}

func (m *MockStore) GetFutureDuties(ctx context.Context, from time.Time) ([]*store.Duty, error) {
	args := m.Called(ctx, from)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*store.Duty), args.Error(1)
}

func (m *MockStore) GetCompletedDutiesInRange(ctx context.Context, start, end time.Time) ([]*store.Duty, error) {
	return nil, nil
}

func (m *MockStore) SaveDailyParticipantRatings(ctx context.Context, date time.Time, ratings []*store.ParticipantDailyRating) error {
	return nil
}

func (m *MockStore) GetParticipantsForRating(ctx context.Context) ([]*store.User, error) {
	return nil, nil
}

func (m *MockStore) GetCurrentMonthParticipantRatings(ctx context.Context, now time.Time) ([]*store.ParticipantDailyRating, error) {
	return nil, nil
}

func (m *MockStore) GetMonthlyParticipantTotals(ctx context.Context, year int, month time.Month) ([]*store.ParticipantMonthlyTotal, error) {
	return nil, nil
}

func (m *MockStore) AddToVolunteerQueue(ctx context.Context, userID int64, days int) error {
	return nil
}
func (m *MockStore) AddToAdminQueue(ctx context.Context, userID int64, days int) error  { return nil }
func (m *MockStore) DecrementVolunteerQueue(ctx context.Context, userID int64) error    { return nil }
func (m *MockStore) DecrementAdminQueue(ctx context.Context, userID int64) error        { return nil }
func (m *MockStore) ReduceAdminQueue(ctx context.Context, userID int64, days int) error { return nil }
func (m *MockStore) GetUsersWithVolunteerQueue(ctx context.Context) ([]*store.User, error) {
	return nil, nil
}

func (m *MockStore) GetUsersWithAdminQueue(ctx context.Context) ([]*store.User, error) {
	return nil, nil
}

func (m *MockStore) SetOffDuty(ctx context.Context, userID int64, start, end time.Time) error {
	return nil
}
func (m *MockStore) ClearOffDuty(ctx context.Context, userID int64) error { return nil }
func (m *MockStore) IsUserOffDuty(ctx context.Context, userID int64, date time.Time) (bool, error) {
	return false, nil
}

func (m *MockStore) GetOffDutyUsers(ctx context.Context, date time.Time) ([]*store.User, error) {
	return nil, nil
}
func (m *MockStore) SetVacationMode(ctx context.Context, enabled bool) error { return nil }
func (m *MockStore) IsVacationMode(ctx context.Context) (bool, error)        { return false, nil }
func (m *MockStore) CreateRecurringChore(ctx context.Context, chore *store.RecurringChore) error {
	return nil
}

func (m *MockStore) GetRecurringChore(ctx context.Context, id int64) (*store.RecurringChore, error) {
	return nil, nil
}

func (m *MockStore) GetActiveRecurringChores(ctx context.Context) ([]*store.RecurringChore, error) {
	return nil, nil
}

func (m *MockStore) GetDueRecurringChores(ctx context.Context, before time.Time) ([]*store.RecurringChore, error) {
	return nil, nil
}

func (m *MockStore) UpdateRecurringChoreNextRun(ctx context.Context, id int64, nextRun time.Time) error {
	return nil
}
func (m *MockStore) CancelRecurringChore(ctx context.Context, id int64) error { return nil }

// MockScheduler is a mock implementation of the Scheduler interface.
type MockScheduler struct {
	mock.Mock
}

func (m *MockScheduler) AssignDutyRoundRobin(ctx context.Context, date time.Time) (*store.Duty, error) {
	args := m.Called(ctx, date)
	duty, _ := args.Get(0).(*store.Duty)
	return duty, args.Error(1)
}

// MockTelegramBot is a mock implementation of the TelegramBot interface.
type MockTelegramBot struct {
	mock.Mock
}

func (m *MockTelegramBot) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	args := m.Called(c)
	return args.Get(0).(tgbotapi.Message), args.Error(1)
}

// setupNotifierTest creates a Notifier with mocked dependencies for testing.
func setupNotifierTest(t *testing.T) (*Notifier, *MockStore, *MockScheduler, *MockTelegramBot) {
	loc, err := time.LoadLocation("Europe/Berlin")
	assert.NoError(t, err)

	mockStore := new(MockStore)
	mockScheduler := new(MockScheduler)
	mockBot := new(MockTelegramBot)

	notifier := NewNotifier(mockStore, mockScheduler, mockBot, 12345, "0 16 * * *", loc, nil)

	// Set a fixed time for predictable testing.
	// This is a Thursday, so "tomorrow" will be a Friday.
	fixedTime := time.Date(2023, 10, 26, 15, 0, 0, 0, loc)
	notifier.now = func() time.Time {
		return fixedTime
	}

	return notifier, mockStore, mockScheduler, mockBot
}

func TestCheckAndNotify_DutyAlreadyExists(t *testing.T) {
	notifier, mockStore, _, mockBot := setupNotifierTest(t)
	tomorrow := notifier.now().In(notifier.location).Add(24 * time.Hour)

	// Arrange
	existingDuty := &store.Duty{
		DutyDate:       tomorrow,
		AssignmentType: store.AssignmentTypeVoluntary,
		User: &store.User{
			FirstName:      "Alex",
			TelegramUserID: 99999,
		},
	}
	mockStore.On("GetDutyByDate", mock.Anything, mock.Anything).Return(existingDuty, nil)
	mockBot.On("Send", mock.Anything).Return(tgbotapi.Message{}, nil)

	// Act
	notifier.checkAndNotify()

	// Assert
	mockStore.AssertCalled(t, "GetDutyByDate", mock.Anything, mock.Anything)
	// Should be called twice - once for group message, once for DM
	assert.Equal(t, 2, len(mockBot.Calls))
	// Check that the group message contains the expected text
	groupMessage := mockBot.Calls[0].Arguments.Get(0).(tgbotapi.MessageConfig)
	assert.Contains(t, groupMessage.Text, "Today's hero")
	assert.Contains(t, groupMessage.Text, "Alex")
	// Check that the DM was sent
	dmMessage := mockBot.Calls[1].Arguments.Get(0).(tgbotapi.MessageConfig)
	assert.Contains(t, dmMessage.Text, "Congratulations")
}

func TestCheckAndNotify_AutoAssignSuccess(t *testing.T) {
	notifier, mockStore, mockScheduler, mockBot := setupNotifierTest(t)
	tomorrow := notifier.now().In(notifier.location).Add(24 * time.Hour)

	// Arrange
	// No duty exists
	mockStore.On("GetDutyByDate", mock.Anything, mock.Anything).Return(nil, errors.New("not found"))
	// Scheduler assigns a new duty
	assignedDuty := &store.Duty{
		DutyDate:       tomorrow,
		AssignmentType: store.AssignmentTypeRoundRobin,
		User: &store.User{
			FirstName:      "Casey",
			TelegramUserID: 88888,
		},
	}
	mockScheduler.On("AssignDutyRoundRobin", mock.Anything, mock.Anything).Return(assignedDuty, nil)
	mockBot.On("Send", mock.Anything).Return(tgbotapi.Message{}, nil)

	// Act
	notifier.checkAndNotify()

	// Assert
	mockStore.AssertCalled(t, "GetDutyByDate", mock.Anything, mock.Anything)
	mockScheduler.AssertCalled(t, "AssignDutyRoundRobin", mock.Anything, mock.Anything)
	// Should be called twice - once for group message, once for DM
	assert.Equal(t, 2, len(mockBot.Calls))
	// Check that the group message contains the expected text
	groupMessage := mockBot.Calls[0].Arguments.Get(0).(tgbotapi.MessageConfig)
	assert.Contains(t, groupMessage.Text, "Today's hero")
	assert.Contains(t, groupMessage.Text, "Casey")
	// Check that the DM was sent
	dmMessage := mockBot.Calls[1].Arguments.Get(0).(tgbotapi.MessageConfig)
	assert.Contains(t, dmMessage.Text, "Congratulations")
}

func TestCheckAndNotify_AutoAssignFails(t *testing.T) {
	notifier, mockStore, mockScheduler, mockBot := setupNotifierTest(t)

	// Arrange
	mockStore.On("GetDutyByDate", mock.Anything, mock.Anything).Return(nil, errors.New("not found"))
	mockScheduler.On("AssignDutyRoundRobin", mock.Anything, mock.Anything).Return(nil, errors.New("scheduler failed"))

	// Act
	notifier.checkAndNotify()

	// Assert
	mockStore.AssertCalled(t, "GetDutyByDate", mock.Anything, mock.Anything)
	mockScheduler.AssertCalled(t, "AssignDutyRoundRobin", mock.Anything, mock.Anything)
	// The bot should NOT be called if assignment fails.
	mockBot.AssertNotCalled(t, "Send", mock.Anything)
}

func TestCheckAndNotify_SendFails(t *testing.T) {
	notifier, mockStore, _, mockBot := setupNotifierTest(t)
	tomorrow := notifier.now().In(notifier.location).Add(24 * time.Hour)

	// Arrange
	existingDuty := &store.Duty{
		DutyDate:       tomorrow,
		AssignmentType: store.AssignmentTypeAdmin,
		User: &store.User{
			FirstName:      "Alex",
			TelegramUserID: 77777,
		},
	}
	mockStore.On("GetDutyByDate", mock.Anything, mock.Anything).Return(existingDuty, nil)
	// Simulate a failure in the Telegram API.
	mockBot.On("Send", mock.Anything).Return(tgbotapi.Message{}, errors.New("telegram network error"))

	// Act
	notifier.checkAndNotify()

	// Assert
	// Should attempt to send twice (group + DM), both will fail but should be attempted
	assert.Equal(t, 2, len(mockBot.Calls))
	// We can't assert on logs directly without a more complex setup, but we expect the error to be logged.
}

func (m *MockStore) CreateChore(ctx context.Context, chore *store.Chore) error {
	args := m.Called(ctx, chore)
	return args.Error(0)
}

func (m *MockStore) GetChoreByReminderID(ctx context.Context, reminderID string) (*store.Chore, error) {
	args := m.Called(ctx, reminderID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*store.Chore), args.Error(1)
}

func (m *MockStore) GetActiveChores(ctx context.Context) ([]*store.Chore, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*store.Chore), args.Error(1)
}

func (m *MockStore) GetActiveChoresByUserID(ctx context.Context, userID int64) ([]*store.Chore, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*store.Chore), args.Error(1)
}

func (m *MockStore) GetOverdueChores(ctx context.Context) ([]*store.Chore, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*store.Chore), args.Error(1)
}

func (m *MockStore) CompleteChoreByReminderID(ctx context.Context, reminderID string) error {
	args := m.Called(ctx, reminderID)
	return args.Error(0)
}

func (m *MockStore) GetTopOverdueChores(ctx context.Context, limit int) ([]*store.ChoreStat, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*store.ChoreStat), args.Error(1)
}

func (m *MockStore) GetTopCompletedChoresUsers(ctx context.Context, limit int) ([]*store.UserChoreStat, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*store.UserChoreStat), args.Error(1)
}

func (m *MockStore) GetUserWeeklyStats(ctx context.Context, since time.Time) ([]*store.UserWeeklyStats, error) {
	args := m.Called(ctx, since)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*store.UserWeeklyStats), args.Error(1)
}

func (m *MockStore) GetLastChoreDigestDate(ctx context.Context) (string, error) {
	args := m.Called(ctx)
	return args.String(0), args.Error(1)
}

func (m *MockStore) CancelChore(ctx context.Context, id int64) (*store.Chore, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*store.Chore), args.Error(1)
}

func (m *MockStore) ListActiveChores(ctx context.Context) ([]*store.Chore, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*store.Chore), args.Error(1)
}

func (m *MockStore) SetLastChoreDigestDate(ctx context.Context, date string) error {
	args := m.Called(ctx, date)
	return args.Error(0)
}

func (m *MockStore) UpdateRecurringChoreDescription(ctx context.Context, id int64, description string) error {
	args := m.Called(ctx, id, description)
	return args.Error(0)
}

func (m *MockStore) GetChoreByID(ctx context.Context, id int64) (*store.Chore, error) {
	return nil, nil
}

func (m *MockStore) UpdateChoreUserID(ctx context.Context, choreID int64, newUserID int64) error {
	return nil
}

func TestSendDailyChoreSummary_NoOverdue(t *testing.T) {
	mockStore := new(MockStore)
	var sentText string
	bot := setupBotAPI(t, func(form url.Values) { sentText = form.Get("text") })

	mockStore.On("GetLastChoreDigestDate", mock.Anything).Return("", nil)
	mockStore.On("GetOverdueChores", mock.Anything).Return([]*store.Chore{}, nil)
	mockStore.On("SetLastChoreDigestDate", mock.Anything, mock.Anything).Return(nil)

	err := SendDailyChoreSummary(context.Background(), bot, mockStore, 123, true, "UTC")
	assert.NoError(t, err)
	assert.Contains(t, sentText, "No overdue chores")
}

func TestSendDailyChoreSummary_Idempotency(t *testing.T) {
	mockStore := new(MockStore)
	var sentMessages []string
	bot := setupBotAPI(t, func(form url.Values) { sentMessages = append(sentMessages, form.Get("text")) })

	todayStr := time.Now().UTC().Format("2006-01-02")

	// First call - should skip because date matches today
	mockStore.On("GetLastChoreDigestDate", mock.Anything).Return(todayStr, nil)

	err := SendDailyChoreSummary(context.Background(), bot, mockStore, 123, true, "UTC")
	assert.NoError(t, err)
	assert.Equal(t, 0, len(sentMessages))

	// Verify GetOverdueChores was NOT called (we skip early)
	mockStore.AssertNotCalled(t, "GetOverdueChores")
}

func TestSendDailyChoreSummary_NotCronIgnoresIdempotency(t *testing.T) {
	mockStore := new(MockStore)
	var sentMessages []string
	bot := setupBotAPI(t, func(form url.Values) { sentMessages = append(sentMessages, form.Get("text")) })

	mockStore.On("GetOverdueChores", mock.Anything).Return([]*store.Chore{}, nil)

	// isCron=false, should NOT check last date, should send the empty message
	err := SendDailyChoreSummary(context.Background(), bot, mockStore, 123, false, "UTC")
	assert.NoError(t, err)
	assert.Equal(t, 1, len(sentMessages))
	assert.Contains(t, sentMessages[0], "No overdue chores")
}

type RoundTripFunc func(req *http.Request) *http.Response

func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

func NewTestClient(fn RoundTripFunc) *http.Client {
	return &http.Client{Transport: fn}
}

func TestRenderBarChart(t *testing.T) {
	assert.Equal(t, "█████░░░░░", renderBarChart(5, 10, 10))
	assert.Equal(t, "██████████", renderBarChart(10, 10, 10))
	assert.Equal(t, "░░░░░░░░░░", renderBarChart(0, 10, 10))
	assert.Equal(t, "██████████", renderBarChart(15, 10, 10)) // Over maximum
	assert.Equal(t, "░░░░░░░░░░", renderBarChart(5, 0, 10))   // Max is 0
}

func TestDetermineWinner(t *testing.T) {
	stats := []*store.UserWeeklyStats{
		{Name: "Alice", CompletedCount: 5, AvgLateSeconds: 3600},
		{Name: "Bob", CompletedCount: 10, AvgLateSeconds: 0},
		{Name: "Charlie", CompletedCount: 10, AvgLateSeconds: 1800}, // Bob wins over Charlie (less late)
	}
	winner := determineWinner(stats)
	assert.NotNil(t, winner)
	assert.Equal(t, "Bob", winner.Name)

	stats2 := []*store.UserWeeklyStats{
		{Name: "Alice", CompletedCount: 5, AvgLateSeconds: 0},
		{Name: "Bob", CompletedCount: 5, AvgLateSeconds: 0}, // Alice wins (first in list)
	}
	winner2 := determineWinner(stats2)
	assert.NotNil(t, winner2)
	assert.Equal(t, "Alice", winner2.Name)

	assert.Nil(t, determineWinner([]*store.UserWeeklyStats{}))
}

func TestSendWeeklyChoreStats(t *testing.T) {
	mockStore := new(MockStore)
	var sentText string
	bot := setupBotAPI(t, func(form url.Values) { sentText = form.Get("text") })

	mockStore.On("GetTopOverdueChores", mock.Anything, 5).Return([]*store.ChoreStat{
		{Description: "Take out trash", Count: 3},
	}, nil)

	mockStore.On("GetUserWeeklyStats", mock.Anything, mock.Anything).Return([]*store.UserWeeklyStats{
		{Name: "Alice", CompletedCount: 5, AvgExecSeconds: 7200, AvgLateSeconds: 0},
		{Name: "Bob", CompletedCount: 2, AvgExecSeconds: 3600, AvgLateSeconds: 1800},
	}, nil)

	err := SendWeeklyChoreStats(context.Background(), bot, mockStore, 123)
	assert.NoError(t, err)

	// Verify the message content
	assert.Contains(t, sentText, "Weekly Chore Statistics")
	assert.Contains(t, sentText, "Take out trash (3 times)")
	assert.Contains(t, sentText, "Top Performers this week:")
	assert.Contains(t, sentText, "Alice")
	assert.Contains(t, sentText, "Bob")
	// Verify simplified one-line format per user
	assert.Contains(t, sentText, "1. Alice — 5 done")
	assert.Contains(t, sentText, "2. Bob — 2 done")
	// Verify shortened winner line
	assert.Contains(t, sentText, "🥇 Winner: Alice")
}

func TestSendDailyChoreSummary_WithOverdue(t *testing.T) {
	mockStore := new(MockStore)
	var sentText string
	bot := setupBotAPI(t, func(form url.Values) { sentText = form.Get("text") })

	loc, _ := time.LoadLocation("Europe/Berlin")
	now := time.Now().In(loc)

	// Create overdue chores in different categories
	// Note: SendDailyChoreSummary uses time.Now() directly, so we must use the actual current time
	chores := []*store.Chore{
		{
			Description:  "Clean the kitchen",
			DeadlineAt:   now.Add(-96 * time.Hour), // 4 days ago - critical
			User:         &store.User{FirstName: "Alice", TelegramUserID: 111},
			ReminderID:   "reminder1",
		},
		{
			Description:  "Take out trash",
			DeadlineAt:   now.Add(-72 * time.Hour), // 3 days ago - critical
			User:         &store.User{FirstName: "Bob", TelegramUserID: 222},
			ReminderID:   "reminder2",
		},
		{
			Description:  "Water plants",
			DeadlineAt:   now.Add(-36 * time.Hour), // 1.5 days ago - medium
			User:         &store.User{FirstName: "Charlie", TelegramUserID: 333},
			ReminderID:   "reminder3",
		},
		{
			Description:  "Fix door",
			DeadlineAt:   now.Add(-24 * time.Hour), // 1 day ago - medium
			User:         &store.User{FirstName: "Diana", TelegramUserID: 444},
			ReminderID:   "reminder4",
		},
		{
			Description:  "Buy groceries",
			// Set deadline to 10 AM today to ensure it's always on the same calendar day
			DeadlineAt:   time.Date(now.Year(), now.Month(), now.Day(), 10, 0, 0, 0, loc),
			User:         &store.User{FirstName: "Eve", TelegramUserID: 555},
			ReminderID:   "reminder5",
		},
	}

	mockStore.On("GetLastChoreDigestDate", mock.Anything).Return("", nil)
	mockStore.On("GetOverdueChores", mock.Anything).Return(chores, nil)
	mockStore.On("SetLastChoreDigestDate", mock.Anything, mock.Anything).Return(nil)

	err := SendDailyChoreSummary(context.Background(), bot, mockStore, 123, true, "Europe/Berlin")
	assert.NoError(t, err)

	// Verify the new compact format
	assert.Contains(t, sentText, "⚠️ <b>Overdue chores:</b>")
	assert.Contains(t, sentText, "🔴 <b>Critical (3+d):</b>")
	assert.Contains(t, sentText, "🟠 <b>Overdue (1-2d):</b>")
	assert.Contains(t, sentText, "🟢 <b>Due today:</b>")

	// Verify the new chore line format "deadline: DATE (+N d)"
	assert.Contains(t, sentText, "deadline: ")
	assert.Contains(t, sentText, "(+4 d)")
	assert.Contains(t, sentText, "(+3 d)")
	assert.Contains(t, sentText, "(+1 d)")

	// Verify descriptions and users are present
	assert.Contains(t, sentText, "Clean the kitchen")
	assert.Contains(t, sentText, "Take out trash")
	assert.Contains(t, sentText, "Water plants")
	assert.Contains(t, sentText, "Fix door")
	assert.Contains(t, sentText, "Buy groceries")
	assert.Contains(t, sentText, "Alice")
	assert.Contains(t, sentText, "Bob")
	assert.Contains(t, sentText, "Charlie")
	assert.Contains(t, sentText, "Diana")
	assert.Contains(t, sentText, "Eve")
}

func setupBotAPI(t *testing.T, checkReq func(url.Values)) *tgbotapi.BotAPI {
	client := NewTestClient(func(req *http.Request) *http.Response {
		if strings.Contains(req.URL.String(), "sendMessage") {
			bodyBytes, _ := io.ReadAll(req.Body)
			form, _ := url.ParseQuery(string(bodyBytes))
			if checkReq != nil {
				checkReq(form)
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewBufferString(`{"ok":true, "result": {"message_id": 1}}`)), Header: make(http.Header)}
		}
		if strings.Contains(req.URL.String(), "getMe") {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewBufferString(`{"ok":true, "result": {"id": 123456, "is_bot": true, "first_name": "TestBot"}}`)), Header: make(http.Header)}
		}
		return &http.Response{StatusCode: 404, Body: io.NopCloser(bytes.NewBufferString(`{}`)), Header: make(http.Header)}
	})
	bot, err := tgbotapi.NewBotAPIWithClient("TOKEN", tgbotapi.APIEndpoint, client)
	assert.NoError(t, err)
	return bot
}
