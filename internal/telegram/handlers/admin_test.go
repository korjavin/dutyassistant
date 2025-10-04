package handlers_test

import (
	"context"
	"errors"
	"testing"

	"github.com/korjavin/dutyassistant/internal/mocks"
	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/korjavin/dutyassistant/internal/telegram/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func TestHandleAssign_IsAdmin(t *testing.T) {
	// Setup
	mockStore := new(mocks.MockStore)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.New(mockStore, mockScheduler)

	adminUser := &tgbotapi.User{ID: 1, FirstName: "Admin"}
	message := &tgbotapi.Message{
		Chat:     &tgbotapi.Chat{ID: 123},
		From:     adminUser,
		Text:     "/assign TestUser 2023-12-25",
		Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 7}},
	}

	// Mock expectations
	adminStoreUser := &store.User{ID: 1, TelegramUserID: 1, IsAdmin: true}
	targetUser := &store.User{ID: 2, FirstName: "TestUser"}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(1)).Return(adminStoreUser, nil)
	mockStore.On("GetUserByName", mock.Anything, "TestUser").Return(targetUser, nil)
	mockScheduler.On("AssignDuty", mock.Anything, targetUser, "2023-12-25").Return(nil)

	// Execute
	msg, err := h.HandleAssign(message)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, int64(123), msg.ChatID)
	assert.Contains(t, msg.Text, "Successfully assigned TestUser to duty on 2023-12-25")
	mockStore.AssertExpectations(t)
	mockScheduler.AssertExpectations(t)
}

func TestHandleModify_Success(t *testing.T) {
	// Setup
	mockStore := new(mocks.MockStore)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.New(mockStore, mockScheduler)

	adminUser := &tgbotapi.User{ID: 1, FirstName: "Admin"}
	message := &tgbotapi.Message{
		Chat:     &tgbotapi.Chat{ID: 123},
		From:     adminUser,
		Text:     "/modify 2023-12-25 NewUser",
		Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 7}},
	}

	// Mock expectations
	adminStoreUser := &store.User{ID: 1, TelegramUserID: 1, IsAdmin: true}
	targetUser := &store.User{ID: 3, FirstName: "NewUser"}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(1)).Return(adminStoreUser, nil)
	mockStore.On("GetUserByName", mock.Anything, "NewUser").Return(targetUser, nil)
	mockScheduler.On("AssignDuty", mock.Anything, targetUser, "2023-12-25").Return(nil)

	// Execute
	msg, err := h.HandleModify(message)

	// Assert
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "Successfully modified duty for 2023-12-25 to be handled by NewUser")
	mockStore.AssertExpectations(t)
	mockScheduler.AssertExpectations(t)
}

func TestHandleAssign_NotAdmin(t *testing.T) {
	// Setup
	mockStore := new(mocks.MockStore)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.New(mockStore, mockScheduler)

	nonAdminUser := &tgbotapi.User{ID: 2, FirstName: "User"}
	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
		From: nonAdminUser,
		Text: "/assign TestUser 2023-12-25",
	}
	message.Entities = []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 7}}

	// Mock expectations
	expectedUser := &store.User{ID: 2, TelegramUserID: 2, IsAdmin: false}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(2)).Return(expectedUser, nil)

	// Execute
	msg, err := h.HandleAssign(message)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, int64(123), msg.ChatID)
	assert.Equal(t, "Sorry, this command is for admins only.", msg.Text)
	mockStore.AssertExpectations(t)
}

func TestHandleAssign_UserNotFound(t *testing.T) {
	// Setup
	mockStore := new(mocks.MockStore)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.New(mockStore, mockScheduler)

	adminUser := &tgbotapi.User{ID: 1, FirstName: "Admin"}
	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
		From: adminUser,
		Text: "/assign TestUser 2023-12-25",
	}
	message.Entities = []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 7}}

	// Mock expectations
	storeError := errors.New("user not found")
	mockStore.On("GetUserByTelegramID", context.Background(), int64(1)).Return(nil, storeError)

	// Execute
	msg, err := h.HandleAssign(message)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "Sorry, this command is for admins only.", msg.Text)
	mockStore.AssertExpectations(t)
}

func TestHandleAssign_InvalidArguments(t *testing.T) {
	// Setup
	mockStore := new(mocks.MockStore)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.New(mockStore, mockScheduler)

	adminUser := &tgbotapi.User{ID: 1, FirstName: "Admin"}
	message := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: 123},
		From: adminUser,
		Text: "/assign TestUser", // Missing date argument
	}
	message.Entities = []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 7}}

	// Mock expectations
	expectedAdmin := &store.User{ID: 1, TelegramUserID: 1, IsAdmin: true}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(1)).Return(expectedAdmin, nil)

	// Execute
	msg, err := h.HandleAssign(message)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "Invalid command format. Use /help for more information.", msg.Text)
	mockStore.AssertExpectations(t)
}

func TestHandleUsers_Success(t *testing.T) {
	// Setup
	mockStore := new(mocks.MockStore)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.New(mockStore, mockScheduler)

	adminUser := &tgbotapi.User{ID: 1, FirstName: "Admin"}
	message := &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: 123}, From: adminUser}

	// Mock expectations
	expectedAdmin := &store.User{ID: 1, TelegramUserID: 1, IsAdmin: true}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(1)).Return(expectedAdmin, nil)

	userList := []*store.User{
		{FirstName: "Alice", IsActive: true, IsAdmin: true},
		{FirstName: "Bob", IsActive: false, IsAdmin: false},
	}
	mockStore.On("ListAllUsers", mock.Anything).Return(userList, nil)

	// Execute
	msg, err := h.HandleUsers(message)

	// Assert
	assert.NoError(t, err)
	assert.Contains(t, msg.Text, "<b>User List:</b>")
	assert.Contains(t, msg.Text, "- Alice (Admin): Active")
	assert.Contains(t, msg.Text, "- Bob: Inactive")
	assert.Equal(t, tgbotapi.ModeHTML, msg.ParseMode)
	mockStore.AssertExpectations(t)
}

func TestHandleToggleActive_Success(t *testing.T) {
	// Setup
	mockStore := new(mocks.MockStore)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.New(mockStore, mockScheduler)

	adminUser := &tgbotapi.User{ID: 1, FirstName: "Admin"}
	message := &tgbotapi.Message{
		Chat:     &tgbotapi.Chat{ID: 123},
		From:     adminUser,
		Text:     "/toggle_active Bob",
		Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 14}},
	}

	// Mock expectations
	adminStoreUser := &store.User{ID: 1, TelegramUserID: 1, IsAdmin: true}
	bobStoreUser := &store.User{ID: 2, FirstName: "Bob", IsActive: true}

	mockStore.On("GetUserByTelegramID", mock.Anything, int64(1)).Return(adminStoreUser, nil)
	mockStore.On("GetUserByName", mock.Anything, "Bob").Return(bobStoreUser, nil)
	mockStore.On("UpdateUser", mock.Anything, mock.MatchedBy(func(u *store.User) bool {
		return u.ID == 2 && !u.IsActive
	})).Return(nil)

	// Execute
	msg, err := h.HandleToggleActive(message)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "Successfully set status for Bob to Inactive.", msg.Text)
	mockStore.AssertExpectations(t)
}

func TestHandleToggleActive_UserNotFound(t *testing.T) {
	// Setup
	mockStore := new(mocks.MockStore)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.New(mockStore, mockScheduler)

	adminUser := &tgbotapi.User{ID: 1, FirstName: "Admin"}
	message := &tgbotapi.Message{
		Chat:     &tgbotapi.Chat{ID: 123},
		From:     adminUser,
		Text:     "/toggle_active Unknown",
		Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 14}},
	}

	// Mock expectations
	adminStoreUser := &store.User{ID: 1, TelegramUserID: 1, IsAdmin: true}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(1)).Return(adminStoreUser, nil)
	mockStore.On("GetUserByName", mock.Anything, "Unknown").Return(nil, errors.New("not found"))

	// Execute
	msg, err := h.HandleToggleActive(message)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "Could not find user: Unknown", msg.Text)
	mockStore.AssertExpectations(t)
}

func TestHandleUsers_StoreFailure(t *testing.T) {
	// Setup
	mockStore := new(mocks.MockStore)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.New(mockStore, mockScheduler)

	adminUser := &tgbotapi.User{ID: 1, FirstName: "Admin"}
	message := &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: 123}, From: adminUser}

	// Mock expectations
	adminStoreUser := &store.User{ID: 1, TelegramUserID: 1, IsAdmin: true}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(1)).Return(adminStoreUser, nil)
	mockStore.On("ListAllUsers", mock.Anything).Return(nil, errors.New("db error"))

	// Execute
	msg, err := h.HandleUsers(message)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "Failed to retrieve user list.", msg.Text)
	mockStore.AssertExpectations(t)
}

func TestHandleAssign_SchedulerFailure(t *testing.T) {
	// Setup
	mockStore := new(mocks.MockStore)
	mockScheduler := new(mocks.MockScheduler)
	h := handlers.New(mockStore, mockScheduler)

	adminUser := &tgbotapi.User{ID: 1, FirstName: "Admin"}
	message := &tgbotapi.Message{
		Chat:     &tgbotapi.Chat{ID: 123},
		From:     adminUser,
		Text:     "/assign TestUser 2023-12-25",
		Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 7}},
	}

	// Mock expectations
	adminStoreUser := &store.User{ID: 1, TelegramUserID: 1, IsAdmin: true}
	targetUser := &store.User{ID: 2, FirstName: "TestUser"}
	mockStore.On("GetUserByTelegramID", mock.Anything, int64(1)).Return(adminStoreUser, nil)
	mockStore.On("GetUserByName", mock.Anything, "TestUser").Return(targetUser, nil)
	mockScheduler.On("AssignDuty", mock.Anything, targetUser, "2023-12-25").Return(errors.New("scheduler failed"))

	// Execute
	msg, err := h.HandleAssign(message)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "Failed to assign TestUser to duty on 2023-12-25.", msg.Text)
	mockScheduler.AssertExpectations(t)
}