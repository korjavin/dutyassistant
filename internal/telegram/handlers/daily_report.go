package handlers

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/llm"
	"github.com/korjavin/dutyassistant/internal/notification"
	"github.com/korjavin/dutyassistant/internal/store"
)

// Daily reports of merits and failures: at 20:00 participants are asked (in
// a DM) to tell what they did today, by voice or text. When they press Done
// (or at 20:45 for everyone else, with or without a report), the LLM
// proposes a score 1-5 with a short analysis and the admin confirms it or
// picks another score (or 5 with an ear) with buttons. This is the only way
// daily ratings are given.

// ReportAnalyzer proposes a daily score from a participant's report,
// following the admin's house rules (may be empty).
type ReportAnalyzer interface {
	AssessDailyReport(ctx context.Context, name, report, facts, rules string) (*llm.ReportAssessment, error)
}

const (
	reportCallbackDone    = "rep_done"
	reportCallbackApprove = "rep_ok"
	// reportCallbackScore sets a score chosen by the admin: "rep_set:<id>:<score>",
	// where score is 1-5 or "5e" (5 with an ear).
	reportCallbackScore = "rep_set"
	// reportCallbackReject is the Reject button of older messages; it now
	// just shows the score buttons.
	reportCallbackReject = "rep_no"

	// ReportDeadline is when FinalizeDailyReports runs (cron in main.go):
	// reports not sent by then go to the admin as they are, missing ones too.
	ReportDeadline = "20:45"
	// missingReportScore is proposed for a day without a report when the LLM
	// can't assess it.
	missingReportScore = 3

	// maxReportChars caps a report so the admin message fits Telegram's limit.
	maxReportChars = 6000
	// maxGroupSummaryRunes caps the one-line reason shown in the group.
	maxGroupSummaryRunes = 300

	reportAlreadySentMessage = "✅ Your report for today is already sent."
)

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
		"When you're finished, press ✅ Done — please send it by " + ReportDeadline + ", the admin rates the day after that."
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
// today's report while the report is being collected (voice messages arrive
// here already transcribed). It returns false for other messages, so they
// can go elsewhere (e.g. the assistant).
func (h *Handlers) HandleDailyReportMessage(m *tgbotapi.Message) (tgbotapi.Chattable, bool) {
	if m.Chat == nil || !m.Chat.IsPrivate() || m.From == nil {
		return nil, false
	}
	text := strings.TrimSpace(m.Text)
	if text == "" {
		return nil, false
	}

	ctx := context.Background()
	user, err := h.Store.GetUserByTelegramID(ctx, m.From.ID)
	if err != nil || user == nil || !h.isReportParticipant(user) {
		return nil, false
	}
	report, err := h.Store.GetDailyReport(ctx, user.ID, normalizeRatingDate(TimeNow()))
	if err != nil {
		slog.Error(fmt.Sprintf("[Report] Failed to load report for %s: %v", user.FirstName, err))
		return nil, false
	}
	if report == nil || report.Status != store.DailyReportCollecting {
		return nil, false
	}
	return h.appendReportPart(ctx, m.Chat.ID, report, text), true
}

// appendReportPart adds text to the report and returns the acknowledgement.
func (h *Handlers) appendReportPart(ctx context.Context, chatID int64, report *store.DailyReport, text string) tgbotapi.Chattable {
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
	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("📝 Got it (part %d). Add more, or press ✅ Done when you're finished.", report.Parts+1))
	msg.ReplyMarkup = reportDoneKeyboard(report.ID)
	return msg
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
// ("rep_done:<id>") and the admin's Confirm ("rep_ok:<id>") or chosen score
// ("rep_set:<id>:<score>").
func (h *Handlers) HandleReportCallback(q *tgbotapi.CallbackQuery) (tgbotapi.Chattable, error) {
	if q.From == nil || q.Message == nil {
		return nil, nil
	}
	action, rest, _ := strings.Cut(q.Data, ":")
	idStr, scoreStr, _ := strings.Cut(rest, ":")
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
	switch action {
	case reportCallbackApprove:
		if report.ProposedScore == 0 {
			return h.reportEdit(q, report, "", ""), nil
		}
		return h.rateReport(ctx, q, report, report.ProposedScore, false), nil
	case reportCallbackScore:
		score, hasEar, ok := parseScoreButton(scoreStr)
		if !ok {
			return nil, nil
		}
		return h.rateReport(ctx, q, report, score, hasEar), nil
	case reportCallbackReject:
		return h.reportEdit(q, report, "\n\nPick a score below.", ""), nil
	}
	return nil, nil
}

// parseScoreButton reads the score of a score button: "1".."5" or "5e".
func parseScoreButton(s string) (score int, hasEar bool, ok bool) {
	if s == "5e" {
		return 5, true, true
	}
	score, err := strconv.Atoi(s)
	if err != nil || score < 1 || score > 5 {
		return 0, false, false
	}
	return score, false, true
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
	// The assessment outlives this button press; keep it running on its own.
	go h.assessReport(context.WithoutCancel(ctx), report.ID)
	return removeButton
}

// FinalizeDailyReports runs at the report deadline: it submits every
// participant's report of date that is still open, so the admin can rate
// everyone. Reports whose authors forgot to press Done go as they are; a
// participant who told nothing gets a "no report" assessment.
func (h *Handlers) FinalizeDailyReports(ctx context.Context, date time.Time) (int, error) {
	participants, err := h.Store.GetParticipantsForRating(ctx)
	if err != nil {
		return 0, err
	}
	day := normalizeRatingDate(date)
	n := 0
	for _, p := range h.filterRatingParticipants(participants) {
		r, err := h.Store.EnsureDailyReport(ctx, p.ID, day)
		if err != nil {
			slog.Error(fmt.Sprintf("[Report] Failed to open report for %s: %v", p.FirstName, err))
			continue
		}
		if r.Status != store.DailyReportCollecting {
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
		if p.TelegramUserID != 0 {
			if r.Parts > 0 {
				h.sendText(p.TelegramUserID, "📨 I've sent what you told me today to the admin.")
			} else {
				h.sendText(p.TelegramUserID, "📭 No report from you today — the admin will rate your day without it.")
			}
		}
		h.assessReport(ctx, r.ID)
		n++
	}
	return n, nil
}

// assessReport asks the LLM for a proposed score and sends the report to the
// admin with the score buttons (and Confirm when there is a proposal). A day
// without a report gets missingReportScore when the LLM can't assess it.
func (h *Handlers) assessReport(ctx context.Context, reportID int64) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	report, err := h.Store.GetDailyReportByID(ctx, reportID)
	if err != nil || report == nil {
		slog.Error(fmt.Sprintf("[Report] Failed to load report %d for assessment: %v", reportID, err))
		return
	}

	if h.ReportAnalyzer != nil {
		rules, err := h.Store.GetSetting(ctx, store.SettingReportRules)
		if err != nil {
			// Better a score without the house rules than none at all.
			slog.Error(fmt.Sprintf("[Report] Failed to load the report rules: %v", err))
		}
		assessment, err := h.ReportAnalyzer.AssessDailyReport(ctx, report.UserName, report.Text, h.reportFacts(ctx, report), rules)
		if err != nil {
			slog.Error(fmt.Sprintf("[Report] LLM assessment of report %d failed: %v", reportID, err))
		} else {
			h.saveProposal(ctx, report, assessment.Score, assessment.Analysis, TruncateRunes(assessment.GroupSummary, maxGroupSummaryRunes))
		}
	}
	if report.Status != store.DailyReportProposed && isMissingReport(report) {
		h.saveProposal(ctx, report, missingReportScore, "No report was sent today.", "No report today.")
	}

	if h.AdminID == 0 {
		slog.Warn("[Report] ADMIN_ID not configured, nobody to send the report to")
		return
	}
	msg := tgbotapi.NewMessage(h.AdminID, formatReportForAdmin(report, ""))
	msg.ParseMode = tgbotapi.ModeHTML
	msg.ReplyMarkup = reportScoreKeyboard(report, "")
	h.send(msg)
}

// saveProposal stores a proposed score and updates report to match.
func (h *Handlers) saveProposal(ctx context.Context, report *store.DailyReport, score int, analysis, summary string) {
	if err := h.Store.SaveDailyReportProposal(ctx, report.ID, score, analysis, summary); err != nil {
		slog.Error(fmt.Sprintf("[Report] Failed to save the proposal for report %d: %v", report.ID, err))
		return
	}
	report.Status, report.ProposedScore, report.Analysis, report.GroupSummary = store.DailyReportProposed, score, analysis, summary
}

// isMissingReport reports whether the participant told nothing today.
func isMissingReport(report *store.DailyReport) bool {
	return strings.TrimSpace(report.Text) == ""
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

// reportScoreKeyboard has Confirm for the proposed score (until a score is
// saved) and a row of scores 1-5 plus 5 with an ear; current ("4", "5e") is
// the saved score, marked, and can still be changed.
func reportScoreKeyboard(report *store.DailyReport, current string) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	if current == "" && report.ProposedScore > 0 {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("✅ Confirm %d", report.ProposedScore), fmt.Sprintf("%s:%d", reportCallbackApprove, report.ID))))
	}
	var scores []tgbotapi.InlineKeyboardButton
	for _, token := range []string{"1", "2", "3", "4", "5", "5e"} {
		label := strings.Replace(token, "e", "👂", 1)
		if token == current {
			label = "✓" + label
		}
		scores = append(scores, tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("%s:%d:%s", reportCallbackScore, report.ID, token)))
	}
	return tgbotapi.NewInlineKeyboardMarkup(append(rows, scores)...)
}

// scoreToken renders a score as on the buttons and in calendars: "4", "5e".
func scoreToken(score int, hasEar bool) string {
	if hasEar {
		return strconv.Itoa(score) + "e"
	}
	return strconv.Itoa(score)
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
			sb.WriteString(html.EscapeString(TruncateRunes(report.Analysis, 1200)))
			sb.WriteString("\n")
		}
		if report.GroupSummary != "" {
			fmt.Fprintf(&sb, "👥 For the group: <i>%s</i>\n", html.EscapeString(report.GroupSummary))
		}
	} else {
		sb.WriteString("⚠️ No AI assessment (the LLM is unavailable). Pick a score below.\n")
	}
	if isMissingReport(report) {
		sb.WriteString("\n<b>Report:</b> <i>not sent</i>")
	} else {
		sb.WriteString("\n<b>Report:</b>\n<i>")
		sb.WriteString(html.EscapeString(TruncateRunes(report.Text, 2400)))
		sb.WriteString("</i>")
	}
	sb.WriteString(footer)
	return sb.String()
}

// reportEdit re-renders the admin's report message with footer (HTML) and
// the score buttons, current being the saved score ("" = none yet).
func (h *Handlers) reportEdit(q *tgbotapi.CallbackQuery, report *store.DailyReport, footer, current string) tgbotapi.Chattable {
	e := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, formatReportForAdmin(report, footer))
	e.ParseMode = tgbotapi.ModeHTML
	kb := reportScoreKeyboard(report, current)
	e.ReplyMarkup = &kb
	return e
}

// rateReport saves the admin's score for the report's participant and day
// (the proposed one, or another picked with the buttons) and posts the day's
// summary to the group once everyone is rated. Pressing another score
// later changes it.
func (h *Handlers) rateReport(ctx context.Context, q *tgbotapi.CallbackQuery, report *store.DailyReport, score int, hasEar bool) tgbotapi.Chattable {
	name := html.EscapeString(report.UserName)
	date := report.ReportDate.Format("2006-01-02")
	token := scoreToken(score, hasEar)

	if report.Status == store.DailyReportCollecting {
		return h.reportEdit(q, report, "\n\nThis report is still being written.", "")
	}
	if !reportConfirmWindowOpen(report.ReportDate, TimeNow()) {
		e := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, formatReportForAdmin(report,
			fmt.Sprintf("\n\n⌛ Too late: the results for %s are already published.", report.ReportDate.Format("January 2006"))))
		e.ParseMode = tgbotapi.ModeHTML
		return e
	}

	h.ratingsMu.Lock()
	defer h.ratingsMu.Unlock()

	previous, err := h.dayRating(ctx, report.UserID, report.ReportDate)
	if err != nil {
		slog.Error(fmt.Sprintf("[Report] Failed to load the rating of report %d: %v", report.ID, err))
		return h.reportEdit(q, report, "\n\n❌ Failed to save the score, please try again.", "")
	}
	err = h.Store.SaveParticipantRating(ctx, &store.ParticipantDailyRating{
		ParticipantID:   report.UserID,
		ParticipantName: report.UserName,
		RatingDate:      report.ReportDate,
		Score:           score,
		HasEar:          hasEar,
	})
	if err != nil {
		slog.Error(fmt.Sprintf("[Report] Failed to save the rating of report %d: %v", report.ID, err))
		return h.reportEdit(q, report, "\n\n❌ Failed to save the score, please try again.", "")
	}
	if report.Status != store.DailyReportApproved {
		if _, err := h.Store.SetDailyReportStatus(ctx, report.ID, report.Status, store.DailyReportApproved); err != nil {
			slog.Error(fmt.Sprintf("[Report] Failed to mark report %d approved: %v", report.ID, err))
		}
	}
	slog.Info(fmt.Sprintf("[Report] Admin rated %s %s on %s", report.UserName, token, date))
	if previous == nil || previous.Score != score || previous.HasEar != hasEar {
		h.announceRatingsIfComplete(ctx, report.ReportDate)
	}
	shown := strings.Replace(token, "e", "👂", 1)
	return h.reportEdit(q, report, fmt.Sprintf("\n\n✅ <b>Confirmed</b>: %s gets %s for %s. Tap another score to change it.", name, shown, date), token)
}

// dayRating returns the participant's saved rating for date, or nil.
func (h *Handlers) dayRating(ctx context.Context, participantID int64, date time.Time) (*store.ParticipantDailyRating, error) {
	ratings, err := h.Store.GetCurrentMonthParticipantRatings(ctx, date)
	if err != nil {
		return nil, err
	}
	day := normalizeRatingDate(date)
	for _, r := range ratings {
		if r != nil && r.ParticipantID == participantID && r.RatingDate.Equal(day) {
			return r, nil
		}
	}
	return nil, nil
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
// once every participant has a score for date (again after a correction).
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
	msg := tgbotapi.NewMessage(h.GroupID, formatDailyAndMonthlySummary(dayRatings, totals, day, h.ratingReasons(ctx, day, dayRatings)))
	msg.ParseMode = tgbotapi.ModeHTML
	h.send(msg)
}

// ratingReason is the bot's explanation of a participant's day for the
// group summary.
type ratingReason struct {
	Text string
	// Suggested is the bot's proposed score when the saved score differs
	// from it (0 = the admin kept the bot's score), so the group sees both.
	Suggested int
}

// ratingReasons returns the group summaries of date's assessed reports by
// participant ID, for the daily summary. When the saved score is not the one
// the bot proposed (the admin picked another one), the reason is still
// shown, marked with the bot's suggested score.
func (h *Handlers) ratingReasons(ctx context.Context, date time.Time, ratings []*store.ParticipantDailyRating) map[int64]ratingReason {
	reports, err := h.Store.ListDailyReports(ctx, normalizeRatingDate(date))
	if err != nil {
		slog.Error(fmt.Sprintf("[Report] Failed to load reports for the summary: %v", err))
		return nil
	}
	scores := make(map[int64]int, len(ratings))
	for _, r := range ratings {
		if r != nil {
			scores[r.ParticipantID] = r.Score
		}
	}
	reasons := make(map[int64]ratingReason)
	for _, r := range reports {
		score, rated := scores[r.UserID]
		if !rated || r.ProposedScore == 0 || r.GroupSummary == "" {
			continue
		}
		reason := ratingReason{Text: r.GroupSummary}
		if score != r.ProposedScore {
			reason.Suggested = r.ProposedScore
		}
		reasons[r.UserID] = reason
	}
	return reasons
}

// TruncateRunes shortens s to at most n runes, adding "…" when cut.
func TruncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
