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
	gotRules  string
}

func (f *fakeAnalyzer) AssessDailyReport(ctx context.Context, name, report, facts, rules string) (*llm.ReportAssessment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gotReport, f.gotFacts, f.gotRules = report, facts, rules
	if f.err != nil {
		return nil, f.err
	}
	return &llm.ReportAssessment{Score: f.score, Analysis: "Помогала по дому <молодец>.", GroupSummary: name + " помогал(а) <дома>."}, nil
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
	h.FetchFile = func(ctx context.Context, fileID string) (io.ReadCloser, error) {
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
	require.NoError(t, env.store.SetSetting(ctx, store.SettingReportRules, "1. Будь добрее к нечищеным зубам."))

	// 20:00: both participants (not the admin) are asked for a report.
	n, err := h.SendDailyReportRequests(ctx, TimeNow())
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	require.Len(t, env.tg.messagesTo(reportMashaTG), 1)
	assert.Contains(t, env.tg.messagesTo(reportMashaTG)[0].Form.Get("text"), "report of merits and failures")
	assert.Contains(t, env.tg.messagesTo(reportMashaTG)[0].Form.Get("text"), "Voice messages are welcome")
	assert.Contains(t, env.tg.messagesTo(reportMashaTG)[0].Form.Get("text"), "by 20:45")
	assert.Empty(t, env.tg.messagesTo(reportAdminTG))

	// Messages while the report is open are appended (voice arrives here
	// already transcribed by the bot).
	resp, handled := h.HandleDailyReportMessage(privateMessage(reportMashaTG, "Помыла посуду"))
	assert.True(t, handled)
	assert.Contains(t, resp.(tgbotapi.MessageConfig).Text, "part 1")
	assert.Contains(t, *resp.(tgbotapi.MessageConfig).ReplyMarkup.(tgbotapi.InlineKeyboardMarkup).InlineKeyboard[0][0].CallbackData, "rep_done:")

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
	assert.Contains(t, adminMsg.Get("text"), "For the group: <i>Masha помогал(а) &lt;дома&gt;.</i>")
	assert.Contains(t, adminMsg.Get("reply_markup"), "rep_ok:"+strconvFormat(masha.ID))
	assert.Contains(t, adminMsg.Get("reply_markup"), "Confirm 4")
	assert.Contains(t, adminMsg.Get("reply_markup"), "rep_set:"+strconvFormat(masha.ID)+":5e")
	assert.Contains(t, adminMsg.Get("reply_markup"), "5👂")
	assert.NotContains(t, adminMsg.Get("reply_markup"), "Reject")
	assert.Equal(t, store.DailyReportProposed, env.report(t, reportMashaTG).Status)
	assert.Contains(t, env.analyzer.gotReport, "Не сделала уроки")
	assert.Contains(t, env.analyzer.gotFacts, "Was not on duty today")
	assert.Contains(t, env.analyzer.gotFacts, "Completed no chores today")
	assert.Equal(t, "1. Будь добрее к нечищеным зубам.", env.analyzer.gotRules)

	// After Done, messages go elsewhere (the assistant).
	_, handled = h.HandleDailyReportMessage(privateMessage(reportMashaTG, "А что у меня по делам?"))
	assert.False(t, handled)

	// Participants can't confirm.
	resp, err = h.HandleReportCallback(callback(reportMashaTG, "rep_ok:"+strconvFormat(masha.ID)))
	require.NoError(t, err)
	assert.Nil(t, resp)

	// The admin confirms: only Masha's score is saved, no group post yet.
	resp, err = h.HandleReportCallback(callback(reportAdminTG, "rep_ok:"+strconvFormat(masha.ID)))
	require.NoError(t, err)
	edit := resp.(tgbotapi.EditMessageTextConfig)
	assert.Contains(t, edit.Text, "Confirmed</b>: Masha gets 4")
	// The score buttons stay, with the saved score marked, so it can be changed.
	require.NotNil(t, edit.ReplyMarkup)
	require.Len(t, edit.ReplyMarkup.InlineKeyboard, 1)
	assert.Equal(t, "✓4", edit.ReplyMarkup.InlineKeyboard[0][3].Text)
	ratings, err := env.store.GetCurrentMonthParticipantRatings(ctx, TimeNow())
	require.NoError(t, err)
	require.Len(t, ratings, 1)
	assert.Equal(t, 4, ratings[0].Score)
	assert.Equal(t, store.DailyReportApproved, env.report(t, reportMashaTG).Status)
	assert.Empty(t, env.tg.messagesTo(reportGroupID))

	// A second press keeps the score.
	resp, err = h.HandleReportCallback(callback(reportAdminTG, "rep_ok:"+strconvFormat(masha.ID)))
	require.NoError(t, err)
	assert.Contains(t, resp.(tgbotapi.EditMessageTextConfig).Text, "Masha gets 4")

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
	assert.Contains(t, group[0].Form.Get("text"), "Masha: 4 — <i>Masha помогал(а) &lt;дома&gt;.</i>")
	assert.Contains(t, group[0].Form.Get("text"), "Petya: 5 — <i>Petya помогал(а) &lt;дома&gt;.</i>")

	// Pressing the same score again does not repost the summary.
	_, err = h.HandleReportCallback(callback(reportAdminTG, "rep_set:"+strconvFormat(petya.ID)+":5"))
	require.NoError(t, err)
	require.Len(t, env.tg.messagesTo(reportGroupID), 1)

	// The admin gives Petya an ear: the correction is posted again.
	resp, err = h.HandleReportCallback(callback(reportAdminTG, "rep_set:"+strconvFormat(petya.ID)+":5e"))
	require.NoError(t, err)
	assert.Contains(t, resp.(tgbotapi.EditMessageTextConfig).Text, "Petya gets 5👂")
	group = env.tg.messagesTo(reportGroupID)
	require.Len(t, group, 2)
	assert.Contains(t, group[1].Form.Get("text"), "Petya: 5e")
	ratings, err = env.store.GetCurrentMonthParticipantRatings(ctx, TimeNow())
	require.NoError(t, err)
	for _, r := range ratings {
		if r.ParticipantID == petya.UserID {
			assert.True(t, r.HasEar)
		}
	}
}

func TestRatingReasons(t *testing.T) {
	env := newReportTestEnv(t, "Masha", "Petya", "Vanya")
	ctx := context.Background()
	day := normalizeRatingDate(TimeNow())
	propose := func(tgID int64, score int, summary, status string) {
		r, err := env.store.EnsureDailyReport(ctx, env.users[tgID].ID, day)
		require.NoError(t, err)
		require.NoError(t, env.store.SaveDailyReportProposal(ctx, r.ID, score, "analysis", summary))
		if status != store.DailyReportProposed {
			_, err = env.store.SetDailyReportStatus(ctx, r.ID, store.DailyReportProposed, status)
			require.NoError(t, err)
		}
	}
	const vanyaTG = reportMashaTG + 2
	propose(reportMashaTG, 4, "Помыла посуду.", store.DailyReportApproved)
	propose(reportPetyaTG, 5, "Вынес мусор.", store.DailyReportApproved)
	propose(vanyaTG, 4, "Ничего особенного.", store.DailyReportRejected)
	masha, petya, vanya := env.users[reportMashaTG].ID, env.users[reportPetyaTG].ID, env.users[vanyaTG].ID

	rating := func(id int64, name string, score int) *store.ParticipantDailyRating {
		return &store.ParticipantDailyRating{ParticipantID: id, ParticipantName: name, Score: score, RatingDate: day}
	}
	// Masha's score was confirmed; the admin picked 3 for Petya and 2 for
	// Vanya (an older, rejected proposal): the group sees what the bot
	// suggested and why.
	ratings := []*store.ParticipantDailyRating{rating(masha, "Masha", 4), rating(petya, "Petya", 3), rating(vanya, "Vanya", 2)}
	got := env.h.ratingReasons(ctx, day, ratings)
	assert.Equal(t, map[int64]ratingReason{
		masha: {Text: "Помыла посуду."},
		petya: {Text: "Вынес мусор.", Suggested: 5},
		vanya: {Text: "Ничего особенного.", Suggested: 4},
	}, got)

	text := formatDailyAndMonthlySummary(ratings, nil, day, got)
	assert.Contains(t, text, "Masha: 4 — <i>Помыла посуду.</i>\n")
	assert.Contains(t, text, "Petya: 3 — 🤖 bot suggested 5: <i>Вынес мусор.</i>\n")
	assert.Contains(t, text, "Vanya: 2 — 🤖 bot suggested 4: <i>Ничего особенного.</i>\n")

	// Nobody without a rating or without an assessment gets a reason.
	assert.Empty(t, env.h.ratingReasons(ctx, day, nil))
}

func TestDailyReport_FallbacksAndScoreButtons(t *testing.T) {
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

	// Scores can't be given while the report is being written.
	resp, err = h.HandleReportCallback(callback(reportAdminTG, "rep_set:"+strconvFormat(r.ID)+":4"))
	require.NoError(t, err)
	assert.Contains(t, resp.(tgbotapi.EditMessageTextConfig).Text, "still being written")

	_, handled = h.HandleDailyReportMessage(privateMessage(reportMashaTG, "Подрался с братом"))
	require.True(t, handled)

	// The LLM fails: the admin gets the report unscored, with score buttons only.
	env.analyzer.err = errors.New("boom")
	_, err = h.HandleReportCallback(callback(reportMashaTG, "rep_done:"+strconvFormat(r.ID)))
	require.NoError(t, err)
	require.Eventually(t, func() bool { return len(env.tg.messagesTo(reportAdminTG)) == 1 }, 2*time.Second, 10*time.Millisecond)
	adminMsg := env.tg.messagesTo(reportAdminTG)[0].Form
	assert.Contains(t, adminMsg.Get("text"), "No AI assessment")
	assert.Contains(t, adminMsg.Get("text"), "Pick a score below")
	assert.Contains(t, adminMsg.Get("text"), "Подрался с братом")
	assert.Contains(t, adminMsg.Get("reply_markup"), "rep_set:"+strconvFormat(r.ID)+":1")
	assert.NotContains(t, adminMsg.Get("reply_markup"), "Confirm")
	assert.Equal(t, store.DailyReportSubmitted, env.report(t, reportMashaTG).Status)

	// Confirm without a proposal just shows the buttons.
	resp, err = h.HandleReportCallback(callback(reportAdminTG, "rep_ok:"+strconvFormat(r.ID)))
	require.NoError(t, err)
	assert.NotContains(t, resp.(tgbotapi.EditMessageTextConfig).Text, "Confirmed")

	// Reject on an older message saves nothing and shows the score buttons.
	require.NoError(t, env.store.SaveDailyReportProposal(ctx, r.ID, 2, "Fought.", ""))
	resp, err = h.HandleReportCallback(callback(reportAdminTG, "rep_no:"+strconvFormat(r.ID)))
	require.NoError(t, err)
	edit := resp.(tgbotapi.EditMessageTextConfig)
	assert.Contains(t, edit.Text, "Pick a score below")
	require.NotNil(t, edit.ReplyMarkup)
	assert.Equal(t, store.DailyReportProposed, env.report(t, reportMashaTG).Status)
	ratings, err := env.store.GetCurrentMonthParticipantRatings(ctx, TimeNow())
	require.NoError(t, err)
	assert.Empty(t, ratings)

	// Bad score buttons and participants' taps are ignored.
	for _, data := range []string{"rep_set:" + strconvFormat(r.ID) + ":6", "rep_set:" + strconvFormat(r.ID) + ":4e", "rep_set:" + strconvFormat(r.ID)} {
		resp, err = h.HandleReportCallback(callback(reportAdminTG, data))
		require.NoError(t, err)
		assert.Nil(t, resp, data)
	}
	resp, err = h.HandleReportCallback(callback(reportMashaTG, "rep_set:"+strconvFormat(r.ID)+":5"))
	require.NoError(t, err)
	assert.Nil(t, resp)

	// The admin picks another score than the proposal.
	resp, err = h.HandleReportCallback(callback(reportAdminTG, "rep_set:"+strconvFormat(r.ID)+":3"))
	require.NoError(t, err)
	assert.Contains(t, resp.(tgbotapi.EditMessageTextConfig).Text, "Masha gets 3")
	ratings, err = env.store.GetCurrentMonthParticipantRatings(ctx, TimeNow())
	require.NoError(t, err)
	require.Len(t, ratings, 1)
	assert.Equal(t, 3, ratings[0].Score)
	assert.False(t, ratings[0].HasEar)
	// Masha is the only participant: the group gets the summary with the
	// bot's suggestion next to the admin's score.
	group := env.tg.messagesTo(reportGroupID)
	require.Len(t, group, 1)
	assert.Contains(t, group[0].Form.Get("text"), "Masha: 3")
}

func TestDailyReport_NoTranscriber(t *testing.T) {
	env := newReportTestEnv(t, "Masha")
	env.h.Transcriber = nil

	_, err := env.h.SendDailyReportRequests(context.Background(), TimeNow())
	require.NoError(t, err)
	assert.Contains(t, env.tg.messagesTo(reportMashaTG)[0].Form.Get("text"), "Write it as text")
}

func TestTranscribeVoice(t *testing.T) {
	env := newReportTestEnv(t)
	ctx := context.Background()
	voice := privateMessage(reportMashaTG, "")
	voice.Voice = &tgbotapi.Voice{FileID: "file-1", FileSize: 100}

	assert.True(t, HasVoice(voice))
	assert.False(t, HasVoice(privateMessage(reportMashaTG, "text")))

	text, err := env.h.TranscribeVoice(ctx, voice)
	require.NoError(t, err)
	assert.Equal(t, "Помыла посуду", text)

	// Video notes and audio files count too.
	circle := privateMessage(reportMashaTG, "")
	circle.VideoNote = &tgbotapi.VideoNote{FileID: "file-2"}
	_, err = env.h.TranscribeVoice(ctx, circle)
	require.NoError(t, err)

	big := privateMessage(reportMashaTG, "")
	big.Voice = &tgbotapi.Voice{FileID: "file-3", FileSize: maxVoiceBytes + 1}
	_, err = env.h.TranscribeVoice(ctx, big)
	assert.Error(t, err)

	env.h.FetchFile = func(ctx context.Context, fileID string) (io.ReadCloser, error) {
		return nil, errors.New("telegram is down")
	}
	_, err = env.h.TranscribeVoice(ctx, voice)
	assert.ErrorContains(t, err, "telegram is down")

	env.h.Transcriber = nil
	assert.False(t, env.h.VoiceEnabled())
	_, err = env.h.TranscribeVoice(ctx, voice)
	assert.Error(t, err)
}

func TestFinalizeDailyReports(t *testing.T) {
	env := newReportTestEnv(t, "Masha", "Petya")
	h, ctx := env.h, context.Background()

	_, err := h.SendDailyReportRequests(ctx, TimeNow())
	require.NoError(t, err)
	_, handled := h.HandleDailyReportMessage(privateMessage(reportMashaTG, "Убрала комнату"))
	require.True(t, handled)

	// Both reports go to the admin: Masha's as it is, Petya's as missing.
	n, err := h.FinalizeDailyReports(ctx, TimeNow())
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.Equal(t, store.DailyReportProposed, env.report(t, reportMashaTG).Status)
	assert.Equal(t, store.DailyReportProposed, env.report(t, reportPetyaTG).Status)
	assert.Empty(t, env.analyzer.gotReport, "Petya's day is assessed without a report")
	admin := env.tg.messagesTo(reportAdminTG)
	require.Len(t, admin, 2)
	assert.Contains(t, admin[1].Form.Get("text"), "Daily report: Petya")
	assert.Contains(t, admin[1].Form.Get("text"), "<b>Report:</b> <i>not sent</i>")
	assert.Contains(t, admin[1].Form.Get("reply_markup"), "Confirm 4")
	petyaDMs := env.tg.messagesTo(reportPetyaTG)
	assert.Contains(t, petyaDMs[len(petyaDMs)-1].Form.Get("text"), "No report from you today")

	// A second run has nothing left to do.
	n, err = h.FinalizeDailyReports(ctx, TimeNow())
	require.NoError(t, err)
	assert.Equal(t, 0, n)
}

func TestFinalizeDailyReports_MissingReportWithoutLLM(t *testing.T) {
	env := newReportTestEnv(t, "Masha")
	env.h.ReportAnalyzer = nil
	h, ctx := env.h, context.Background()

	// Masha was never asked (no row yet) and sent nothing: she still gets
	// a proposal of 3, so the admin can confirm the day.
	n, err := h.FinalizeDailyReports(ctx, TimeNow())
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	r := env.report(t, reportMashaTG)
	assert.Equal(t, store.DailyReportProposed, r.Status)
	assert.Equal(t, missingReportScore, r.ProposedScore)
	admin := env.tg.messagesTo(reportAdminTG)
	require.Len(t, admin, 1)
	assert.Contains(t, admin[0].Form.Get("reply_markup"), "Confirm 3")

	_, err = h.HandleReportCallback(callback(reportAdminTG, "rep_ok:"+strconvFormat(r.ID)))
	require.NoError(t, err)
	group := env.tg.messagesTo(reportGroupID)
	require.Len(t, group, 1)
	assert.Contains(t, group[0].Form.Get("text"), "Masha: 3 — <i>No report today.</i>")
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
