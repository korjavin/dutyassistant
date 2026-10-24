package handlers

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/llm"
	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/korjavin/dutyassistant/internal/store/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	reportAdminTG  = int64(100)
	reportGroupID  = int64(-1000)
	reportMashaTG  = int64(501)
	reportPetyaTG  = int64(502)
	reportTestDate = "2026-10-24"
)

// sentRequest is one Telegram API call made through the fake bot.
type sentRequest struct {
	Method string
	Form   url.Values
}

// fakeTelegram records the bot's API calls and answers them successfully.
type fakeTelegram struct {
	mu   sync.Mutex
	sent []sentRequest
}

func (f *fakeTelegram) RoundTrip(req *http.Request) (*http.Response, error) {
	method := req.URL.Path[strings.LastIndex(req.URL.Path, "/")+1:]
	body := `{"ok":true,"result":{"message_id":1,"chat":{"id":1}}}`
	if method == "getMe" {
		body = `{"ok":true,"result":{"id":999,"is_bot":true,"first_name":"Bot","username":"bot"}}`
	} else {
		data, _ := io.ReadAll(req.Body)
		form, _ := url.ParseQuery(string(data))
		f.mu.Lock()
		f.sent = append(f.sent, sentRequest{Method: method, Form: form})
		f.mu.Unlock()
	}
	return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewBufferString(body)), Header: make(http.Header)}, nil
}

// messagesTo returns the texts sent to chatID.
func (f *fakeTelegram) messagesTo(chatID int64) []sentRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []sentRequest
	for _, r := range f.sent {
		if r.Method == "sendMessage" && r.Form.Get("chat_id") == strconvFormat(chatID) {
			out = append(out, r)
		}
	}
	return out
}

func strconvFormat(id int64) string {
	return strconv.FormatInt(id, 10)
}

type fakeTranscriber struct{ text string }

func (f fakeTranscriber) Transcribe(ctx context.Context, audio io.Reader, filename string) (string, error) {
	return f.text, nil
}

type fakeAnalyzer struct {
	mu        sync.Mutex
	score     int
	err       error
	gotReport string
	gotFacts  string
}

func (f *fakeAnalyzer) AssessDailyReport(ctx context.Context, name, report, facts string) (*llm.ReportAssessment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gotReport, f.gotFacts = report, facts
	if f.err != nil {
		return nil, f.err
	}
	return &llm.ReportAssessment{Score: f.score, Analysis: "Помогала по дому <молодец>."}, nil
}

type reportTestEnv struct {
	h        *Handlers
	store    *sqlite.SQLiteStore
	tg       *fakeTelegram
	analyzer *fakeAnalyzer
	users    map[int64]*store.User // by Telegram ID
}

func newReportTestEnv(t *testing.T, participants ...string) *reportTestEnv {
	t.Helper()
	ctx := context.Background()
	st, err := sqlite.New(ctx, ":memory:")
	require.NoError(t, err)

	tg := &fakeTelegram{}
	bot, err := tgbotapi.NewBotAPIWithClient("TOKEN", tgbotapi.APIEndpoint, &http.Client{Transport: tg})
	require.NoError(t, err)

	env := &reportTestEnv{store: st, tg: tg, analyzer: &fakeAnalyzer{score: 4}, users: map[int64]*store.User{}}
	admin := &store.User{TelegramUserID: reportAdminTG, FirstName: "Admin", IsActive: true}
	require.NoError(t, st.CreateUser(ctx, admin))
	for i, name := range participants {
		u := &store.User{TelegramUserID: reportMashaTG + int64(i), FirstName: name, IsActive: true}
		require.NoError(t, st.CreateUser(ctx, u))
		env.users[u.TelegramUserID] = u
	}

	h := NewWithAdminID(st, nil, reportGroupID, reportAdminTG, nil)
	h.Bot = bot
	h.ReportAnalyzer = env.analyzer
	h.Transcriber = fakeTranscriber{text: "Помыла посуду"}
	h.fetchFile = func(ctx context.Context, fileID string) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("OGG")), nil
	}
	env.h = h

	originalNow := TimeNow
	berlin, _ := time.LoadLocation(ratingTimezoneName)
	TimeNow = func() time.Time { return time.Date(2026, 10, 24, 20, 41, 0, 0, berlin) }
	t.Cleanup(func() { TimeNow = originalNow })
	return env
}

func (e *reportTestEnv) report(t *testing.T, tgID int64) *store.DailyReport {
	t.Helper()
	day, _ := time.Parse("2006-01-02", reportTestDate)
	r, err := e.store.GetDailyReport(context.Background(), e.users[tgID].ID, day)
	require.NoError(t, err)
	require.NotNil(t, r)
	return r
}

func privateMessage(tgID int64, text string) *tgbotapi.Message {
	return &tgbotapi.Message{
		MessageID: 7,
		From:      &tgbotapi.User{ID: tgID},
		Chat:      &tgbotapi.Chat{ID: tgID, Type: "private"},
		Text:      text,
	}
}

func callback(fromID int64, data string) *tgbotapi.CallbackQuery {
	return &tgbotapi.CallbackQuery{
		ID:      "cb",
		From:    &tgbotapi.User{ID: fromID},
		Message: &tgbotapi.Message{MessageID: 55, Chat: &tgbotapi.Chat{ID: fromID}},
		Data:    data,
	}
}

func TestDailyReportFlow(t *testing.T) {
	env := newReportTestEnv(t, "Masha", "Petya")
	h, ctx := env.h, context.Background()

	// 20:40: both participants (not the admin) are asked for a report.
	n, err := h.SendDailyReportRequests(ctx, TimeNow())
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	require.Len(t, env.tg.messagesTo(reportMashaTG), 1)
	assert.Contains(t, env.tg.messagesTo(reportMashaTG)[0].Form.Get("text"), "report of merits and failures")
	assert.Contains(t, env.tg.messagesTo(reportMashaTG)[0].Form.Get("text"), "Voice messages are welcome")
	assert.Empty(t, env.tg.messagesTo(reportAdminTG))

	// A voice message is transcribed and appended.
	voice := privateMessage(reportMashaTG, "")
	voice.Voice = &tgbotapi.Voice{FileID: "file-1", FileSize: 100}
	resp, handled := h.HandleDailyReportMessage(voice)
	assert.True(t, handled)
	assert.Nil(t, resp)
	require.Eventually(t, func() bool { return env.report(t, reportMashaTG).Parts == 1 }, 2*time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool { return len(env.tg.messagesTo(reportMashaTG)) == 2 }, 2*time.Second, 10*time.Millisecond)
	ack := env.tg.messagesTo(reportMashaTG)[1].Form
	assert.Contains(t, ack.Get("text"), "I heard: «Помыла посуду»")
	assert.Contains(t, ack.Get("reply_markup"), "rep_done:")

	// Text while the report is open is appended too.
	resp, handled = h.HandleDailyReportMessage(privateMessage(reportMashaTG, "Не сделала уроки"))
	assert.True(t, handled)
	assert.Contains(t, resp.(tgbotapi.MessageConfig).Text, "part 2")
	masha := env.report(t, reportMashaTG)
	assert.Equal(t, "Помыла посуду\nНе сделала уроки", masha.Text)

	// The admin's free text is not a report.
	_, handled = h.HandleDailyReportMessage(privateMessage(reportAdminTG, "Пусть Ваня дежурит"))
	assert.False(t, handled)

	// Only the author can press Done.
	resp, err = h.HandleReportCallback(callback(reportPetyaTG, "rep_done:"+strconvFormat(masha.ID)))
	require.NoError(t, err)
	assert.Nil(t, resp)

	// Done: the report is assessed and the admin gets the proposal.
	resp, err = h.HandleReportCallback(callback(reportMashaTG, "rep_done:"+strconvFormat(masha.ID)))
	require.NoError(t, err)
	assert.IsType(t, tgbotapi.EditMessageReplyMarkupConfig{}, resp)
	require.Eventually(t, func() bool { return len(env.tg.messagesTo(reportAdminTG)) == 1 }, 2*time.Second, 10*time.Millisecond)
	adminMsg := env.tg.messagesTo(reportAdminTG)[0].Form
	assert.Contains(t, adminMsg.Get("text"), "Daily report: Masha")
	assert.Contains(t, adminMsg.Get("text"), "Proposed score: <b>4</b>/5")
	assert.Contains(t, adminMsg.Get("text"), "&lt;молодец&gt;")
	assert.Contains(t, adminMsg.Get("reply_markup"), "rep_ok:"+strconvFormat(masha.ID))
	assert.Contains(t, adminMsg.Get("reply_markup"), "Confirm 4")
	assert.Equal(t, store.DailyReportProposed, env.report(t, reportMashaTG).Status)
	assert.Contains(t, env.analyzer.gotReport, "Не сделала уроки")
	assert.Contains(t, env.analyzer.gotFacts, "Was not on duty today")
	assert.Contains(t, env.analyzer.gotFacts, "Completed no chores today")

	// After Done, text goes elsewhere and voice is refused.
	_, handled = h.HandleDailyReportMessage(privateMessage(reportMashaTG, "А что у меня по делам?"))
	assert.False(t, handled)
	resp, handled = h.HandleDailyReportMessage(voice)
	assert.True(t, handled)
	assert.Equal(t, reportAlreadySentMessage, resp.(tgbotapi.MessageConfig).Text)

	// Participants can't confirm.
	resp, err = h.HandleReportCallback(callback(reportMashaTG, "rep_ok:"+strconvFormat(masha.ID)))
	require.NoError(t, err)
	assert.Nil(t, resp)

	// The admin confirms: only Masha's score is saved, no group post yet.
	resp, err = h.HandleReportCallback(callback(reportAdminTG, "rep_ok:"+strconvFormat(masha.ID)))
	require.NoError(t, err)
	assert.Contains(t, resp.(tgbotapi.EditMessageTextConfig).Text, "Confirmed</b>: Masha gets 4")
	ratings, err := env.store.GetCurrentMonthParticipantRatings(ctx, TimeNow())
	require.NoError(t, err)
	require.Len(t, ratings, 1)
	assert.Equal(t, 4, ratings[0].Score)
	assert.Empty(t, env.tg.messagesTo(reportGroupID))

	// A second press does nothing.
	resp, err = h.HandleReportCallback(callback(reportAdminTG, "rep_ok:"+strconvFormat(masha.ID)))
	require.NoError(t, err)
	assert.Contains(t, resp.(tgbotapi.EditMessageTextConfig).Text, "already handled")

	// Petya reports too; once he is confirmed, the group gets the summary.
	env.analyzer.score = 5
	_, handled = h.HandleDailyReportMessage(privateMessage(reportPetyaTG, "Вынес мусор"))
	require.True(t, handled)
	petya := env.report(t, reportPetyaTG)
	_, err = h.HandleReportCallback(callback(reportPetyaTG, "rep_done:"+strconvFormat(petya.ID)))
	require.NoError(t, err)
	require.Eventually(t, func() bool { return env.report(t, reportPetyaTG).Status == store.DailyReportProposed }, 2*time.Second, 10*time.Millisecond)
	_, err = h.HandleReportCallback(callback(reportAdminTG, "rep_ok:"+strconvFormat(petya.ID)))
	require.NoError(t, err)
	group := env.tg.messagesTo(reportGroupID)
	require.Len(t, group, 1)
	assert.Contains(t, group[0].Form.Get("text"), "Daily Ratings for "+reportTestDate)
	assert.Contains(t, group[0].Form.Get("text"), "Masha: 4")
	assert.Contains(t, group[0].Form.Get("text"), "Petya: 5")
}

func TestDailyReport_RejectAndFallbacks(t *testing.T) {
	env := newReportTestEnv(t, "Masha")
	h, ctx := env.h, context.Background()

	// Without a report open, text is not taken.
	_, handled := h.HandleDailyReportMessage(privateMessage(reportMashaTG, "Привет"))
	assert.False(t, handled)

	// /report opens it.
	msg, err := h.HandleReport(privateMessage(reportMashaTG, "/report"))
	require.NoError(t, err)
	assert.Contains(t, msg.Text, "report of merits and failures")

	// Done with nothing to send.
	r := env.report(t, reportMashaTG)
	resp, err := h.HandleReportCallback(callback(reportMashaTG, "rep_done:"+strconvFormat(r.ID)))
	require.NoError(t, err)
	assert.Contains(t, resp.(tgbotapi.MessageConfig).Text, "nothing to send yet")

	_, handled = h.HandleDailyReportMessage(privateMessage(reportMashaTG, "Подрался с братом"))
	require.True(t, handled)

	// The LLM fails: the admin gets the report unscored, without buttons.
	env.analyzer.err = errors.New("boom")
	_, err = h.HandleReportCallback(callback(reportMashaTG, "rep_done:"+strconvFormat(r.ID)))
	require.NoError(t, err)
	require.Eventually(t, func() bool { return len(env.tg.messagesTo(reportAdminTG)) == 1 }, 2*time.Second, 10*time.Millisecond)
	adminMsg := env.tg.messagesTo(reportAdminTG)[0].Form
	assert.Contains(t, adminMsg.Get("text"), "No AI assessment")
	assert.Contains(t, adminMsg.Get("text"), "Подрался с братом")
	assert.Empty(t, adminMsg.Get("reply_markup"))
	assert.Equal(t, store.DailyReportSubmitted, env.report(t, reportMashaTG).Status)

	// Reject on a proposal saves nothing.
	require.NoError(t, env.store.SaveDailyReportProposal(ctx, r.ID, 2, "Fought."))
	resp, err = h.HandleReportCallback(callback(reportAdminTG, "rep_no:"+strconvFormat(r.ID)))
	require.NoError(t, err)
	assert.Contains(t, resp.(tgbotapi.EditMessageTextConfig).Text, "Rejected")
	assert.Equal(t, store.DailyReportRejected, env.report(t, reportMashaTG).Status)
	ratings, err := env.store.GetCurrentMonthParticipantRatings(ctx, TimeNow())
	require.NoError(t, err)
	assert.Empty(t, ratings)
}

func TestDailyReport_NoTranscriber(t *testing.T) {
	env := newReportTestEnv(t, "Masha")
	env.h.Transcriber = nil

	voice := privateMessage(reportMashaTG, "")
	voice.Voice = &tgbotapi.Voice{FileID: "file-1"}
	resp, handled := env.h.HandleDailyReportMessage(voice)
	assert.True(t, handled)
	assert.Equal(t, reportNoVoiceMessage, resp.(tgbotapi.MessageConfig).Text)

	_, err := env.h.SendDailyReportRequests(context.Background(), TimeNow())
	require.NoError(t, err)
	assert.Contains(t, env.tg.messagesTo(reportMashaTG)[0].Form.Get("text"), "Write it as text")
}

func TestFinalizeDailyReports(t *testing.T) {
	env := newReportTestEnv(t, "Masha", "Petya")
	h, ctx := env.h, context.Background()

	_, err := h.SendDailyReportRequests(ctx, TimeNow())
	require.NoError(t, err)
	_, handled := h.HandleDailyReportMessage(privateMessage(reportMashaTG, "Убрала комнату"))
	require.True(t, handled)

	// Masha's report has content and is sent; Petya's is empty and stays open.
	n, err := h.FinalizeDailyReports(ctx, TimeNow())
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, store.DailyReportProposed, env.report(t, reportMashaTG).Status)
	assert.Equal(t, store.DailyReportCollecting, env.report(t, reportPetyaTG).Status)
	assert.Len(t, env.tg.messagesTo(reportAdminTG), 1)
}

func TestReportConfirmWindowOpen(t *testing.T) {
	berlin, err := time.LoadLocation(ratingTimezoneName)
	require.NoError(t, err)
	oct24 := time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)
	oct31 := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)

	assert.True(t, reportConfirmWindowOpen(oct24, time.Date(2026, 10, 25, 9, 0, 0, 0, berlin)))
	assert.True(t, reportConfirmWindowOpen(oct31, time.Date(2026, 10, 31, 20, 59, 0, 0, berlin)))
	assert.False(t, reportConfirmWindowOpen(oct31, time.Date(2026, 10, 31, 21, 0, 0, 0, berlin)))
	assert.False(t, reportConfirmWindowOpen(oct24, time.Date(2026, 11, 1, 9, 0, 0, 0, berlin)))
}
