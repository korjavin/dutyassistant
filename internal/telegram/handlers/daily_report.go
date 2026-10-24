package handlers

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/llm"
	"github.com/korjavin/dutyassistant/internal/notification"
	"github.com/korjavin/dutyassistant/internal/store"
)

// Daily reports of merits and failures: at 20:40 participants are asked (in
// a DM) to tell what they did today, by voice or text. When they press Done,
// the LLM proposes a score 1-5 with a short analysis and the admin confirms
// or rejects it with buttons; only a confirmed score is saved as the rating.

// ReportAnalyzer proposes a daily score from a participant's report.
type ReportAnalyzer interface {
	AssessDailyReport(ctx context.Context, name, report, facts string) (*llm.ReportAssessment, error)
}

// SpeechTranscriber turns a voice message into text.
type SpeechTranscriber interface {
	Transcribe(ctx context.Context, audio io.Reader, filename string) (string, error)
}

const (
	reportCallbackDone    = "rep_done"
	reportCallbackApprove = "rep_ok"
	reportCallbackReject  = "rep_no"

	// maxReportChars caps a report so the admin message fits Telegram's limit.
	maxReportChars = 6000
	// maxVoiceBytes is the largest file the Telegram Bot API lets bots download.
	maxVoiceBytes = 20 << 20

	reportAlreadySentMessage = "✅ Your report for today is already sent."
	reportNoVoiceMessage     = "🎙 I can't listen to voice messages right now — please write your report as text."
)

// reportAudio is a voice-like attachment of a message.
type reportAudio struct {
	FileID   string
	FileName string
	Size     int
}

// reportAudioOf returns the voice message, audio file or video note of m, if any.
func reportAudioOf(m *tgbotapi.Message) *reportAudio {
	switch {
	case m.Voice != nil:
		return &reportAudio{FileID: m.Voice.FileID, FileName: "voice.ogg", Size: m.Voice.FileSize}
	case m.Audio != nil:
		name := m.Audio.FileName
		if name == "" {
			name = "audio.mp3"
		}
		return &reportAudio{FileID: m.Audio.FileID, FileName: name, Size: m.Audio.FileSize}
	case m.VideoNote != nil:
		return &reportAudio{FileID: m.VideoNote.FileID, FileName: "video_note.mp4", Size: m.VideoNote.FileSize}
	}
	return nil
}

// isReportParticipant reports whether the user is asked for daily reports:
// the same people who get daily ratings.
func (h *Handlers) isReportParticipant(user *store.User) bool {
	return user != nil && user.IsActive && len(h.filterRatingParticipants([]*store.User{user})) == 1
}

// SendDailyReportRequests asks every rating participant for today's report
// in a DM. People whose report is already sent are skipped. It returns how
// many requests were sent.
func (h *Handlers) SendDailyReportRequests(ctx context.Context, now time.Time) (int, error) {
	if h.Bot == nil {
		return 0, fmt.Errorf("bot not configured")
	}
	participants, err := h.Store.GetParticipantsForRating(ctx)
	if err != nil {
		return 0, err
	}
	date := normalizeRatingDate(now)
	sent := 0
	for _, p := range h.filterRatingParticipants(participants) {
		if p.TelegramUserID == 0 {
			continue
		}
		report, err := h.Store.EnsureDailyReport(ctx, p.ID, date)
		if err != nil {
			slog.Error(fmt.Sprintf("[Report] Failed to open report for %s: %v", p.FirstName, err))
			continue
		}
		if report.Status != store.DailyReportCollecting {
			continue
		}
		if _, err := h.Bot.Send(h.reportRequestMessage(p.TelegramUserID, report)); err != nil {
			slog.Error(fmt.Sprintf("[Report] Failed to ask %s for a report: %v", p.FirstName, err))
			continue
		}
		sent++
	}
	return sent, nil
}

func (h *Handlers) reportRequestMessage(chatID int64, report *store.DailyReport) tgbotapi.MessageConfig {
	how := "Voice messages are welcome (one or several), or just write."
	if h.Transcriber == nil {
		how = "Write it as text, in one or several messages."
	}
	text := "📝 Time for today's report of merits and failures!\n\n" +
		"Tell me what you did today: what went well and what didn't. " + how + "\n" +
		"When you're finished, press ✅ Done — the admin will look at it before today's ratings."
	msg := tgbotapi.NewMessage(chatID, text)
	if report.Parts > 0 {
		msg.Text += fmt.Sprintf("\n\nI already have %d part(s) of your report — add more or press ✅ Done.", report.Parts)
		msg.ReplyMarkup = reportDoneKeyboard(report.ID)
	}
	return msg
}

func reportDoneKeyboard(reportID int64) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("✅ Done, send it", fmt.Sprintf("%s:%d", reportCallbackDone, reportID)),
	))
}

// HandleReport handles /report: a participant opens (or reopens) today's report.
func (h *Handlers) HandleReport(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	if !m.Chat.IsPrivate() {
		return tgbotapi.NewMessage(m.Chat.ID, "Send /report to me in a private chat."), nil
	}
	ctx := context.Background()
	user, err := h.Store.GetUserByTelegramID(ctx, m.From.ID)
	if err != nil || user == nil {
		return tgbotapi.NewMessage(m.Chat.ID, "Could not find your user profile. Please use /start first."), nil
	}
	if !h.isReportParticipant(user) {
		return tgbotapi.NewMessage(m.Chat.ID, "Daily reports are for active participants."), nil
	}
	report, err := h.Store.EnsureDailyReport(ctx, user.ID, normalizeRatingDate(TimeNow()))
	if err != nil {
		slog.Error(fmt.Sprintf("[Report] Failed to open report for %s: %v", user.FirstName, err))
		return tgbotapi.NewMessage(m.Chat.ID, genericErrorMessage), nil
	}
	if report.Status != store.DailyReportCollecting {
		return tgbotapi.NewMessage(m.Chat.ID, reportAlreadySentMessage), nil
	}
	return h.reportRequestMessage(m.Chat.ID, report), nil
}

// HandleDailyReportMessage takes a participant's private message as part of
// today's report: voice messages always (they start a report if needed),
// text only while a report is being collected. It returns false for messages
// that are not report parts, so they can go elsewhere (e.g. the assistant).
func (h *Handlers) HandleDailyReportMessage(m *tgbotapi.Message) (tgbotapi.Chattable, bool) {
	if m.Chat == nil || !m.Chat.IsPrivate() || m.From == nil {
		return nil, false
	}
	audio := reportAudioOf(m)
	text := strings.TrimSpace(m.Text)
	if audio == nil && text == "" {
		return nil, false
	}

	ctx := context.Background()
	user, err := h.Store.GetUserByTelegramID(ctx, m.From.ID)
	if err != nil || user == nil || !h.isReportParticipant(user) {
		return nil, false
	}
	date := normalizeRatingDate(TimeNow())
	report, err := h.Store.GetDailyReport(ctx, user.ID, date)
	if err != nil {
		slog.Error(fmt.Sprintf("[Report] Failed to load report for %s: %v", user.FirstName, err))
		if audio == nil {
			return nil, false
		}
		return tgbotapi.NewMessage(m.Chat.ID, genericErrorMessage), true
	}

	if audio == nil {
		if report == nil || report.Status != store.DailyReportCollecting {
			return nil, false
		}
		return h.appendReportPart(ctx, m.Chat.ID, report, text, false), true
	}

	if report == nil {
		if report, err = h.Store.EnsureDailyReport(ctx, user.ID, date); err != nil {
			slog.Error(fmt.Sprintf("[Report] Failed to open report for %s: %v", user.FirstName, err))
			return tgbotapi.NewMessage(m.Chat.ID, genericErrorMessage), true
		}
	}
	if report.Status != store.DailyReportCollecting {
		return tgbotapi.NewMessage(m.Chat.ID, reportAlreadySentMessage), true
	}
	if h.Transcriber == nil {
		return tgbotapi.NewMessage(m.Chat.ID, reportNoVoiceMessage), true
	}
	if audio.Size > maxVoiceBytes {
		return tgbotapi.NewMessage(m.Chat.ID, "This recording is too large for me. Please send a shorter one."), true
	}
	// Downloading and transcribing takes a while; don't block other updates.
	go h.transcribeReportPart(m.Chat.ID, report.ID, audio)
	return nil, true
}

// appendReportPart adds text to the report and returns the acknowledgement.
// heard marks text recognized from a voice message, which is echoed back.
func (h *Handlers) appendReportPart(ctx context.Context, chatID int64, report *store.DailyReport, text string, heard bool) tgbotapi.Chattable {
	if len(report.Text)+len(text) > maxReportChars {
		msg := tgbotapi.NewMessage(chatID, "Your report is already long enough 🙂 Press ✅ Done to send it.")
		msg.ReplyMarkup = reportDoneKeyboard(report.ID)
		return msg
	}
	ok, err := h.Store.AppendDailyReport(ctx, report.ID, text)
	if err != nil {
		slog.Error(fmt.Sprintf("[Report] Failed to append to report %d: %v", report.ID, err))
		return tgbotapi.NewMessage(chatID, genericErrorMessage)
	}
	if !ok {
		return tgbotapi.NewMessage(chatID, reportAlreadySentMessage)
	}
	var sb strings.Builder
	if heard {
		fmt.Fprintf(&sb, "🎧 I heard: «%s»\n\n", truncateRunes(text, 1500))
	}
	fmt.Fprintf(&sb, "📝 Got it (part %d). Add more, or press ✅ Done when you're finished.", report.Parts+1)
	msg := tgbotapi.NewMessage(chatID, sb.String())
	msg.ReplyMarkup = reportDoneKeyboard(report.ID)
	return msg
}

// transcribeReportPart downloads a voice message, transcribes it and appends
// it to the report. Parts of one report are processed in order.
func (h *Handlers) transcribeReportPart(chatID, reportID int64, audio *reportAudio) {
	lock := h.reportLock(reportID)
	lock.Lock()
	defer lock.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	text, err := h.transcribe(ctx, audio)
	if err != nil {
		slog.Error(fmt.Sprintf("[Report] Failed to transcribe a voice message for report %d: %v", reportID, err))
		h.sendText(chatID, "😕 I couldn't make out that voice message. Please try again, or write it as text.")
		return
	}
	if text == "" {
		h.sendText(chatID, "🤔 I didn't hear any words in that recording. Please try again.")
		return
	}
	report, err := h.Store.GetDailyReportByID(ctx, reportID)
	if err != nil || report == nil {
		slog.Error(fmt.Sprintf("[Report] Failed to reload report %d: %v", reportID, err))
		h.sendText(chatID, genericErrorMessage)
		return
	}
	h.send(h.appendReportPart(ctx, chatID, report, text, true))
}

func (h *Handlers) transcribe(ctx context.Context, audio *reportAudio) (string, error) {
	fetch := h.fetchFile
	if fetch == nil {
		fetch = h.downloadTelegramFile
	}
	body, err := fetch(ctx, audio.FileID)
	if err != nil {
		return "", err
	}
	defer func() { _ = body.Close() }()
	return h.Transcriber.Transcribe(ctx, io.LimitReader(body, maxVoiceBytes), audio.FileName)
}

// downloadTelegramFile streams a file the bot received.
func (h *Handlers) downloadTelegramFile(ctx context.Context, fileID string) (io.ReadCloser, error) {
	if h.Bot == nil {
		return nil, fmt.Errorf("bot not configured")
	}
	fileURL, err := h.Bot.GetFileDirectURL(fileID)
	if err != nil {
		return nil, fmt.Errorf("could not get the file from Telegram: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return nil, fmt.Errorf("could not build the download request")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// The URL contains the bot token: keep it out of errors and logs.
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return nil, fmt.Errorf("could not download the file: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("could not download the file: status %d", resp.StatusCode)
	}
	return resp.Body, nil
}

func (h *Handlers) reportLock(reportID int64) *sync.Mutex {
	lock, _ := h.reportLocks.LoadOrStore(reportID, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

func (h *Handlers) send(c tgbotapi.Chattable) {
	if h.Bot == nil || c == nil {
		return
	}
	if _, err := h.Bot.Send(c); err != nil {
		slog.Error(fmt.Sprintf("[Report] Failed to send a message: %v", err))
	}
}

func (h *Handlers) sendText(chatID int64, text string) {
	h.send(tgbotapi.NewMessage(chatID, text))
}

// HandleReportCallback handles the report buttons: the participant's Done
// ("rep_done:<id>") and the admin's Confirm / Reject ("rep_ok:<id>" / "rep_no:<id>").
func (h *Handlers) HandleReportCallback(q *tgbotapi.CallbackQuery) (tgbotapi.Chattable, error) {
	if q.From == nil || q.Message == nil {
		return nil, nil
	}
	action, idStr, _ := strings.Cut(q.Data, ":")
	reportID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return nil, nil
	}
	ctx := context.Background()
	report, err := h.Store.GetDailyReportByID(ctx, reportID)
	if err != nil {
		slog.Error(fmt.Sprintf("[Report] Failed to load report %d: %v", reportID, err))
		return tgbotapi.NewMessage(q.Message.Chat.ID, genericErrorMessage), nil
	}
	if report == nil {
		return nil, nil
	}

	if action == reportCallbackDone {
		if q.From.ID != report.TelegramUserID {
			return nil, nil
		}
		return h.submitReport(ctx, q, report), nil
	}
	if !h.IsAdmin(q.From.ID) {
		return nil, nil // Ignore non-admin clicks silently
	}
	return h.decideReport(ctx, q, report, action == reportCallbackApprove), nil
}

// submitReport finishes collecting and sends the report for assessment.
func (h *Handlers) submitReport(ctx context.Context, q *tgbotapi.CallbackQuery, report *store.DailyReport) tgbotapi.Chattable {
	chatID := q.Message.Chat.ID
	removeButton := tgbotapi.NewEditMessageReplyMarkup(chatID, q.Message.MessageID,
		tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{}})

	if report.Status != store.DailyReportCollecting {
		h.sendText(chatID, reportAlreadySentMessage)
		return removeButton
	}
	if report.Parts == 0 {
		return tgbotapi.NewMessage(chatID, "There's nothing to send yet — record a voice message or write what you did today.")
	}
	ok, err := h.Store.SetDailyReportStatus(ctx, report.ID, store.DailyReportCollecting, store.DailyReportSubmitted)
	if err != nil {
		slog.Error(fmt.Sprintf("[Report] Failed to submit report %d: %v", report.ID, err))
		return tgbotapi.NewMessage(chatID, genericErrorMessage)
	}
	if !ok {
		h.sendText(chatID, reportAlreadySentMessage)
		return removeButton
	}
	slog.Info(fmt.Sprintf("[Report] %s submitted the report for %s", report.UserName, report.ReportDate.Format("2006-01-02")))
	h.sendText(chatID, "🙏 Thanks! Your report is sent to the admin.")
	go h.assessReport(report.ID)
	return removeButton
}

// FinalizeDailyReports submits the reports of date that have parts but whose
// authors forgot to press Done, so the admin still gets them.
func (h *Handlers) FinalizeDailyReports(ctx context.Context, date time.Time) (int, error) {
	reports, err := h.Store.ListDailyReports(ctx, normalizeRatingDate(date))
	if err != nil {
		return 0, err
	}
	n := 0
	for _, r := range reports {
		if r.Status != store.DailyReportCollecting || r.Parts == 0 {
			continue
		}
		ok, err := h.Store.SetDailyReportStatus(ctx, r.ID, store.DailyReportCollecting, store.DailyReportSubmitted)
		if err != nil {
			slog.Error(fmt.Sprintf("[Report] Failed to submit report %d: %v", r.ID, err))
			continue
		}
		if !ok {
			continue
		}
		h.sendText(r.TelegramUserID, "📨 I've sent what you told me today to the admin.")
		h.assessReport(r.ID)
		n++
	}
	return n, nil
}

// assessReport asks the LLM for a proposed score and sends the report to the
// admin: with Confirm / Reject buttons when there is a proposal, or as plain
// information (rate manually) when the LLM is unavailable or fails.
func (h *Handlers) assessReport(reportID int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	report, err := h.Store.GetDailyReportByID(ctx, reportID)
	if err != nil || report == nil {
		slog.Error(fmt.Sprintf("[Report] Failed to load report %d for assessment: %v", reportID, err))
		return
	}

	if h.ReportAnalyzer != nil {
		assessment, err := h.ReportAnalyzer.AssessDailyReport(ctx, report.UserName, report.Text, h.reportFacts(ctx, report))
		switch {
		case err != nil:
			slog.Error(fmt.Sprintf("[Report] LLM assessment of report %d failed: %v", reportID, err))
		default:
			if err := h.Store.SaveDailyReportProposal(ctx, reportID, assessment.Score, assessment.Analysis); err != nil {
				slog.Error(fmt.Sprintf("[Report] Failed to save the proposal for report %d: %v", reportID, err))
			} else {
				report.Status, report.ProposedScore, report.Analysis = store.DailyReportProposed, assessment.Score, assessment.Analysis
			}
		}
	}

	if h.AdminID == 0 {
		slog.Warn("[Report] ADMIN_ID not configured, nobody to send the report to")
		return
	}
	msg := tgbotapi.NewMessage(h.AdminID, formatReportForAdmin(report, ""))
	msg.ParseMode = tgbotapi.ModeHTML
	if report.Status == store.DailyReportProposed {
		msg.ReplyMarkup = reportDecisionKeyboard(report)
	}
	h.send(msg)
}

// reportFacts describes what the bot knows about the participant's day, so
// the LLM can weigh the report against it.
func (h *Handlers) reportFacts(ctx context.Context, report *store.DailyReport) string {
	loc, err := time.LoadLocation(ratingTimezoneName)
	if err != nil {
		loc = time.UTC
	}
	d := report.ReportDate
	dayStart := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc)
	now := TimeNow()

	var sb strings.Builder
	if duty, err := h.Store.GetDutyByDate(ctx, d); err == nil && duty != nil && duty.UserID == report.UserID {
		sb.WriteString("- Was on duty today.\n")
	} else if err == nil {
		sb.WriteString("- Was not on duty today.\n")
	}

	if done, err := h.Store.GetChoresCompletedBetween(ctx, report.UserID, dayStart, dayStart.AddDate(0, 0, 1)); err == nil {
		if len(done) == 0 {
			sb.WriteString("- Completed no chores today.\n")
		} else {
			sb.WriteString("- Chores completed today:\n")
			for _, c := range done {
				fmt.Fprintf(&sb, "  * \"%s\"\n", c.Description)
			}
		}
	}

	if pending, err := h.Store.GetActiveChoresByUserID(ctx, report.UserID); err == nil {
		if len(pending) == 0 {
			sb.WriteString("- Has no pending chores.\n")
		} else {
			sb.WriteString("- Pending chores:\n")
			sb.WriteString(notification.DescribeChoreDelays(pending, now))
		}
	}

	if sb.Len() == 0 {
		return "(no data)"
	}
	return sb.String()
}

func reportDecisionKeyboard(report *store.DailyReport) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("✅ Confirm %d", report.ProposedScore), fmt.Sprintf("%s:%d", reportCallbackApprove, report.ID)),
		tgbotapi.NewInlineKeyboardButtonData("❌ Reject", fmt.Sprintf("%s:%d", reportCallbackReject, report.ID)),
	))
}

// formatReportForAdmin renders the report, the proposed score and the
// analysis as HTML, followed by footer (already HTML).
func formatReportForAdmin(report *store.DailyReport, footer string) string {
	name := html.EscapeString(report.UserName)
	var sb strings.Builder
	fmt.Fprintf(&sb, "📝 <b>Daily report: %s</b> (%s)\n\n", name, report.ReportDate.Format("2006-01-02"))
	if report.ProposedScore > 0 {
		fmt.Fprintf(&sb, "🤖 Proposed score: <b>%d</b>/5\n", report.ProposedScore)
		if report.Analysis != "" {
			sb.WriteString(html.EscapeString(truncateRunes(report.Analysis, 1200)))
			sb.WriteString("\n")
		}
	} else {
		fmt.Fprintf(&sb, "⚠️ No AI assessment (the LLM is unavailable). Rate %s in the daily ratings prompt.\n", name)
	}
	sb.WriteString("\n<b>Report:</b>\n<i>")
	sb.WriteString(html.EscapeString(truncateRunes(report.Text, 2400)))
	sb.WriteString("</i>")
	sb.WriteString(footer)
	return sb.String()
}

// decideReport applies the admin's Confirm / Reject to a proposed score.
func (h *Handlers) decideReport(ctx context.Context, q *tgbotapi.CallbackQuery, report *store.DailyReport, approve bool) tgbotapi.Chattable {
	edit := func(footer string, keepButtons bool) tgbotapi.Chattable {
		e := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, formatReportForAdmin(report, footer))
		e.ParseMode = tgbotapi.ModeHTML
		if keepButtons {
			kb := reportDecisionKeyboard(report)
			e.ReplyMarkup = &kb
		}
		return e
	}
	name := html.EscapeString(report.UserName)
	date := report.ReportDate.Format("2006-01-02")

	if report.Status != store.DailyReportProposed {
		return edit("\n\nThis report was already handled.", false)
	}

	if !approve {
		ok, err := h.Store.SetDailyReportStatus(ctx, report.ID, store.DailyReportProposed, store.DailyReportRejected)
		if err != nil {
			slog.Error(fmt.Sprintf("[Report] Failed to reject report %d: %v", report.ID, err))
			return edit("\n\n❌ Failed to reject, please try again.", true)
		}
		if !ok {
			return edit("\n\nThis report was already handled.", false)
		}
		return edit(fmt.Sprintf("\n\n❌ <b>Rejected</b> — no score saved. Rate %s in the daily ratings prompt if needed.", name), false)
	}

	if !reportConfirmWindowOpen(report.ReportDate, TimeNow()) {
		return edit(fmt.Sprintf("\n\n⌛ Too late: the results for %s are already published.", report.ReportDate.Format("January 2006")), false)
	}

	h.ratingsMu.Lock()
	defer h.ratingsMu.Unlock()

	ok, err := h.Store.SetDailyReportStatus(ctx, report.ID, store.DailyReportProposed, store.DailyReportApproved)
	if err != nil {
		slog.Error(fmt.Sprintf("[Report] Failed to approve report %d: %v", report.ID, err))
		return edit("\n\n❌ Failed to save the score, please try again.", true)
	}
	if !ok {
		return edit("\n\nThis report was already handled.", false)
	}
	err = h.Store.SaveParticipantRating(ctx, &store.ParticipantDailyRating{
		ParticipantID:   report.UserID,
		ParticipantName: report.UserName,
		RatingDate:      report.ReportDate,
		Score:           report.ProposedScore,
	})
	if err != nil {
		slog.Error(fmt.Sprintf("[Report] Failed to save the rating of report %d: %v", report.ID, err))
		if _, revertErr := h.Store.SetDailyReportStatus(ctx, report.ID, store.DailyReportApproved, store.DailyReportProposed); revertErr != nil {
			slog.Error(fmt.Sprintf("[Report] Failed to reopen report %d: %v", report.ID, revertErr))
		}
		return edit("\n\n❌ Failed to save the score, please try again.", true)
	}
	slog.Info(fmt.Sprintf("[Report] Admin confirmed %d for %s on %s", report.ProposedScore, report.UserName, date))
	h.announceRatingsIfComplete(ctx, report.ReportDate)
	return edit(fmt.Sprintf("\n\n✅ <b>Confirmed</b>: %s gets %d for %s.", name, report.ProposedScore, date), false)
}

// reportConfirmWindowOpen reports whether a score for date may still be
// saved: until the month's winners are announced (21:00 on its last day).
func reportConfirmWindowOpen(date, now time.Time) bool {
	loc, err := time.LoadLocation(ratingTimezoneName)
	if err != nil {
		loc = time.UTC
	}
	d := normalizeRatingDate(date)
	cutoff := time.Date(d.Year(), d.Month()+1, 0, 21, 0, 0, 0, loc)
	return now.Before(cutoff)
}

// announceRatingsIfComplete posts the daily and monthly summary to the group
// once every participant has a score for date, like the manual rating flow.
// Callers hold ratingsMu.
func (h *Handlers) announceRatingsIfComplete(ctx context.Context, date time.Time) {
	if h.Bot == nil || h.GroupID == 0 {
		return
	}
	participants, err := h.Store.GetParticipantsForRating(ctx)
	if err != nil {
		slog.Error(fmt.Sprintf("[Report] Failed to load participants: %v", err))
		return
	}
	participants = h.filterRatingParticipants(participants)
	if len(participants) == 0 {
		return
	}
	monthRatings, err := h.Store.GetCurrentMonthParticipantRatings(ctx, date)
	if err != nil {
		slog.Error(fmt.Sprintf("[Report] Failed to load ratings: %v", err))
		return
	}
	day := normalizeRatingDate(date)
	rated := make(map[int64]bool)
	var dayRatings []*store.ParticipantDailyRating
	for _, r := range monthRatings {
		if r != nil && r.RatingDate.Equal(day) {
			rated[r.ParticipantID] = true
			dayRatings = append(dayRatings, r)
		}
	}
	for _, p := range participants {
		if !rated[p.ID] {
			return
		}
	}
	totals, err := h.Store.GetMonthlyParticipantTotals(ctx, day.Year(), day.Month())
	if err != nil {
		slog.Error(fmt.Sprintf("[Report] Failed to load monthly totals: %v", err))
		return
	}
	msg := tgbotapi.NewMessage(h.GroupID, formatDailyAndMonthlySummary(dayRatings, totals, day))
	msg.ParseMode = tgbotapi.ModeHTML
	h.send(msg)
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
