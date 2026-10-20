package assistant

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/korjavin/dutyassistant/internal/store/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	testChat  = int64(100)
	testAdmin = int64(100)
)

// fakeModel serves scripted Messages API responses and records request bodies.
type fakeModel struct {
	mu        sync.Mutex
	responses []string
	requests  []string
}

func (f *fakeModel) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.requests = append(f.requests, string(body))
		if len(f.responses) == 0 {
			t.Errorf("unexpected model call #%d", len(f.requests))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		resp := f.responses[0]
		f.responses = f.responses[1:]
		w.Header().Set("Content-Type", "application/json")
		if resp == serverError {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"type":"error","error":{"type":"api_error","message":"boom"}}`))
			return
		}
		_, _ = w.Write([]byte(resp))
	}
}

func (f *fakeModel) lastRequest() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.requests[len(f.requests)-1]
}

// serverError scripts a 500 response from the fake Messages API.
const serverError = "500"

func modelReply(stopReason string, blocks ...string) string {
	return `{"id":"msg","type":"message","role":"assistant","model":"claude-opus-5-5","content":[` +
		strings.Join(blocks, ",") + `],"stop_reason":"` + stopReason + `","usage":{"input_tokens":1,"output_tokens":1}}`
}

func textBlock(s string) string {
	b, _ := json.Marshal(s)
	return `{"type":"text","text":` + string(b) + `}`
}

func toolUse(id, name, input string) string {
	return `{"type":"tool_use","id":"` + id + `","name":"` + name + `","input":` + input + `}`
}

type fakeExecutor struct {
	commands  []string
	callbacks []string
	reply     map[string]Result
}

func (e *fakeExecutor) RunCommand(_ context.Context, _, _ int64, line string) (Result, error) {
	e.commands = append(e.commands, line)
	return e.reply[line], nil
}

func (e *fakeExecutor) RunCallback(_ context.Context, _, _ int64, data string) (Result, error) {
	e.callbacks = append(e.callbacks, data)
	return Result{Text: "callback done: " + data}, nil
}

type fakeSender struct {
	messages []tgbotapi.MessageConfig
}

func (s *fakeSender) Send(c tgbotapi.Chattable) error {
	if m, ok := c.(tgbotapi.MessageConfig); ok {
		s.messages = append(s.messages, m)
	}
	return nil
}

func (s *fakeSender) texts() string {
	var parts []string
	for _, m := range s.messages {
		parts = append(parts, m.Text)
	}
	return strings.Join(parts, "\n---\n")
}

// confirmationID returns the pending ID from the last Confirm button sent.
func (s *fakeSender) confirmationID(t *testing.T) string {
	t.Helper()
	for i := len(s.messages) - 1; i >= 0; i-- {
		markup, ok := s.messages[i].ReplyMarkup.(tgbotapi.InlineKeyboardMarkup)
		if !ok {
			continue
		}
		id, approved, ok := ParseCallback(*markup.InlineKeyboard[0][0].CallbackData)
		require.True(t, ok)
		require.True(t, approved)
		return id
	}
	t.Fatal("no confirmation message sent")
	return ""
}

type testEnv struct {
	agent    *Agent
	model    *fakeModel
	store    *mocks.MockStore
	executor *fakeExecutor
	sender   *fakeSender
	now      time.Time
}

func newTestEnv(t *testing.T, responses ...string) *testEnv {
	t.Helper()
	fm := &fakeModel{responses: responses}
	server := httptest.NewServer(fm.handler(t))
	t.Cleanup(server.Close)

	client := anthropic.NewClient(option.WithAPIKey("k"), option.WithBaseURL(server.URL), option.WithMaxRetries(0))
	env := &testEnv{
		model:    fm,
		store:    new(mocks.MockStore),
		executor: &fakeExecutor{reply: map[string]Result{}},
		sender:   &fakeSender{},
		now:      time.Date(2026, 10, 20, 18, 0, 0, 0, time.UTC),
	}
	env.agent = New(Config{
		Client:   &client,
		Model:    "claude-opus-5-5",
		Store:    env.store,
		Executor: env.executor,
		Sender:   env.sender,
		Now:      func() time.Time { return env.now },
	})
	return env
}

func (e *testEnv) withUsers(users ...*store.User) {
	e.store.On("ListAllUsers", mock.Anything).Return(users, nil)
}

func TestNew_NilClientDisables(t *testing.T) {
	assert.Nil(t, New(Config{}))
}

func TestAgent_ReadToolThenAnswer(t *testing.T) {
	env := newTestEnv(t,
		modelReply("tool_use", toolUse("tu_1", "list_users", `{}`)),
		modelReply("end_turn", textBlock("У <b>Ivan</b> 2 дня в очереди & всё ок")),
	)
	env.executor.reply["/users"] = Result{Text: "Ivan: admin queue 2"}

	env.agent.HandleMessage(context.Background(), testChat, testAdmin, "сколько дней у Вани?")

	assert.Equal(t, []string{"/users"}, env.executor.commands)
	assert.Contains(t, env.model.lastRequest(), "Ivan: admin queue 2")
	assert.Contains(t, env.model.lastRequest(), `"tool_use_id":"tu_1"`)
	assert.Contains(t, env.sender.texts(), "У <b>Ivan</b> 2 дня в очереди &amp; всё ок")
	// First request carries the timestamped admin text, the tools and the system prompt.
	first := env.model.requests[0]
	assert.Contains(t, first, "[2026-10-20 18:00 Tuesday UTC] сколько дней у Вани?")
	assert.Contains(t, first, `"name":"assign_chore"`)
	assert.Contains(t, first, "Confirm/Cancel")
	assert.Contains(t, first, `"effort":"medium"`)
}

func TestAgent_MutatingToolNeedsConfirmation(t *testing.T) {
	env := newTestEnv(t,
		modelReply("tool_use",
			textBlock("Назначаю Ване посуду."),
			toolUse("tu_1", "list_active_chores", `{}`),
			toolUse("tu_2", "assign_chore", `{"description":"Wash   the dishes","assignee_name":"ivan"}`)),
		modelReply("end_turn", textBlock("Готово.")),
	)
	env.withUsers(&store.User{ID: 1, FirstName: "Ivan"})
	env.store.On("GetActiveChores", mock.Anything).Return([]*store.Chore{}, nil)
	env.executor.reply["/chore Wash the dishes /Ivan"] = Result{Text: "🎯 Chore assigned to Ivan", ParseMode: "HTML"}

	env.agent.HandleMessage(context.Background(), testChat, testAdmin, "пусть Ваня помоет посуду")

	// Nothing changed yet; the admin sees the plan with buttons.
	assert.Empty(t, env.executor.commands)
	assert.Len(t, env.model.requests, 1)
	confirm := env.sender.texts()
	assert.Contains(t, confirm, "Назначаю Ване посуду.")
	assert.Contains(t, confirm, "Please confirm")
	assert.Contains(t, confirm, "Assign one-off chore <i>Wash the dishes</i> to <b>Ivan</b>")
	assert.Contains(t, confirm, "<code>/chore Wash the dishes /Ivan</code>")

	env.agent.HandleDecision(context.Background(), testChat, testAdmin, env.sender.confirmationID(t), true)

	assert.Equal(t, []string{"/chore Wash the dishes /Ivan"}, env.executor.commands)
	last := env.model.lastRequest()
	// Both tool results of the turn go back together.
	assert.Contains(t, last, `"tool_use_id":"tu_1"`)
	assert.Contains(t, last, "No active chores.")
	assert.Contains(t, last, `"tool_use_id":"tu_2"`)
	assert.Contains(t, last, "Executed. The bot replied to the admin")
	texts := env.sender.texts()
	assert.Contains(t, texts, "🎯 Chore assigned to Ivan")
	assert.Contains(t, texts, "Готово.")
}

func TestAgent_DeclineDoesNotExecute(t *testing.T) {
	env := newTestEnv(t,
		modelReply("tool_use", toolUse("tu_1", "set_vacation_mode", `{"enabled":true}`)),
		modelReply("end_turn", textBlock("ok")),
	)

	env.agent.HandleMessage(context.Background(), testChat, testAdmin, "включи отпуск")
	env.agent.HandleDecision(context.Background(), testChat, testAdmin, env.sender.confirmationID(t), false)

	assert.Empty(t, env.executor.commands)
	assert.Contains(t, env.sender.texts(), "Cancelled, nothing was changed")
	assert.Len(t, env.model.requests, 1)

	// The decline is part of the history for the next message.
	env.agent.HandleMessage(context.Background(), testChat, testAdmin, "а что сейчас?")
	assert.Contains(t, env.model.lastRequest(), "The admin declined this action")
}

func TestAgent_NewMessageDropsPendingActions(t *testing.T) {
	env := newTestEnv(t,
		modelReply("tool_use", toolUse("tu_1", "set_vacation_mode", `{"enabled":true}`)),
		modelReply("end_turn", textBlock("ok")),
	)

	env.agent.HandleMessage(context.Background(), testChat, testAdmin, "включи отпуск")
	id := env.sender.confirmationID(t)
	env.agent.HandleMessage(context.Background(), testChat, testAdmin, "нет, не надо")

	assert.Contains(t, env.model.lastRequest(), "Not executed: the admin sent a new message")
	// The old buttons no longer work.
	env.agent.HandleDecision(context.Background(), testChat, testAdmin, id, true)
	assert.Empty(t, env.executor.commands)
	assert.Contains(t, env.sender.texts(), "no longer valid")
}

func TestAgent_PrepareErrorGoesBackToModel(t *testing.T) {
	env := newTestEnv(t,
		modelReply("tool_use", toolUse("tu_1", "set_sviniya_balance", `{"user_name":"Petya","balance":3}`)),
		modelReply("end_turn", textBlock("Нет такого пользователя.")),
	)
	env.withUsers(&store.User{ID: 1, FirstName: "Ivan"})

	env.agent.HandleMessage(context.Background(), testChat, testAdmin, "дай Пете 3 свиньи")

	last := env.model.lastRequest()
	assert.Contains(t, last, `no user named \"Petya\"; known users: Ivan`)
	assert.Contains(t, last, `"is_error":true`)
	assert.NotContains(t, env.sender.texts(), "Please confirm")
	assert.Contains(t, env.sender.texts(), "Нет такого пользователя.")
}

func TestAgent_ExpiredConfirmation(t *testing.T) {
	env := newTestEnv(t,
		modelReply("tool_use", toolUse("tu_1", "set_vacation_mode", `{"enabled":false}`)),
	)
	env.agent.HandleMessage(context.Background(), testChat, testAdmin, "выключи отпуск")
	id := env.sender.confirmationID(t)

	env.now = env.now.Add(confirmationTTL + time.Minute)
	env.agent.HandleDecision(context.Background(), testChat, testAdmin, id, true)

	assert.Empty(t, env.executor.commands)
	assert.Contains(t, env.sender.texts(), "no longer valid")
}

func TestAgent_ModelErrorResetsConversation(t *testing.T) {
	env := newTestEnv(t, serverError)
	env.agent.HandleMessage(context.Background(), testChat, testAdmin, "привет")

	assert.Contains(t, env.sender.texts(), "assistant is unavailable")
	assert.Empty(t, env.agent.conversation(testChat).messages)
}

func TestAgent_StepLimit(t *testing.T) {
	var responses []string
	for i := 0; i < maxSteps; i++ {
		responses = append(responses, modelReply("tool_use", toolUse("tu", "get_sviniya_balances", `{}`)))
	}
	env := newTestEnv(t, responses...)

	env.agent.HandleMessage(context.Background(), testChat, testAdmin, "loop")

	assert.Len(t, env.executor.commands, maxSteps)
	assert.Contains(t, env.sender.texts(), "too many steps")
	assert.Empty(t, env.agent.conversation(testChat).messages)
}

func TestParseCallback(t *testing.T) {
	id, approved, ok := ParseCallback("aa:y:abc123")
	assert.True(t, ok)
	assert.True(t, approved)
	assert.Equal(t, "abc123", id)

	_, approved, ok = ParseCallback("aa:n:abc123")
	assert.True(t, ok)
	assert.False(t, approved)

	for _, bad := range []string{"aa:x:1", "aa:y:", "aa:y", "complete_chore:1", "aa:y:1:2"} {
		_, _, ok := ParseCallback(bad)
		assert.False(t, ok, bad)
	}
	assert.True(t, IsCallback("aa:y:1"))
	assert.False(t, IsCallback("assign_user:1"))
}

func TestTruncate(t *testing.T) {
	short := "привет"
	assert.Equal(t, short, truncate(short))

	long := strings.Repeat("я", maxMessageLen) // 2 bytes per rune
	got := truncate(long)
	assert.LessOrEqual(t, len(got), maxMessageLen+len("…"))
	assert.True(t, strings.HasSuffix(got, "…"))
	assert.True(t, strings.HasPrefix(got, "яяя"))
}
