package notification

import (
	"context"
	"errors"
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
func (m *MockStore) GetUserByTelegramID(ctx context.Context, id int64) (*store.User, error) { return nil, nil }
func (m *MockStore) GetUserByName(ctx context.Context, name string) (*store.User, error)    { return nil, nil }
func (m *MockStore) ListActiveUsers(ctx context.Context) ([]*store.User, error)              { return nil, nil }
func (m *MockStore) ListAllUsers(ctx context.Context) ([]*store.User, error)                 { return nil, nil }
func (m *MockStore) CreateUser(ctx context.Context, user *store.User) error                  { return nil }
func (m *MockStore) UpdateUser(ctx context.Context, user *store.User) error                  { return nil }
func (m *MockStore) GetUserStats(ctx context.Context, userID int64) (*store.UserStats, error) { return nil, nil }
func (m *MockStore) CreateDuty(ctx context.Context, duty *store.Duty) error                  { return nil }
func (m *MockStore) UpdateDuty(ctx context.Context, duty *store.Duty) error                  { return nil }
func (m *MockStore) DeleteDuty(ctx context.Context, date time.Time) error                    { return nil }
func (m *MockStore) GetDutiesByMonth(ctx context.Context, year int, month time.Month) ([]*store.Duty, error) {
	return nil, nil
}
func (m *MockStore) CompleteDuty(ctx context.Context, date time.Time) error                  { return nil }
func (m *MockStore) GetTodaysDuty(ctx context.Context) (*store.Duty, error)                  { return nil, nil }
func (m *MockStore) GetCompletedDutiesInRange(ctx context.Context, start, end time.Time) ([]*store.Duty, error) {
	return nil, nil
}
func (m *MockStore) AddToVolunteerQueue(ctx context.Context, userID int64, days int) error   { return nil }
func (m *MockStore) AddToAdminQueue(ctx context.Context, userID int64, days int) error       { return nil }
func (m *MockStore) DecrementVolunteerQueue(ctx context.Context, userID int64) error         { return nil }
func (m *MockStore) DecrementAdminQueue(ctx context.Context, userID int64) error             { return nil }
func (m *MockStore) ReduceAdminQueue(ctx context.Context, userID int64, days int) error      { return nil }
func (m *MockStore) GetUsersWithVolunteerQueue(ctx context.Context) ([]*store.User, error)   { return nil, nil }
func (m *MockStore) GetUsersWithAdminQueue(ctx context.Context) ([]*store.User, error)       { return nil, nil }
func (m *MockStore) SetOffDuty(ctx context.Context, userID int64, start, end time.Time) error { return nil }
func (m *MockStore) ClearOffDuty(ctx context.Context, userID int64) error                    { return nil }
func (m *MockStore) IsUserOffDuty(ctx context.Context, userID int64, date time.Time) (bool, error) {
	return false, nil
}
func (m *MockStore) GetOffDutyUsers(ctx context.Context, date time.Time) ([]*store.User, error) {
	return nil, nil
}
func (m *MockStore) SetVacationMode(ctx context.Context, enabled bool) error { return nil }
func (m *MockStore) IsVacationMode(ctx context.Context) (bool, error) { return false, nil }

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

	notifier := NewNotifier(mockStore, mockScheduler, mockBot, 12345, "0 16 * * *", loc)

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
