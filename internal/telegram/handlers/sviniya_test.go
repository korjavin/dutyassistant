package handlers_test

import (
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
	assert.Contains(t, msg.Text, "&amp;lt;User&amp;gt;: 1 sviniya")
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
