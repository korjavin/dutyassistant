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

func TestHandleChore_Recurring_DuringHours(t *testing.T) {
	// Set mock time to 14:00 (During hours)
	berlinLoc, _ := time.LoadLocation("Europe/Berlin")
	mockTime := time.Date(2025, time.October, 15, 14, 0, 0, 0, berlinLoc)
	handlers.TimeNow = func() time.Time {
		return mockTime
	}
	defer func() { handlers.TimeNow = time.Now }()

	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	adminUser := &store.User{ID: 1, TelegramUserID: 123, FirstName: "Admin", IsAdmin: true}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(123)).Return(adminUser, nil)

	// We expect a call to create a recurring chore
	mockStore.On("CreateRecurringChore", mock.Anything, mock.MatchedBy(func(c *store.RecurringChore) bool {
		return c.Description == "Clean the kitchen" && c.Interval == 5 &&
			c.NextRunAt.Equal(time.Date(2025, time.October, 20, 10, 0, 0, 0, berlinLoc))
	})).Return(nil).Run(func(args mock.Arguments) {
		chore := args.Get(1).(*store.RecurringChore)
		chore.ID = 42
	})

	// Immediate assignment mock expectations
	activeUsers := []*store.User{adminUser}
	mockStore.On("ListActiveUsers", mock.Anything).Return(activeUsers, nil)
	mockStore.On("GetOffDutyUsers", mock.Anything, mock.Anything).Return([]*store.User{}, nil)
	mockStore.On("CreateChore", mock.Anything, mock.MatchedBy(func(c *store.Chore) bool {
		return c.Description == "Clean the kitchen" && c.UserID == adminUser.ID
	})).Return(nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 789},
		From: &tgbotapi.User{ID: 123},
		Text: "/chore Clean the kitchen /5d",
		Entities: []tgbotapi.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: 6},
		},
	}

	response, err := h.HandleChore(message)
	assert.NoError(t, err)

	assert.Contains(t, response.Text, "Assigned chore to")
	assert.Contains(t, response.Text, "Admin")
	assert.Contains(t, response.Text, "Clean the kitchen")
	assert.Contains(t, response.Text, "Recurring chore scheduled")
	assert.Contains(t, response.Text, "42")
	assert.Contains(t, response.Text, "every 5 days")
}

func TestHandleChore_Recurring_OutsideHours(t *testing.T) {
	// Set mock time to 20:00 (Outside hours)
	berlinLoc, _ := time.LoadLocation("Europe/Berlin")
	mockTime := time.Date(2025, time.October, 15, 20, 0, 0, 0, berlinLoc)
	handlers.TimeNow = func() time.Time {
		return mockTime
	}
	defer func() { handlers.TimeNow = time.Now }()

	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	adminUser := &store.User{ID: 1, TelegramUserID: 123, FirstName: "Admin", IsAdmin: true}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(123)).Return(adminUser, nil)

	// We expect a call to create a recurring chore with tomorrow 10:00 as NextRunAt
	mockStore.On("CreateRecurringChore", mock.Anything, mock.MatchedBy(func(c *store.RecurringChore) bool {
		return c.Description == "Clean the kitchen" && c.Interval == 5 &&
			c.NextRunAt.Equal(time.Date(2025, time.October, 16, 10, 0, 0, 0, berlinLoc))
	})).Return(nil).Run(func(args mock.Arguments) {
		chore := args.Get(1).(*store.RecurringChore)
		chore.ID = 42
	})

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 789},
		From: &tgbotapi.User{ID: 123},
		Text: "/chore Clean the kitchen /5d",
		Entities: []tgbotapi.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: 6},
		},
	}

	response, err := h.HandleChore(message)
	assert.NoError(t, err)

	assert.NotContains(t, response.Text, "Assigned chore to")
	assert.Contains(t, response.Text, "Clean the kitchen")
	assert.Contains(t, response.Text, "Recurring chore scheduled")
	assert.Contains(t, response.Text, "42")
	assert.Contains(t, response.Text, "every 5 days")
}

func TestHandleList_Recurring(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	adminUser := &store.User{ID: 1, TelegramUserID: 123, IsAdmin: true}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(123)).Return(adminUser, nil)

	chores := []*store.RecurringChore{
		{ID: 1, Description: "Foo", Interval: 3, NextRunAt: time.Now()},
		{ID: 2, Description: "Bar", Interval: 7, NextRunAt: time.Now()},
	}

	mockStore.On("GetActiveRecurringChores", mock.Anything).Return(chores, nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 789},
		From: &tgbotapi.User{ID: 123},
		Text: "/list chore",
		Entities: []tgbotapi.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: 5},
		},
	}

	response, err := h.HandleList(message)
	assert.NoError(t, err)

	assert.Contains(t, response.Text, "Foo")
	assert.Contains(t, response.Text, "every 3 days")
	assert.Contains(t, response.Text, "Bar")
	assert.Contains(t, response.Text, "every 7 days")
}

func TestHandleCancel_Recurring(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	adminUser := &store.User{ID: 1, TelegramUserID: 123, IsAdmin: true}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(123)).Return(adminUser, nil)

	chore := &store.RecurringChore{ID: 1, Description: "Foo", IsActive: true}
	mockStore.On("GetRecurringChore", mock.Anything, int64(1)).Return(chore, nil)
	mockStore.On("CancelRecurringChore", mock.Anything, int64(1)).Return(nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 789},
		From: &tgbotapi.User{ID: 123},
		Text: "/cancel chore 1",
		Entities: []tgbotapi.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: 7},
		},
	}

	response, err := h.HandleCancel(message)
	assert.NoError(t, err)

	assert.Contains(t, response.Text, "Periodic chore cancelled")
	assert.Contains(t, response.Text, "Foo")
}

func TestProcessRecurringChores_Success(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	// Mock ChoreReminderManager so that it does not fail on missing Bot API
	botAPI, _ := tgbotapi.NewBotAPI("dummy:token")
	mockStore.On("GetActiveChores", mock.Anything).Return([]*store.Chore{}, nil)
	h.SetBot(botAPI)
	// Actually, we don't want to make real API requests, so let's mock the actual function internally
	// or bypass it by setting TelegramUserID = 0 to simulate failure, OR we can let it fail
	// Wait, if ChoreReminderManager is initialized, SendInitialDM will attempt an API call and return an error
	// if bot API call fails.
	// Since we don't have a mock ChoreReminderManager, we can test the case where selectedUser.TelegramUserID = 0
	// to avoid DM, but wait, if it's 0 it fails assigning.

	now := time.Now()
	chores := []*store.RecurringChore{
		{ID: 1, Description: "Foo", Interval: 3, NextRunAt: now.Add(-time.Hour), IsActive: true},
	}

	// 1. Get due chores
	mockStore.On("GetDueRecurringChores", mock.Anything, mock.Anything).Return(chores, nil)

	// 2. assignRecurringChore needs active users
	users := []*store.User{
		{ID: 1, TelegramUserID: 123, FirstName: "Alice", IsActive: true},
	}
	mockStore.On("ListActiveUsers", mock.Anything).Return(users, nil)
	mockStore.On("GetOffDutyUsers", mock.Anything, mock.Anything).Return([]*store.User{}, nil)

	var capturedReminderID string
	mockStore.On("CreateChore", mock.Anything, mock.MatchedBy(func(c *store.Chore) bool {
		capturedReminderID = c.ReminderID
		return c.Description == "Foo" && c.ReminderID != ""
	})).Return(nil)

	// 3. update next run
	mockStore.On("UpdateRecurringChoreNextRun", mock.Anything, int64(1), mock.MatchedBy(func(next time.Time) bool {
		return next.After(now)
	})).Return(nil)

	err := h.ProcessRecurringChores(context.Background())
	assert.NoError(t, err)

	// Assert the captured ID is non-empty
	assert.NotEmpty(t, capturedReminderID, "ReminderID should not be empty")

	// Ensure methods were called
	mockStore.AssertExpectations(t)
}

func TestHandleList_Task(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	adminUser := &store.User{ID: 1, TelegramUserID: 123, IsAdmin: true}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(123)).Return(adminUser, nil)

	chores := []*store.Chore{
		{ID: 1, Description: "Foo Task", AssignedAt: time.Now(), User: &store.User{FirstName: "Alice"}},
		{ID: 2, Description: "Bar Task", AssignedAt: time.Now(), User: &store.User{FirstName: "Bob"}},
	}

	mockStore.On("ListActiveChores", mock.Anything).Return(chores, nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 789},
		From: &tgbotapi.User{ID: 123},
		Text: "/list task",
		Entities: []tgbotapi.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: 5},
		},
	}

	response, err := h.HandleList(message)
	assert.NoError(t, err)

	assert.Contains(t, response.Text, "Foo Task")
	assert.Contains(t, response.Text, "Alice")
	assert.Contains(t, response.Text, "Bar Task")
	assert.Contains(t, response.Text, "Bob")
}

func TestHandleCancel_Task(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	adminUser := &store.User{ID: 1, TelegramUserID: 123, IsAdmin: true}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(123)).Return(adminUser, nil)

	chore := &store.Chore{ID: 42, ReminderID: "rem-123"}
	mockStore.On("CancelChore", mock.Anything, int64(42)).Return(chore, nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 789},
		From: &tgbotapi.User{ID: 123},
		Text: "/cancel task 42",
		Entities: []tgbotapi.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: 7},
		},
	}

	response, err := h.HandleCancel(message)
	assert.NoError(t, err)

	assert.Contains(t, response.Text, "Regular chore 42 cancelled successfully")
}

func TestHandleCancel_Task_ErrorCases(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	adminUser := &store.User{ID: 1, TelegramUserID: 123, IsAdmin: true}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(123)).Return(adminUser, nil)

	mockStore.On("CancelChore", mock.Anything, int64(99)).Return(nil, assert.AnError)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 789},
		From: &tgbotapi.User{ID: 123},
		Text: "/cancel task 99",
		Entities: []tgbotapi.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: 7},
		},
	}

	response, err := h.HandleCancel(message)
	assert.NoError(t, err)

	assert.Contains(t, response.Text, "Failed to cancel regular chore (not found, already completed, or cancelled)")
}
