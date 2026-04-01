package handlers_test

import (
	"fmt"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/mocks"
	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/korjavin/dutyassistant/internal/telegram/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestHandleSviniya_BalancesExist(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
		From: &tgbotapi.User{ID: 456, FirstName: "TestUser"},
	}

	balances := []*store.SviniyaBalance{
		{UserID: 1, UserName: "Ivan", Balance: 3},
		{UserID: 2, UserName: "Maria", Balance: 1},
		{UserID: 3, UserName: "Peter", Balance: 0},
	}
	mockStore.On("GetAllSviniyaBalances", mock.Anything).Return(balances, nil)

	msg, err := h.HandleSviniya(message)
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "Sviniya Balances")
	assert.Contains(t, msg.Text, "Ivan: 3 sviniyas")
	assert.Contains(t, msg.Text, "Maria: 1 sviniya")
	assert.Contains(t, msg.Text, "Peter: 0 sviniyas")
	assert.Equal(t, tgbotapi.ModeHTML, msg.ParseMode)
	mockStore.AssertExpectations(t)
}

func TestHandleSviniya_NoBalances(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
		From: &tgbotapi.User{ID: 456, FirstName: "TestUser"},
	}

	mockStore.On("GetAllSviniyaBalances", mock.Anything).Return([]*store.SviniyaBalance{}, nil)

	msg, err := h.HandleSviniya(message)
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "No sviniya balances found")
	mockStore.AssertExpectations(t)
}

func TestHandleSviniya_StoreError(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
		From: &tgbotapi.User{ID: 456, FirstName: "TestUser"},
	}

	mockStore.On("GetAllSviniyaBalances", mock.Anything).Return([]*store.SviniyaBalance{}, assert.AnError)

	msg, err := h.HandleSviniya(message)
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "Failed to fetch sviniya balances")
	mockStore.AssertExpectations(t)
}

func TestHandleSviniya_HTMLEscaping(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
		From: &tgbotapi.User{ID: 456, FirstName: "TestUser"},
	}

	balances := []*store.SviniyaBalance{
		{UserID: 1, UserName: "<User>", Balance: 1},
		{UserID: 2, UserName: "A&B", Balance: 2},
	}
	mockStore.On("GetAllSviniyaBalances", mock.Anything).Return(balances, nil)

	msg, err := h.HandleSviniya(message)
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "&lt;User&gt;: 1 sviniya")
	assert.Contains(t, msg.Text, "A&amp;B: 2 sviniyas")
	mockStore.AssertExpectations(t)
}

func TestHandleSetSviniyaBalance_Admin_Success(t *testing.T) {
	mockStore := new(mocks.MockStore)
	adminID := int64(999)
	h := handlers.NewWithAdminID(mockStore, nil, 0, adminID, nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
		From: &tgbotapi.User{ID: adminID, FirstName: "AdminUser"},
		Text:     "/set_sviniya_balance Ivan 3",
		Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 20}},
	}

	user := &store.User{ID: 1, TelegramUserID: 111, FirstName: "Ivan", IsAdmin: false, IsActive: true}
	mockStore.On("GetUserByName", mock.Anything, "Ivan").Return(user, nil)
	mockStore.On("SetSviniyaBalance", mock.Anything, int64(1), 3).Return(nil)

	msg, err := h.HandleSetSviniyaBalance(message)
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "Set sviniya balance for Ivan to 3")
	mockStore.AssertExpectations(t)
}

func TestHandleSetSviniyaBalance_NotAdmin(t *testing.T) {
	mockStore := new(mocks.MockStore)
	adminID := int64(999)
	h := handlers.NewWithAdminID(mockStore, nil, 0, adminID, nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
		From: &tgbotapi.User{ID: 123, FirstName: "RegularUser"},
		Text:     "/set_sviniya_balance Ivan 3",
		Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 20}},
	}

	msg, err := h.HandleSetSviniyaBalance(message)
	assert.NoError(t, err)
	assert.Equal(t, "Sorry, this command is for admins only.", msg.Text)
	mockStore.AssertNotCalled(t, "GetUserByName", mock.Anything, mock.Anything)
	mockStore.AssertNotCalled(t, "SetSviniyaBalance", mock.Anything, mock.Anything, mock.Anything)
}

func TestHandleSetSviniyaBalance_NoArguments(t *testing.T) {
	mockStore := new(mocks.MockStore)
	adminID := int64(999)
	h := handlers.NewWithAdminID(mockStore, nil, 0, adminID, nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
		From: &tgbotapi.User{ID: adminID, FirstName: "AdminUser"},
		Text:     "/set_sviniya_balance",
		Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 20}},
	}

	msg, err := h.HandleSetSviniyaBalance(message)
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "Usage:")
	mockStore.AssertNotCalled(t, "GetUserByName", mock.Anything, mock.Anything)
}

func TestHandleSetSviniyaBalance_InvalidBalance(t *testing.T) {
	mockStore := new(mocks.MockStore)
	adminID := int64(999)
	h := handlers.NewWithAdminID(mockStore, nil, 0, adminID, nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
		From: &tgbotapi.User{ID: adminID, FirstName: "AdminUser"},
		Text:     "/set_sviniya_balance Ivan abc",
		Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 20}},
	}

	msg, err := h.HandleSetSviniyaBalance(message)
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "Invalid balance value")
	mockStore.AssertNotCalled(t, "GetUserByName", mock.Anything, mock.Anything)
}

func TestHandleSetSviniyaBalance_UserNotFound(t *testing.T) {
	mockStore := new(mocks.MockStore)
	adminID := int64(999)
	h := handlers.NewWithAdminID(mockStore, nil, 0, adminID, nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
		From: &tgbotapi.User{ID: adminID, FirstName: "AdminUser"},
		Text:     "/set_sviniya_balance NonExistent 3",
		Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 20}},
	}

	mockStore.On("GetUserByName", mock.Anything, "NonExistent").Return(nil, nil)

	msg, err := h.HandleSetSviniyaBalance(message)
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "User 'NonExistent' not found")
	mockStore.AssertExpectations(t)
}

func TestHandleSetSviniyaBalance_StoreError(t *testing.T) {
	mockStore := new(mocks.MockStore)
	adminID := int64(999)
	h := handlers.NewWithAdminID(mockStore, nil, 0, adminID, nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
		From: &tgbotapi.User{ID: adminID, FirstName: "AdminUser"},
		Text:     "/set_sviniya_balance Ivan 3",
		Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 20}},
	}

	user := &store.User{ID: 1, TelegramUserID: 111, FirstName: "Ivan", IsAdmin: false, IsActive: true}
	mockStore.On("GetUserByName", mock.Anything, "Ivan").Return(user, nil)
	mockStore.On("SetSviniyaBalance", mock.Anything, int64(1), 3).Return(assert.AnError)

	msg, err := h.HandleSetSviniyaBalance(message)
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "Failed to set sviniya balance")
	mockStore.AssertExpectations(t)
}

func TestHandleSetSviniyaBalance_AdminIDNotConfigured(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
		From: &tgbotapi.User{ID: 456, FirstName: "RegularUser"},
		Text:     "/set_sviniya_balance Ivan 3",
		Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 20}},
	}

	// When AdminID is 0, it falls back to database flag
	nonAdminUser := &store.User{ID: 1, TelegramUserID: 456, FirstName: "RegularUser", IsAdmin: false, IsActive: true}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(456)).Return(nonAdminUser, nil)

	msg, err := h.HandleSetSviniyaBalance(message)
	assert.NoError(t, err)
	assert.Equal(t, "Sorry, this command is for admins only.", msg.Text)
	mockStore.AssertExpectations(t)
}

func TestHandleSpend_ZeroBalance(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
		From: &tgbotapi.User{ID: 456, FirstName: "TestUser"},
		Text: "/spend",
	}

	// First, GetUserByTelegramID is called to get internal ID
	user := &store.User{ID: 1, TelegramUserID: 456, FirstName: "TestUser", IsAdmin: false, IsActive: true}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(456)).Return(user, nil).Once()

	// Return balance of 0 using internal ID
	mockStore.On("GetSviniyaBalance", mock.Anything, int64(1)).Return(&store.SviniyaBalance{UserID: 1, Balance: 0}, nil)

	msg, err := h.HandleSpend(message)
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "no sviniyas")
	mockStore.AssertExpectations(t)
}

func TestHandleSpend_NilBalance(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
		From: &tgbotapi.User{ID: 456, FirstName: "TestUser"},
		Text: "/spend",
	}

	// First, GetUserByTelegramID is called to get internal ID
	user := &store.User{ID: 1, TelegramUserID: 456, FirstName: "TestUser", IsAdmin: false, IsActive: true}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(456)).Return(user, nil).Once()

	// Return nil balance (user doesn't exist in sviniya_balances)
	mockStore.On("GetSviniyaBalance", mock.Anything, int64(1)).Return(nil, nil)

	msg, err := h.HandleSpend(message)
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "no sviniyas")
	mockStore.AssertExpectations(t)
}

func TestHandleSpend_WithInlineDescription(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil) // nil LLM client tests fallback

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
		From: &tgbotapi.User{ID: 456, FirstName: "TestUser"},
		Text:     "/spend coffee for everyone",
		Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 6}},
	}

	user := &store.User{ID: 1, TelegramUserID: 456, FirstName: "TestUser", IsAdmin: false, IsActive: true}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(456)).Return(user, nil).Twice()
	mockStore.On("GetSviniyaBalance", mock.Anything, int64(1)).Return(&store.SviniyaBalance{UserID: 1, Balance: 5}, nil)
	mockStore.On("DecrementSviniyaBalance", mock.Anything, int64(1)).Return(nil)

	msg, err := h.HandleSpend(message)
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "Spent 1 sviniya")
	assert.Contains(t, msg.Text, "coffee for everyone")
	// When Bot is nil, announcement is not sent, so message should NOT contain "Announcement sent"
	assert.NotContains(t, msg.Text, "Announcement sent")
	mockStore.AssertExpectations(t)
}

func TestHandleSpend_InteractiveMode_HasBalance(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
		From: &tgbotapi.User{ID: 456, FirstName: "TestUser"},
		Text: "/spend",
	}

	// First, GetUserByTelegramID is called to get internal ID
	user := &store.User{ID: 1, TelegramUserID: 456, FirstName: "TestUser", IsAdmin: false, IsActive: true}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(456)).Return(user, nil).Once()

	// Return balance > 0 using internal ID
	mockStore.On("GetSviniyaBalance", mock.Anything, int64(1)).Return(&store.SviniyaBalance{UserID: 1, Balance: 3}, nil)

	msg, err := h.HandleSpend(message)
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "You have 3 sviniya(s)")
	assert.Contains(t, msg.Text, "What would you like to spend it on?")
	assert.Contains(t, msg.Text, "/cancel")

	// Check that session was started
	session, exists := h.SessionManager.GetSession(123)
	assert.True(t, exists, "Session should be started")
	assert.Equal(t, handlers.SessionTypeSpendSviniya, session.Type)

	mockStore.AssertExpectations(t)
}

func TestHandleSpendInteractive_Cancel(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	// Start a session first
	h.SessionManager.StartSession(123, 456, handlers.SessionTypeSpendSviniya)

	message := &tgbotapi.Message{
		Chat:     &tgbotapi.Chat{ID: 123},
		From:     &tgbotapi.User{ID: 456, FirstName: "TestUser"},
		Text:     "/cancel",
		Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 7}},
	}

	msg, err := h.HandleSpendInteractive(message)
	assert.NoError(t, err)
	msgConfig := msg.(tgbotapi.MessageConfig)
	assert.Contains(t, msgConfig.Text, "cancelled")

	// Check that session was ended
	_, exists := h.SessionManager.GetSession(123)
	assert.False(t, exists, "Session should be ended")
}

func TestHandleSpendInteractive_HappyPath(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil) // nil LLM client tests fallback

	// Start a session first
	h.SessionManager.StartSession(123, 456, handlers.SessionTypeSpendSviniya)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
		From: &tgbotapi.User{ID: 456, FirstName: "TestUser"},
		Text: "a fancy dinner",
	}

	user := &store.User{ID: 1, TelegramUserID: 456, FirstName: "TestUser", IsAdmin: false, IsActive: true}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(456)).Return(user, nil)
	mockStore.On("DecrementSviniyaBalance", mock.Anything, int64(1)).Return(nil)

	msg, err := h.HandleSpendInteractive(message)
	assert.NoError(t, err)
	msgConfig := msg.(tgbotapi.MessageConfig)
	assert.Contains(t, msgConfig.Text, "Spent 1 sviniya")
	assert.Contains(t, msgConfig.Text, "a fancy dinner")
	// When Bot is nil, announcement is not sent, so message should NOT contain "Announcement sent"
	assert.NotContains(t, msgConfig.Text, "Announcement sent")

	// Check that session was ended
	_, exists := h.SessionManager.GetSession(123)
	assert.False(t, exists, "Session should be ended")

	mockStore.AssertExpectations(t)
}

func TestHandleSpend_NoLLMFallback(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil) // No LLM client

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
		From: &tgbotapi.User{ID: 456, FirstName: "TestUser"},
		Text:     "/spend pizza party",
		Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 6}},
	}

	user := &store.User{ID: 1, TelegramUserID: 456, FirstName: "TestUser", IsAdmin: false, IsActive: true}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(456)).Return(user, nil).Twice()
	mockStore.On("GetSviniyaBalance", mock.Anything, int64(1)).Return(&store.SviniyaBalance{UserID: 1, Balance: 5}, nil)
	mockStore.On("DecrementSviniyaBalance", mock.Anything, int64(1)).Return(nil)

	msg, err := h.HandleSpend(message)
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "Spent 1 sviniya")
	assert.Contains(t, msg.Text, "pizza party")
	mockStore.AssertExpectations(t)
}

func TestHandleSpend_GetBalanceError(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
		From: &tgbotapi.User{ID: 456, FirstName: "TestUser"},
		Text: "/spend",
	}

	user := &store.User{ID: 1, TelegramUserID: 456, FirstName: "TestUser", IsAdmin: false, IsActive: true}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(456)).Return(user, nil).Once()
	mockStore.On("GetSviniyaBalance", mock.Anything, int64(1)).Return(nil, assert.AnError)

	msg, err := h.HandleSpend(message)
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "Failed to check your sviniya balance")
	mockStore.AssertExpectations(t)
}

func TestHandleSpendInteractive_GetUserError(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	// Start a session first
	h.SessionManager.StartSession(123, 456, handlers.SessionTypeSpendSviniya)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
		From: &tgbotapi.User{ID: 456, FirstName: "TestUser"},
		Text: "something nice",
	}

	mockStore.On("GetUserByTelegramID", mock.Anything, int64(456)).Return(nil, assert.AnError)

	msg, err := h.HandleSpendInteractive(message)
	assert.NoError(t, err)
	msgConfig := msg.(tgbotapi.MessageConfig)
	assert.Contains(t, msgConfig.Text, "Failed to retrieve your user information")
	mockStore.AssertExpectations(t)
}

func TestHandleSpendInteractive_DecrementError(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	// Start a session first
	h.SessionManager.StartSession(123, 456, handlers.SessionTypeSpendSviniya)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
		From: &tgbotapi.User{ID: 456, FirstName: "TestUser"},
		Text: "chocolate",
	}

	user := &store.User{ID: 1, TelegramUserID: 456, FirstName: "TestUser", IsAdmin: false, IsActive: true}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(456)).Return(user, nil)
	mockStore.On("DecrementSviniyaBalance", mock.Anything, int64(1)).Return(assert.AnError)

	msg, err := h.HandleSpendInteractive(message)
	assert.NoError(t, err)
	msgConfig := msg.(tgbotapi.MessageConfig)
	assert.Contains(t, msgConfig.Text, "Failed to spend sviniya")
	mockStore.AssertExpectations(t)
}

func TestHandleSpendInteractive_WrongUser(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	// Start a session first with user 456
	h.SessionManager.StartSession(123, 456, handlers.SessionTypeSpendSviniya)

	// Try to spend from a different user (789)
	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
		From: &tgbotapi.User{ID: 789, FirstName: "OtherUser"},
		Text: "something nice",
	}

	msg, err := h.HandleSpendInteractive(message)
	assert.NoError(t, err)
	// Message should be ignored (nil) for non-owners
	assert.Nil(t, msg)

	// Check that session is still active (not ended)
	session, exists := h.SessionManager.GetSession(123)
	assert.True(t, exists, "Session should still be active")
	assert.Equal(t, int64(456), session.UserID)
}

func TestHandleSpendInteractive_NoSession(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	// Don't start a session

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
		From: &tgbotapi.User{ID: 456, FirstName: "TestUser"},
		Text: "something nice",
	}

	msg, err := h.HandleSpendInteractive(message)
	assert.NoError(t, err)
	msgConfig := msg.(tgbotapi.MessageConfig)
	assert.Contains(t, msgConfig.Text, "No active spend session")
}

func TestHandleSpendInteractive_EmptyText(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	// Start a session first
	h.SessionManager.StartSession(123, 456, handlers.SessionTypeSpendSviniya)

	// Send a message with empty text (e.g., a sticker or photo)
	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
		From: &tgbotapi.User{ID: 456, FirstName: "TestUser"},
		Text: "",
	}

	msg, err := h.HandleSpendInteractive(message)
	assert.NoError(t, err)
	msgConfig := msg.(tgbotapi.MessageConfig)
	assert.Contains(t, msgConfig.Text, "Please send a text description")

	// Check that session is still active (not ended)
	_, exists := h.SessionManager.GetSession(123)
	assert.True(t, exists, "Session should still be active")
}

func TestHandleSpend_ExistingSessionBySameUser(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	// Start an existing session (could be any session type)
	h.SessionManager.StartSession(123, 456, handlers.SessionTypeChoreCreation)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
		From: &tgbotapi.User{ID: 456, FirstName: "TestUser"},
		Text: "/spend",
	}

	// First, GetUserByTelegramID is called to get internal ID
	user := &store.User{ID: 1, TelegramUserID: 456, FirstName: "TestUser", IsAdmin: false, IsActive: true}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(456)).Return(user, nil).Once()

	// Return balance > 0 using internal ID (won't actually check because of existing session)
	mockStore.On("GetSviniyaBalance", mock.Anything, int64(1)).Return(&store.SviniyaBalance{UserID: 1, Balance: 3}, nil)

	msg, err := h.HandleSpend(message)
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "You already have an active session")

	// Check that original chore creation session is still active
	session, exists := h.SessionManager.GetSession(123)
	assert.True(t, exists, "Session should still be active")
	assert.Equal(t, handlers.SessionTypeChoreCreation, session.Type)
}

func TestHandleSpend_ExistingSessionByDifferentUser(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	// Start an existing session by a different user
	h.SessionManager.StartSession(123, 789, handlers.SessionTypeChoreCreation)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
		From: &tgbotapi.User{ID: 456, FirstName: "TestUser"},
		Text: "/spend",
	}

	// First, GetUserByTelegramID is called to get internal ID
	user := &store.User{ID: 1, TelegramUserID: 456, FirstName: "TestUser", IsAdmin: false, IsActive: true}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(456)).Return(user, nil).Once()

	// Return balance > 0 using internal ID (won't actually check because of existing session)
	mockStore.On("GetSviniyaBalance", mock.Anything, int64(1)).Return(&store.SviniyaBalance{UserID: 1, Balance: 3}, nil)

	msg, err := h.HandleSpend(message)
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "Another user has an active session")

	// Check that original session is still active
	session, exists := h.SessionManager.GetSession(123)
	assert.True(t, exists, "Session should still be active")
	assert.Equal(t, int64(789), session.UserID)
}

func TestHandleSpendInteractive_InsufficientBalanceError(t *testing.T) {
	mockStore := new(mocks.MockStore)
	h := handlers.New(mockStore, nil, 0, nil)

	// Start a session first
	h.SessionManager.StartSession(123, 456, handlers.SessionTypeSpendSviniya)

	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
		From: &tgbotapi.User{ID: 456, FirstName: "TestUser"},
		Text: "chocolate",
	}

	user := &store.User{ID: 1, TelegramUserID: 456, FirstName: "TestUser", IsAdmin: false, IsActive: true}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(456)).Return(user, nil)
	// Return insufficient balance error
	mockStore.On("DecrementSviniyaBalance", mock.Anything, int64(1)).Return(fmt.Errorf("insufficient sviniya balance for user 1"))

	msg, err := h.HandleSpendInteractive(message)
	assert.NoError(t, err)
	msgConfig := msg.(tgbotapi.MessageConfig)
	assert.Contains(t, msgConfig.Text, "no sviniyas")

	// Check that session was ended
	_, exists := h.SessionManager.GetSession(123)
	assert.False(t, exists, "Session should be ended")
}
