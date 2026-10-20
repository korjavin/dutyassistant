package handlers_test

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/mocks"
	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/korjavin/dutyassistant/internal/telegram/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	regAdminID    = int64(1000)
	regNewcomerID = int64(2000)
)

// sentMessages captures sendMessage calls made through a fake Telegram API.
type sentMessages struct {
	mu    sync.Mutex
	forms []url.Values
}

func (s *sentMessages) to(chatID string) []url.Values {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []url.Values
	for _, f := range s.forms {
		if f.Get("chat_id") == chatID {
			out = append(out, f)
		}
	}
	return out
}

func newRegistrationHandlers(t *testing.T) (*handlers.Handlers, *mocks.MockStore, *sentMessages) {
	t.Helper()
	sent := &sentMessages{}
	client := NewTestClient(func(req *http.Request) *http.Response {
		if strings.Contains(req.URL.String(), "getMe") {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewBufferString(`{"ok":true,"result":{"id":1}}`)), Header: make(http.Header)}
		}
		if strings.Contains(req.URL.String(), "sendMessage") {
			body, _ := io.ReadAll(req.Body)
			form, _ := url.ParseQuery(string(body))
			sent.mu.Lock()
			sent.forms = append(sent.forms, form)
			sent.mu.Unlock()
			return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewBufferString(`{"ok":true,"result":{"message_id":1}}`)), Header: make(http.Header)}
		}
		return &http.Response{StatusCode: 404, Body: io.NopCloser(bytes.NewBufferString(`{}`)), Header: make(http.Header)}
	})
	bot, err := tgbotapi.NewBotAPIWithClient("TOKEN", tgbotapi.APIEndpoint, client)
	require.NoError(t, err)

	st := new(mocks.MockStore)
	h := handlers.NewWithAdminID(st, nil, 0, regAdminID, nil)
	h.Bot = bot
	return h, st, sent
}

func startFrom(id int64) *tgbotapi.Message {
	return &tgbotapi.Message{
		Text:     "/start",
		Chat:     &tgbotapi.Chat{ID: id, Type: "private"},
		From:     &tgbotapi.User{ID: id, FirstName: "Petya", LastName: "Ivanov", UserName: "petya_i"},
		Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 6}},
	}
}

func regCallback(from int64, data string) *tgbotapi.CallbackQuery {
	return &tgbotapi.CallbackQuery{
		From:    &tgbotapi.User{ID: from},
		Data:    data,
		Message: &tgbotapi.Message{MessageID: 42, Chat: &tgbotapi.Chat{ID: regAdminID}},
	}
}

func TestStart_UnknownUserAsksAdmin(t *testing.T) {
	h, st, sent := newRegistrationHandlers(t)
	st.On("GetUserByTelegramID", mock.Anything, regNewcomerID).Return(nil, nil)
	st.On("GetRegistrationRequest", mock.Anything, regNewcomerID).Return(nil, nil)
	st.On("UpsertRegistrationRequest", mock.Anything, mock.MatchedBy(func(r *store.RegistrationRequest) bool {
		return r.TelegramUserID == regNewcomerID && r.FirstName == "Petya" && r.LastName == "Ivanov" &&
			r.Username == "petya_i" && r.Status == store.RegistrationPending
	})).Return(nil)

	msg, err := h.HandleStart(startFrom(regNewcomerID))

	require.NoError(t, err)
	assert.Contains(t, msg.Text, "sent to the admin")
	st.AssertNotCalled(t, "CreateUser", mock.Anything, mock.Anything)

	toAdmin := sent.to("1000")
	require.Len(t, toAdmin, 1)
	assert.Contains(t, toAdmin[0].Get("text"), "New user wants to join")
	assert.Contains(t, toAdmin[0].Get("text"), "Petya Ivanov")
	assert.Contains(t, toAdmin[0].Get("text"), "@petya_i")
	assert.Contains(t, toAdmin[0].Get("reply_markup"), "reg_ok:2000")
	assert.Contains(t, toAdmin[0].Get("reply_markup"), "reg_no:2000")
}

func TestStart_PendingAndRejectedDoNotSpamAdmin(t *testing.T) {
	h, st, sent := newRegistrationHandlers(t)
	st.On("GetUserByTelegramID", mock.Anything, regNewcomerID).Return(nil, nil)

	st.On("GetRegistrationRequest", mock.Anything, regNewcomerID).Return(&store.RegistrationRequest{
		TelegramUserID: regNewcomerID, Status: store.RegistrationPending, RequestedAt: time.Now(),
	}, nil).Once()
	msg, err := h.HandleStart(startFrom(regNewcomerID))
	require.NoError(t, err)
	assert.Contains(t, msg.Text, "still waiting")

	st.On("GetRegistrationRequest", mock.Anything, regNewcomerID).Return(&store.RegistrationRequest{
		TelegramUserID: regNewcomerID, Status: store.RegistrationRejected, RequestedAt: time.Now(),
	}, nil).Once()
	msg, err = h.HandleStart(startFrom(regNewcomerID))
	require.NoError(t, err)
	assert.Contains(t, msg.Text, "declined")

	assert.Empty(t, sent.to("1000"))
	st.AssertNotCalled(t, "UpsertRegistrationRequest", mock.Anything, mock.Anything)
}

func TestStart_OldPendingRequestRemindsAdmin(t *testing.T) {
	h, st, sent := newRegistrationHandlers(t)
	st.On("GetUserByTelegramID", mock.Anything, regNewcomerID).Return(nil, nil)
	st.On("GetRegistrationRequest", mock.Anything, regNewcomerID).Return(&store.RegistrationRequest{
		TelegramUserID: regNewcomerID, Status: store.RegistrationPending, RequestedAt: time.Now().Add(-2 * time.Hour),
	}, nil)
	st.On("UpsertRegistrationRequest", mock.Anything, mock.Anything).Return(nil)

	_, err := h.HandleStart(startFrom(regNewcomerID))

	require.NoError(t, err)
	assert.Len(t, sent.to("1000"), 1)
}

func TestStart_AdminAndExistingUsersSkipApproval(t *testing.T) {
	h, st, _ := newRegistrationHandlers(t)
	st.On("GetUserByTelegramID", mock.Anything, regAdminID).Return(nil, nil)
	st.On("CreateUser", mock.Anything, mock.MatchedBy(func(u *store.User) bool { return u.IsAdmin })).Return(nil)
	msg, err := h.HandleStart(startFrom(regAdminID))
	require.NoError(t, err)
	assert.Contains(t, msg.Text, "Welcome")

	st.On("GetUserByTelegramID", mock.Anything, regNewcomerID).Return(&store.User{ID: 5, TelegramUserID: regNewcomerID, FirstName: "Petya", IsActive: true}, nil)
	msg, err = h.HandleStart(startFrom(regNewcomerID))
	require.NoError(t, err)
	assert.Contains(t, msg.Text, "Welcome")
	st.AssertNotCalled(t, "GetRegistrationRequest", mock.Anything, mock.Anything)
}

func TestStart_NoAdminConfiguredKeepsOldBehavior(t *testing.T) {
	st := new(mocks.MockStore)
	h := handlers.New(st, nil, 0, nil)
	st.On("GetUserByTelegramID", mock.Anything, regNewcomerID).Return(nil, nil)
	st.On("CreateUser", mock.Anything, mock.Anything).Return(nil)

	msg, err := h.HandleStart(startFrom(regNewcomerID))

	require.NoError(t, err)
	assert.Contains(t, msg.Text, "Welcome")
	st.AssertCalled(t, "CreateUser", mock.Anything, mock.Anything)
}

func TestRegistrationCallback_Approve(t *testing.T) {
	h, st, sent := newRegistrationHandlers(t)
	st.On("GetRegistrationRequest", mock.Anything, regNewcomerID).Return(&store.RegistrationRequest{
		TelegramUserID: regNewcomerID, FirstName: "Petya", Status: store.RegistrationPending,
	}, nil)
	st.On("GetUserByTelegramID", mock.Anything, regNewcomerID).Return(nil, nil)
	st.On("CreateUser", mock.Anything, mock.MatchedBy(func(u *store.User) bool {
		return u.TelegramUserID == regNewcomerID && u.FirstName == "Petya" && u.IsActive && !u.IsAdmin
	})).Return(nil)
	st.On("DeleteRegistrationRequest", mock.Anything, regNewcomerID).Return(nil)

	resp, err := h.HandleRegistrationCallback(regCallback(regAdminID, "reg_ok:2000"))

	require.NoError(t, err)
	edit := resp.(tgbotapi.EditMessageTextConfig)
	assert.Contains(t, edit.Text, "Approved")
	assert.Equal(t, 42, edit.MessageID)
	st.AssertExpectations(t)
	dm := sent.to("2000")
	require.Len(t, dm, 1)
	assert.Contains(t, dm[0].Get("text"), "approved you")
}

func TestRegistrationCallback_Reject(t *testing.T) {
	h, st, sent := newRegistrationHandlers(t)
	st.On("GetRegistrationRequest", mock.Anything, regNewcomerID).Return(&store.RegistrationRequest{
		TelegramUserID: regNewcomerID, FirstName: "Petya", Status: store.RegistrationPending,
	}, nil)
	st.On("UpsertRegistrationRequest", mock.Anything, mock.MatchedBy(func(r *store.RegistrationRequest) bool {
		return r.Status == store.RegistrationRejected
	})).Return(nil)

	resp, err := h.HandleRegistrationCallback(regCallback(regAdminID, "reg_no:2000"))

	require.NoError(t, err)
	assert.Contains(t, resp.(tgbotapi.EditMessageTextConfig).Text, "Rejected")
	st.AssertNotCalled(t, "CreateUser", mock.Anything, mock.Anything)
	dm := sent.to("2000")
	require.Len(t, dm, 1)
	assert.Contains(t, dm[0].Get("text"), "declined")
}

func TestRegistrationCallback_Guards(t *testing.T) {
	h, st, _ := newRegistrationHandlers(t)

	// Non-admins cannot approve, even themselves.
	st.On("GetUserByTelegramID", mock.Anything, regNewcomerID).Return(nil, nil)
	resp, err := h.HandleRegistrationCallback(regCallback(regNewcomerID, "reg_ok:2000"))
	require.NoError(t, err)
	assert.Nil(t, resp)
	st.AssertNotCalled(t, "CreateUser", mock.Anything, mock.Anything)

	// A second click after the request was handled does nothing.
	st.On("GetRegistrationRequest", mock.Anything, int64(3000)).Return(nil, nil)
	resp, err = h.HandleRegistrationCallback(regCallback(regAdminID, "reg_ok:3000"))
	require.NoError(t, err)
	assert.Contains(t, resp.(tgbotapi.EditMessageTextConfig).Text, "already handled")

	// Malformed data is ignored.
	resp, err = h.HandleRegistrationCallback(regCallback(regAdminID, "reg_ok:abc"))
	require.NoError(t, err)
	assert.Nil(t, resp)
}
