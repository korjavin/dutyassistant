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
	"sync/atomic"
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

const (
	mediaParticipant = int64(501)
	mediaGroup       = int64(-1000)
	mediaBotID       = int64(999)
)

// recordingTelegram answers every Bot API call and records sendMessage calls per chat.
type recordingTelegram struct {
	mu    sync.Mutex
	forms map[string][]url.Values
}

func (r *recordingTelegram) RoundTrip(req *http.Request) (*http.Response, error) {
	body := `{"ok":true,"result":{"message_id":1,"chat":{"id":1}}}`
	if strings.HasSuffix(req.URL.Path, "/getMe") {
		body = `{"ok":true,"result":{"id":999,"is_bot":true,"first_name":"Bot","username":"dish_bot"}}`
	} else if strings.HasSuffix(req.URL.Path, "/getChatMember") {
		body = `{"ok":true,"result":{"status":"member","user":{"id":501}}}`
	} else if strings.HasSuffix(req.URL.Path, "/sendMessage") {
		data, _ := io.ReadAll(req.Body)
		form, _ := url.ParseQuery(string(data))
		r.mu.Lock()
		r.forms[form.Get("chat_id")] = append(r.forms[form.Get("chat_id")], form)
		r.mu.Unlock()
	}
	return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewBufferString(body)), Header: make(http.Header)}, nil
}

func (r *recordingTelegram) sentTo(chatID int64) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var texts []string
	for _, f := range r.forms[strconv.FormatInt(chatID, 10)] {
		texts = append(texts, f.Get("text"))
	}
	return texts
}

type stubTranscriber struct {
	text  string
	calls *atomic.Int32
}

func (s stubTranscriber) Transcribe(ctx context.Context, audio io.Reader, filename string) (string, error) {
	s.calls.Add(1)
	return s.text, nil
}

type stubImageReader struct {
	text       string
	gotCaption string
	mu         sync.Mutex
}

func (s *stubImageReader) DescribeImage(ctx context.Context, mediaType string, data []byte, caption string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gotCaption = caption
	return s.text + " (" + mediaType + ")", nil
}

type mediaTestEnv struct {
	bot         *Bot
	store       *mocks.MockStore
	tg          *recordingTelegram
	transcribed *atomic.Int32
	images      *stubImageReader
	mu          sync.Mutex
	modelBodies []string
}

// newMediaTestBot builds a bot with a fake Telegram, stub speech-to-text and
// image reader (when enabled) and assistants backed by a fake Messages API
// that answers modelReply.
func newMediaTestBot(t *testing.T, transcript, imageText, modelReply string) *mediaTestEnv {
	t.Helper()
	env := &mediaTestEnv{tg: &recordingTelegram{forms: map[string][]url.Values{}}, transcribed: &atomic.Int32{}}
	api, err := tgbotapi.NewBotAPIWithClient("TOKEN", tgbotapi.APIEndpoint, &http.Client{Transport: env.tg})
	require.NoError(t, err)

	env.store = new(mocks.MockStore)
	h := handlers.NewWithAdminID(env.store, new(mocks.MockScheduler), mediaGroup, assistantTestAdmin, nil)
	h.Bot = api
	h.FetchFile = func(ctx context.Context, fileID string) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("\xff\xd8\xff\xe0JPEG")), nil
	}
	if transcript != "" {
		h.Transcriber = stubTranscriber{text: transcript, calls: env.transcribed}
	}
	if imageText != "" {
		env.images = &stubImageReader{text: imageText}
		h.ImageReader = env.images
	}
	env.bot = &Bot{api: api, handlers: h, ownerID: assistantTestAdmin, groupID: mediaGroup, self: api.Self}

	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		env.mu.Lock()
		env.modelBodies = append(env.modelBodies, string(body))
		env.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg","type":"message","role":"assistant","model":"claude-opus-5-5","content":[{"type":"text","text":` +
			strconv.Quote(modelReply) + `}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`))
	}))
	t.Cleanup(model.Close)
	client := anthropic.NewClient(option.WithAPIKey("k"), option.WithBaseURL(model.URL), option.WithMaxRetries(0))
	env.bot.EnableAssistants(AssistantConfig{Client: &client, Model: "claude-opus-5-5", Store: env.store, Location: time.UTC, ParticipantDailyLimit: 5})
	return env
}

func (e *mediaTestEnv) lastModelRequest() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.modelBodies) == 0 {
		return ""
	}
	return e.modelBodies[len(e.modelBodies)-1]
}

func (e *mediaTestEnv) withParticipant() {
	e.store.On("GetUserByTelegramID", mock.Anything, mediaParticipant).
		Return(&store.User{ID: 5, TelegramUserID: mediaParticipant, FirstName: "Petya", IsActive: true}, nil)
}

func (e *mediaTestEnv) withAdmin() {
	e.store.On("GetUserByTelegramID", mock.Anything, assistantTestAdmin).
		Return(&store.User{ID: 1, FirstName: "Admin", IsAdmin: true, IsActive: true}, nil)
}

func privateUpdate(from int64) tgbotapi.Update {
	return tgbotapi.Update{Message: &tgbotapi.Message{
		MessageID: 3,
		From:      &tgbotapi.User{ID: from, FirstName: "Petya"},
		Chat:      &tgbotapi.Chat{ID: from, Type: "private"},
	}}
}

func groupUpdate(from int64, replyToBot bool) tgbotapi.Update {
	u := tgbotapi.Update{Message: &tgbotapi.Message{
		MessageID: 30,
		From:      &tgbotapi.User{ID: from, FirstName: "Petya"},
		Chat:      &tgbotapi.Chat{ID: mediaGroup, Type: "supergroup"},
	}}
	if replyToBot {
		u.Message.ReplyToMessage = &tgbotapi.Message{MessageID: 29, From: &tgbotapi.User{ID: mediaBotID, IsBot: true}}
	}
	return u
}

func withVoice(u tgbotapi.Update) tgbotapi.Update {
	u.Message.Voice = &tgbotapi.Voice{FileID: "voice-1", FileSize: 1000}
	return u
}

func withPhoto(u tgbotapi.Update, caption string) tgbotapi.Update {
	u.Message.Photo = []tgbotapi.PhotoSize{{FileID: "small", FileSize: 100}, {FileID: "big", FileSize: 90000}}
	u.Message.Caption = caption
	return u
}

func withText(u tgbotapi.Update, text string) tgbotapi.Update {
	u.Message.Text = text
	return u
}

func eventually(t *testing.T, cond func() bool) {
	t.Helper()
	require.Eventually(t, cond, 2*time.Second, 10*time.Millisecond)
}

func TestMedia_AdminVoiceGoesToAssistant(t *testing.T) {
	env := newMediaTestBot(t, "Сколько свиней у Маши?", "", "У Маши 2 свиньи.")
	env.withAdmin()

	env.bot.handleUpdate(withVoice(privateUpdate(assistantTestAdmin)))

	eventually(t, func() bool { return len(env.tg.sentTo(assistantTestAdmin)) >= 2 })
	sent := env.tg.sentTo(assistantTestAdmin)
	assert.Equal(t, "🎧 I heard: «Сколько свиней у Маши?»", sent[0])
	assert.Contains(t, sent[1], "У Маши 2 свиньи.")
	assert.Contains(t, env.lastModelRequest(), "Сколько свиней у Маши?")
}

func TestMedia_PhotoWithCaptionGoesToAssistant(t *testing.T) {
	env := newMediaTestBot(t, "", "A whiteboard saying: Petya - trash", "Назначить Пете мусор?")
	env.withAdmin()

	env.bot.handleUpdate(withPhoto(privateUpdate(assistantTestAdmin), "сделай как тут написано"))

	eventually(t, func() bool { return len(env.tg.sentTo(assistantTestAdmin)) >= 2 })
	sent := env.tg.sentTo(assistantTestAdmin)
	assert.Equal(t, "🖼 I see: A whiteboard saying: Petya - trash (image/jpeg)", sent[0])
	assert.Equal(t, "сделай как тут написано", env.images.gotCaption)
	req := env.lastModelRequest()
	assert.Contains(t, req, "сделай как тут написано")
	assert.Contains(t, req, "[Photo: A whiteboard saying: Petya - trash (image/jpeg)]")
}

func TestMedia_ParticipantVoiceIsReportPart(t *testing.T) {
	env := newMediaTestBot(t, "Помыл посуду", "", "")
	env.withParticipant()
	report := &store.DailyReport{ID: 9, UserID: 5, Status: store.DailyReportCollecting}
	env.store.On("GetDailyReport", mock.Anything, int64(5), mock.Anything).Return(report, nil)
	env.store.On("AppendDailyReport", mock.Anything, int64(9), "Помыл посуду").Return(true, nil)

	env.bot.handleUpdate(withVoice(privateUpdate(mediaParticipant)))

	eventually(t, func() bool { return len(env.tg.sentTo(mediaParticipant)) >= 2 })
	sent := env.tg.sentTo(mediaParticipant)
	assert.Equal(t, "🎧 I heard: «Помыл посуду»", sent[0])
	assert.Contains(t, sent[1], "Got it (part 1)")
}

func TestMedia_GroupReplyToBotGoesToAssistant(t *testing.T) {
	env := newMediaTestBot(t, "А когда я дежурю?", "", "В среду.")
	env.withParticipant()

	// A typed reply to the bot.
	env.bot.handleUpdate(withText(groupUpdate(mediaParticipant, true), "а Маша когда?"))
	eventually(t, func() bool { return len(env.tg.sentTo(mediaGroup)) >= 1 })
	assert.Contains(t, env.lastModelRequest(), "Group chat message from Petya")
	assert.Contains(t, env.lastModelRequest(), "а Маша когда?")

	// A voice reply to the bot: heard, then answered, both as replies.
	env.bot.handleUpdate(withVoice(groupUpdate(mediaParticipant, true)))
	eventually(t, func() bool { return len(env.tg.sentTo(mediaGroup)) >= 3 })
	assert.Equal(t, "🎧 I heard: «А когда я дежурю?»", env.tg.sentTo(mediaGroup)[1])
	assert.Contains(t, env.lastModelRequest(), "А когда я дежурю?")
	env.tg.mu.Lock()
	for _, f := range env.tg.forms[strconv.FormatInt(mediaGroup, 10)] {
		assert.Equal(t, "30", f.Get("reply_to_message_id"))
	}
	env.tg.mu.Unlock()
}

func TestMedia_GroupChatterIsIgnored(t *testing.T) {
	env := newMediaTestBot(t, "Купи хлеба", "A cat", "")
	env.withParticipant()

	// Not addressed to the bot: plain voice, a photo, a reply to someone else.
	env.bot.handleUpdate(withVoice(groupUpdate(mediaParticipant, false)))
	env.bot.handleUpdate(withPhoto(groupUpdate(mediaParticipant, false), "смотри"))
	other := withVoice(groupUpdate(mediaParticipant, false))
	other.Message.ReplyToMessage = &tgbotapi.Message{MessageID: 28, From: &tgbotapi.User{ID: 777}}
	env.bot.handleUpdate(other)

	time.Sleep(50 * time.Millisecond)
	assert.Empty(t, env.tg.sentTo(mediaGroup))
	assert.Zero(t, env.transcribed.Load(), "group chatter must not be transcribed")
	assert.Empty(t, env.lastModelRequest())
}

func TestMedia_CaptionMentionAddressesBot(t *testing.T) {
	env := newMediaTestBot(t, "", "A clean kitchen", "Молодец!")
	env.withParticipant()

	u := withPhoto(groupUpdate(mediaParticipant, false), "@dish_bot я убрал кухню")
	u.Message.CaptionEntities = []tgbotapi.MessageEntity{{Type: "mention", Offset: 0, Length: 9}}
	env.bot.handleUpdate(u)

	eventually(t, func() bool { return len(env.tg.sentTo(mediaGroup)) >= 2 })
	req := env.lastModelRequest()
	assert.Contains(t, req, "я убрал кухню")
	assert.NotContains(t, req, "@dish_bot")
	assert.Contains(t, req, "[Photo: A clean kitchen (image/jpeg)]")
}

func TestMedia_WithoutReaders(t *testing.T) {
	env := newMediaTestBot(t, "", "", "Понял.")
	env.withAdmin()
	env.withParticipant()

	env.bot.handleUpdate(withVoice(privateUpdate(assistantTestAdmin)))
	env.bot.handleUpdate(withVoice(privateUpdate(mediaParticipant)))
	env.bot.handleUpdate(withPhoto(privateUpdate(mediaParticipant), ""))
	assert.Equal(t, []string{adminVoiceUnavailableMessage}, env.tg.sentTo(assistantTestAdmin))
	assert.Equal(t, []string{voiceUnavailableMessage, imageUnavailableMessage}, env.tg.sentTo(mediaParticipant))

	// A photo's caption still gets through.
	env.store.On("GetDailyReport", mock.Anything, int64(5), mock.Anything).Return(nil, nil)
	env.bot.handleUpdate(withPhoto(privateUpdate(mediaParticipant), "что у меня сегодня?"))
	eventually(t, func() bool { return strings.Contains(env.lastModelRequest(), "что у меня сегодня?") })
}
