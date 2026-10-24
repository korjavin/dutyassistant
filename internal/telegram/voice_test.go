package telegram

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/mocks"
	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/korjavin/dutyassistant/internal/telegram/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const voiceParticipant = int64(501)

// recordingTelegram answers every Bot API call and records sendMessage texts per chat.
type recordingTelegram struct {
	mu    sync.Mutex
	texts map[string][]string
}

func (r *recordingTelegram) RoundTrip(req *http.Request) (*http.Response, error) {
	body := `{"ok":true,"result":{"message_id":1,"chat":{"id":1}}}`
	if strings.HasSuffix(req.URL.Path, "/getMe") {
		body = `{"ok":true,"result":{"id":999,"is_bot":true,"first_name":"Bot","username":"bot"}}`
	} else if strings.HasSuffix(req.URL.Path, "/sendMessage") {
		data, _ := io.ReadAll(req.Body)
		form, _ := url.ParseQuery(string(data))
		r.mu.Lock()
		r.texts[form.Get("chat_id")] = append(r.texts[form.Get("chat_id")], form.Get("text"))
		r.mu.Unlock()
	}
	return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewBufferString(body)), Header: make(http.Header)}, nil
}

func (r *recordingTelegram) sentTo(chatID int64) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.texts[strconv.FormatInt(chatID, 10)]...)
}

type stubTranscriber struct{ text string }

func (s stubTranscriber) Transcribe(ctx context.Context, audio io.Reader, filename string) (string, error) {
	return s.text, nil
}

func newVoiceTestBot(t *testing.T, transcript string) (*Bot, *mocks.MockStore, *recordingTelegram) {
	t.Helper()
	tg := &recordingTelegram{texts: map[string][]string{}}
	api, err := tgbotapi.NewBotAPIWithClient("TOKEN", tgbotapi.APIEndpoint, &http.Client{Transport: tg})
	require.NoError(t, err)

	mockStore := new(mocks.MockStore)
	h := handlers.NewWithAdminID(mockStore, new(mocks.MockScheduler), 0, assistantTestAdmin, nil)
	h.Bot = api
	if transcript != "" {
		h.Transcriber = stubTranscriber{text: transcript}
		h.FetchFile = func(ctx context.Context, fileID string) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("OGG")), nil
		}
	}
	return &Bot{api: api, handlers: h, ownerID: assistantTestAdmin}, mockStore, tg
}

func voiceUpdate(from int64) tgbotapi.Update {
	return tgbotapi.Update{Message: &tgbotapi.Message{
		MessageID: 3,
		From:      &tgbotapi.User{ID: from},
		Chat:      &tgbotapi.Chat{ID: from, Type: "private"},
		Voice:     &tgbotapi.Voice{FileID: "voice-1", FileSize: 1000},
	}}
}

func TestVoice_AdminRequestGoesToAssistant(t *testing.T) {
	bot, mockStore, tg := newVoiceTestBot(t, "Сколько свиней у Маши?")
	mockStore.On("GetUserByTelegramID", mock.Anything, assistantTestAdmin).Return(&store.User{ID: 1, FirstName: "Admin", IsAdmin: true, IsActive: true}, nil)

	var mu sync.Mutex
	var modelRequest string
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		modelRequest = string(body)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg","type":"message","role":"assistant","model":"claude-opus-5-5","content":[{"type":"text","text":"У Маши 2 свиньи."}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`))
	}))
	defer model.Close()
	client := anthropic.NewClient(option.WithAPIKey("k"), option.WithBaseURL(model.URL), option.WithMaxRetries(0))
	bot.EnableAssistants(AssistantConfig{Client: &client, Model: "claude-opus-5-5", Store: mockStore, Location: time.UTC})

	bot.handleUpdate(voiceUpdate(assistantTestAdmin))

	require.Eventually(t, func() bool { return len(tg.sentTo(assistantTestAdmin)) >= 2 }, 2*time.Second, 10*time.Millisecond)
	sent := tg.sentTo(assistantTestAdmin)
	assert.Equal(t, "🎧 I heard: «Сколько свиней у Маши?»", sent[0])
	assert.Contains(t, sent[1], "У Маши 2 свиньи.")
	mu.Lock()
	defer mu.Unlock()
	assert.Contains(t, modelRequest, "Сколько свиней у Маши?")
}

func TestVoice_ParticipantReportPart(t *testing.T) {
	bot, mockStore, tg := newVoiceTestBot(t, "Помыл посуду")
	user := &store.User{ID: 5, TelegramUserID: voiceParticipant, FirstName: "Petya", IsActive: true}
	report := &store.DailyReport{ID: 9, UserID: 5, Status: store.DailyReportCollecting}
	mockStore.On("GetUserByTelegramID", mock.Anything, voiceParticipant).Return(user, nil)
	mockStore.On("GetDailyReport", mock.Anything, int64(5), mock.Anything).Return(report, nil)
	mockStore.On("AppendDailyReport", mock.Anything, int64(9), "Помыл посуду").Return(true, nil)

	bot.handleUpdate(voiceUpdate(voiceParticipant))

	require.Eventually(t, func() bool { return len(tg.sentTo(voiceParticipant)) >= 2 }, 2*time.Second, 10*time.Millisecond)
	sent := tg.sentTo(voiceParticipant)
	assert.Equal(t, "🎧 I heard: «Помыл посуду»", sent[0])
	assert.Contains(t, sent[1], "Got it (part 1)")
	mockStore.AssertCalled(t, "AppendDailyReport", mock.Anything, int64(9), "Помыл посуду")
}

func TestVoice_WithoutTranscription(t *testing.T) {
	bot, mockStore, tg := newVoiceTestBot(t, "")
	mockStore.On("GetUserByTelegramID", mock.Anything, mock.Anything).Return(nil, nil)

	bot.handleUpdate(voiceUpdate(assistantTestAdmin))
	bot.handleUpdate(voiceUpdate(voiceParticipant))

	assert.Equal(t, []string{adminVoiceUnavailableMessage}, tg.sentTo(assistantTestAdmin))
	assert.Equal(t, []string{voiceUnavailableMessage}, tg.sentTo(voiceParticipant))
}
