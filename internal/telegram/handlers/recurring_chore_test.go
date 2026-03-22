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

	msgText := response.(tgbotapi.MessageConfig).Text
	assert.Contains(t, msgText, "Assigned chore to")
	assert.Contains(t, msgText, "Admin")
	assert.Contains(t, msgText, "Clean the kitchen")
	assert.Contains(t, msgText, "Recurring chore scheduled")
	assert.Contains(t, msgText, "42")
	assert.Contains(t, msgText, "every 5 days")
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

	msgText := response.(tgbotapi.MessageConfig).Text
	assert.NotContains(t, msgText, "Assigned chore to")
	assert.Contains(t, msgText, "Clean the kitchen")
	assert.Contains(t, msgText, "Recurring chore scheduled")
	assert.Contains(t, msgText, "42")
	assert.Contains(t, msgText, "every 5 days")
}

func TestHandleList_Interactive(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	adminUser := &store.User{ID: 1, TelegramUserID: 123, IsAdmin: true}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(123)).Return(adminUser, nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 789},
		From: &tgbotapi.User{ID: 123},
		Text: "/list",
		Entities: []tgbotapi.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: 5},
		},
	}

	response, err := h.HandleList(message)
	assert.NoError(t, err)

	assert.Contains(t, response.Text, "Select which type of chores you want to list:")

	keyboard, ok := response.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)
	assert.True(t, ok)
	assert.Len(t, keyboard.InlineKeyboard, 2)
	assert.Equal(t, "list:chore", *keyboard.InlineKeyboard[0][0].CallbackData)
	assert.Equal(t, "list:task", *keyboard.InlineKeyboard[1][0].CallbackData)
}

func TestHandleListCallback(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	adminUser := &store.User{ID: 1, TelegramUserID: 123, IsAdmin: true}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(123)).Return(adminUser, nil)

	chores := []*store.RecurringChore{
		{ID: 1, Description: "Foo", Interval: 3, NextRunAt: time.Now()},
	}
	mockStore.On("GetActiveRecurringChores", mock.Anything).Return(chores, nil)

	cb := &tgbotapi.CallbackQuery{
		ID:   "123",
		From: &tgbotapi.User{ID: 123},
		Message: &tgbotapi.Message{
			Chat:      &tgbotapi.Chat{ID: 789},
			MessageID: 10,
		},
		Data: "list:chore",
	}

	response, err := h.HandleListCallback(cb)
	assert.NoError(t, err)

	editConfig, ok := response.(tgbotapi.EditMessageTextConfig)
	assert.True(t, ok)
	assert.Contains(t, editConfig.Text, "Active Recurring Chores")
	assert.Contains(t, editConfig.Text, "Foo")
}

func TestHandleListCallback_Task(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	adminUser := &store.User{ID: 1, TelegramUserID: 123, IsAdmin: true}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(123)).Return(adminUser, nil)

	chores := []*store.Chore{
		{ID: 1, Description: "Bar Task", AssignedAt: time.Now(), User: &store.User{FirstName: "Bob"}},
	}
	mockStore.On("ListActiveChores", mock.Anything).Return(chores, nil)

	cb := &tgbotapi.CallbackQuery{
		ID:   "123",
		From: &tgbotapi.User{ID: 123},
		Message: &tgbotapi.Message{
			Chat:      &tgbotapi.Chat{ID: 789},
			MessageID: 10,
		},
		Data: "list:task",
	}

	response, err := h.HandleListCallback(cb)
	assert.NoError(t, err)

	editConfig, ok := response.(tgbotapi.EditMessageTextConfig)
	assert.True(t, ok)
	assert.Contains(t, editConfig.Text, "Active Regular Chores")
	assert.Contains(t, editConfig.Text, "Bar Task")
	assert.Contains(t, editConfig.Text, "Bob")
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

func TestHandleEdit_Recurring_Success(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	adminUser := &store.User{ID: 1, TelegramUserID: 123, IsAdmin: true}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(123)).Return(adminUser, nil)

	chore := &store.RecurringChore{ID: 1, Description: "Old Foo", IsActive: true}
	mockStore.On("GetRecurringChore", mock.Anything, int64(1)).Return(chore, nil)
	mockStore.On("UpdateRecurringChoreDescription", mock.Anything, int64(1), "New Foo Description").Return(nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 789},
		From: &tgbotapi.User{ID: 123},
		Text: "/edit chore 1 New Foo Description",
		Entities: []tgbotapi.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: 5},
		},
	}

	response, err := h.HandleEdit(message)
	assert.NoError(t, err)

	assert.Contains(t, response.Text, "Periodic chore description updated")
	assert.Contains(t, response.Text, "Old Foo")
	assert.Contains(t, response.Text, "New Foo Description")
}

func TestHandleEdit_Recurring_InteractiveList(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	adminUser := &store.User{ID: 1, TelegramUserID: 123, IsAdmin: true}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(123)).Return(adminUser, nil)

	chores := []*store.RecurringChore{
		{ID: 1, Description: "First Chore", IsActive: true},
		{ID: 2, Description: "A very long chore description that should be truncated because it is more than 30 characters long", IsActive: true},
	}
	mockStore.On("GetActiveRecurringChores", mock.Anything).Return(chores, nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 789},
		From: &tgbotapi.User{ID: 123},
		Text: "/edit",
		Entities: []tgbotapi.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: 5},
		},
	}

	response, err := h.HandleEdit(message)
	assert.NoError(t, err)

	assert.Contains(t, response.Text, "Select a chore to edit")

	keyboard, ok := response.ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)
	assert.True(t, ok)
	assert.Len(t, keyboard.InlineKeyboard, 2)
	assert.Equal(t, "edit_chore:1", *keyboard.InlineKeyboard[0][0].CallbackData)
	assert.Contains(t, keyboard.InlineKeyboard[1][0].Text, "A very long chore descripti...")
}

func TestHandleEdit_Recurring_InteractiveList_Empty(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	adminUser := &store.User{ID: 1, TelegramUserID: 123, IsAdmin: true}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(123)).Return(adminUser, nil)

	var chores []*store.RecurringChore
	mockStore.On("GetActiveRecurringChores", mock.Anything).Return(chores, nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 789},
		From: &tgbotapi.User{ID: 123},
		Text: "/edit",
		Entities: []tgbotapi.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: 5},
		},
	}

	response, err := h.HandleEdit(message)
	assert.NoError(t, err)

	assert.Contains(t, response.Text, "No active recurring chores found to edit.")
}

func TestHandleEditChoreCallback(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	adminUser := &store.User{ID: 1, TelegramUserID: 123, IsAdmin: true}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(123)).Return(adminUser, nil)

	chore := &store.RecurringChore{ID: 1, Description: "Test Chore", IsActive: true}
	mockStore.On("GetRecurringChore", mock.Anything, int64(1)).Return(chore, nil)

	cb := &tgbotapi.CallbackQuery{
		ID:   "123",
		From: &tgbotapi.User{ID: 123},
		Message: &tgbotapi.Message{
			Chat:      &tgbotapi.Chat{ID: 789},
			MessageID: 10,
		},
		Data: "edit_chore:1",
	}

	response, err := h.HandleEditChoreCallback(cb)
	assert.NoError(t, err)

	msgConfig, ok := response.(tgbotapi.MessageConfig)
	assert.True(t, ok)
	assert.Contains(t, msgConfig.Text, "Test Chore")
	assert.Contains(t, msgConfig.Text, "reply with the new description")

	// Ensure session was started
	session, exists := h.SessionManager.GetSession(789)
	assert.True(t, exists)
	assert.Equal(t, handlers.SessionTypeEditChore, session.Type)

	val, _ := session.GetData("chore_id")
	assert.Equal(t, int64(1), val)
}

func TestHandleEditChoreInteractive_Success(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	// Start a session
	h.SessionManager.StartSession(789, 123, handlers.SessionTypeEditChore)
	session, _ := h.SessionManager.GetSession(789)
	session.SetData("chore_id", int64(1))
	session.SetData("old_description", "Old Test Chore")

	mockStore.On("UpdateRecurringChoreDescription", mock.Anything, int64(1), "New Updated Description").Return(nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 789},
		From: &tgbotapi.User{ID: 123},
		Text: "New Updated Description",
	}

	response, err := h.HandleEditChoreInteractive(message)
	assert.NoError(t, err)

	msgConfig, ok := response.(tgbotapi.MessageConfig)
	assert.True(t, ok)
	assert.Contains(t, msgConfig.Text, "updated interactively")
	assert.Contains(t, msgConfig.Text, "Old Test Chore")
	assert.Contains(t, msgConfig.Text, "New Updated Description")

	// Ensure session is ended
	_, exists := h.SessionManager.GetSession(789)
	assert.False(t, exists)
}

func TestHandleEditChoreInteractive_Cancel(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	// Start a session
	h.SessionManager.StartSession(789, 123, handlers.SessionTypeEditChore)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 789},
		From: &tgbotapi.User{ID: 123},
		Text: "/cancel",
	}

	response, err := h.HandleEditChoreInteractive(message)
	assert.NoError(t, err)

	msgConfig, ok := response.(tgbotapi.MessageConfig)
	assert.True(t, ok)
	assert.Contains(t, msgConfig.Text, "cancelled")

	// Ensure session is ended
	_, exists := h.SessionManager.GetSession(789)
	assert.False(t, exists)
}

func TestHandleEdit_Recurring_InvalidFormat(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	adminUser := &store.User{ID: 1, TelegramUserID: 123, IsAdmin: true}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(123)).Return(adminUser, nil)

	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "Missing description",
			text: "/edit chore 1",
			want: "Use /edit chore <id> <new description>",
		},
		{
			name: "Invalid ID",
			text: "/edit chore abc new desc",
			want: "❌ Invalid chore ID format",
		},
		{
			name: "Wrong subcommand",
			text: "/edit task 1 new desc",
			want: "Use /edit chore <id> <new description>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message := &tgbotapi.Message{
				Chat: &tgbotapi.Chat{ID: 789},
				From: &tgbotapi.User{ID: 123},
				Text: tt.text,
				Entities: []tgbotapi.MessageEntity{
					{Type: "bot_command", Offset: 0, Length: 5},
				},
			}

			response, err := h.HandleEdit(message)
			assert.NoError(t, err)
			assert.Contains(t, response.Text, tt.want)
		})
	}
}

func TestHandleEdit_Recurring_NonAdmin(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	normalUser := &store.User{ID: 1, TelegramUserID: 456, IsAdmin: false}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(456)).Return(normalUser, nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 789},
		From: &tgbotapi.User{ID: 456},
		Text: "/edit chore 1 New Foo Description",
		Entities: []tgbotapi.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: 5},
		},
	}

	response, err := h.HandleEdit(message)
	assert.NoError(t, err)

	assert.Contains(t, response.Text, "Sorry, this command is for admins only.")
}

func TestHandleEdit_Recurring_NotFoundOrInactive(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	adminUser := &store.User{ID: 1, TelegramUserID: 123, IsAdmin: true}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(123)).Return(adminUser, nil)

	tests := []struct {
		name  string
		chore *store.RecurringChore
		err   error
	}{
		{
			name:  "Not found",
			chore: nil,
			err:   nil,
		},
		{
			name:  "Inactive",
			chore: &store.RecurringChore{ID: 1, IsActive: false},
			err:   nil,
		},
		{
			name:  "DB Error",
			chore: nil,
			err:   assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mock
			mockStore.ExpectedCalls = nil
			mockStore.On("GetUserByTelegramID", mock.Anything, int64(123)).Return(adminUser, nil)
			mockStore.On("GetRecurringChore", mock.Anything, int64(1)).Return(tt.chore, tt.err)

			message := &tgbotapi.Message{
				Chat: &tgbotapi.Chat{ID: 789},
				From: &tgbotapi.User{ID: 123},
				Text: "/edit chore 1 new desc",
				Entities: []tgbotapi.MessageEntity{
					{Type: "bot_command", Offset: 0, Length: 5},
				},
			}

			response, err := h.HandleEdit(message)
			assert.NoError(t, err)

			if tt.err != nil {
				assert.Contains(t, response.Text, "Failed to retrieve")
			} else {
				assert.Contains(t, response.Text, "not found or already cancelled")
			}
		})
	}
}
