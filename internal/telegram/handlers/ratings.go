package handlers

import (
	"context"
	"fmt"
	"log"
	"html"
	"slices"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/store"
)

const (
	ratingSessionParticipantsKey = "participants"
	ratingSessionDateKey         = "rating_date"
	ratingTimezoneName           = "Europe/Berlin"
)

type ratingSessionParticipant struct {
	ID   int64
	Name string
}

// PrepareDailyRatingsReminder starts the daily rating session and returns the prompt when participants exist.
func (h *Handlers) PrepareDailyRatingsReminder(chatID, userID int64, ratingDate time.Time) (*tgbotapi.MessageConfig, bool, error) {
	isAdmin, err := h.checkAdmin(userID)
	if err != nil {
		return nil, false, err
	}
	if !isAdmin {
		msg := tgbotapi.NewMessage(chatID, adminOnlyMessage)
		return &msg, true, nil
	}
	if session, exists := h.SessionManager.GetSession(chatID); exists && session.Type != SessionTypeDailyRatings {
		msg := tgbotapi.NewMessage(chatID, "Finish your current admin workflow before submitting participant ratings.")
		return &msg, true, nil
	}

	participants, err := h.Store.GetParticipantsForRating(context.Background())
	if err != nil {
		return nil, false, err
	}
	participants = h.filterRatingParticipants(participants)
	if len(participants) == 0 {
		return nil, false, nil
	}

	h.SessionManager.StartSession(chatID, userID, SessionTypeDailyRatings)
	session, _ := h.SessionManager.GetSession(chatID)
	session.SetData(ratingSessionParticipantsKey, sessionParticipants(participants))
	session.SetData(ratingSessionDateKey, normalizeRatingDate(ratingDate))

	msg := tgbotapi.NewMessage(chatID, buildDailyRatingsPrompt(participants, ratingDate))
	return &msg, true, nil
}

// StartDailyRatingsSession prepares the admin prompt and stores the participant order in session state.
func (h *Handlers) StartDailyRatingsSession(chatID, userID int64, ratingDate time.Time) (tgbotapi.MessageConfig, error) {
	msg, ok, err := h.PrepareDailyRatingsReminder(chatID, userID, ratingDate)
	if err != nil {
		return tgbotapi.NewMessage(chatID, genericErrorMessage), nil
	}
	if !ok {
		return tgbotapi.NewMessage(chatID, "No active participants are available for rating right now."), nil
	}
	return *msg, nil
}

// HandleDailyRatingsInteractive validates and saves a daily participant rating reply.
func (h *Handlers) HandleDailyRatingsInteractive(m *tgbotapi.Message) (tgbotapi.Chattable, error) {
	session, exists := h.SessionManager.GetSession(m.Chat.ID)
	if !exists || session.Type != SessionTypeDailyRatings {
		return nil, nil
	}

	if session.UserID != m.From.ID {
		return nil, nil
	}

	isAdmin, err := h.checkAdmin(m.From.ID)
	if err != nil || !isAdmin {
		return nil, nil
	}

	participants, ok := sessionParticipantsFromSession(session)
	if !ok || len(participants) == 0 {
		h.SessionManager.EndSession(m.Chat.ID)
		return tgbotapi.NewMessage(m.Chat.ID, "The rating session expired. Please start a new rating prompt."), nil
	}

	ratingDate, ok := ratingDateFromSession(session)
	if !ok {
		h.SessionManager.EndSession(m.Chat.ID)
		return tgbotapi.NewMessage(m.Chat.ID, "The rating session expired. Please start a new rating prompt."), nil
	}

	h.ratingsMu.Lock()
	defer h.ratingsMu.Unlock()

	if !ratingSubmissionWindowOpen(ratingDate, TimeNow()) {
		h.SessionManager.EndSession(m.Chat.ID)
		return tgbotapi.NewMessage(m.Chat.ID, "The rating session expired. Please start a new rating prompt."), nil
	}

	scores, parseErr := parseParticipantScores(m.Text, len(participants))
	if parseErr != nil {
		h.SessionManager.TouchSession(m.Chat.ID)
		return tgbotapi.NewMessage(m.Chat.ID, formatRatingValidationError(parseErr.Error(), participants, ratingDate)), nil
	}

	ratings := make([]*store.ParticipantDailyRating, 0, len(participants))
	for i, participant := range participants {
		ratings = append(ratings, &store.ParticipantDailyRating{
			ParticipantID:   participant.ID,
			ParticipantName: participant.Name,
			RatingDate:      ratingDate,
			Score:           scores[i],
		})
	}

	if err := h.Store.SaveDailyParticipantRatings(context.Background(), ratingDate, ratings); err != nil {
		return tgbotapi.NewMessage(m.Chat.ID, genericErrorMessage), nil
	}

	if h.Bot != nil && h.GroupID != 0 {
		normalizedNow := normalizeRatingDate(ratingDate)
		totals, err := h.Store.GetMonthlyParticipantTotals(context.Background(), normalizedNow.Year(), normalizedNow.Month())
		if err == nil {
			msgText := formatDailyAndMonthlySummary(ratings, totals, ratingDate)
			msg := tgbotapi.NewMessage(h.GroupID, msgText)
			msg.ParseMode = tgbotapi.ModeHTML
			if _, sendErr := h.Bot.Send(msg); sendErr != nil {
				log.Printf("error sending daily ratings group notification: %v", sendErr)
			}
		} else {
			log.Printf("error fetching monthly participant totals for group notification: %v", err)
		}
	}

	h.SessionManager.TouchSession(m.Chat.ID)
	return tgbotapi.NewMessage(m.Chat.ID, formatRatingSuccess(participants, ratingDate)), nil
}

// HandleRatingsCalendar renders the current month's participant ratings for admins.
func (h *Handlers) HandleRatingsCalendar(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	isAdmin, err := h.checkAdmin(m.From.ID)
	if err != nil || !isAdmin {
		return tgbotapi.NewMessage(m.Chat.ID, adminOnlyMessage), nil
	}

	now := normalizeRatingDate(TimeNow())
	participants, err := h.Store.GetParticipantsForRating(context.Background())
	if err != nil {
		return tgbotapi.NewMessage(m.Chat.ID, genericErrorMessage), nil
	}
	participants = h.filterRatingParticipants(participants)

	ratings, err := h.Store.GetCurrentMonthParticipantRatings(context.Background(), now)
	if err != nil {
		return tgbotapi.NewMessage(m.Chat.ID, genericErrorMessage), nil
	}
	participants = mergeCalendarParticipants(participants, ratings)
	if len(participants) == 0 {
		return tgbotapi.NewMessage(m.Chat.ID, "No active participants are available for rating right now."), nil
	}

	text := formatRatingsCalendar(participants, ratings, now)
	msg := tgbotapi.NewMessage(m.Chat.ID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	return msg, nil
}

func buildDailyRatingsPrompt(participants []*store.User, ratingDate time.Time) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Daily participant ratings for %s\n\n", normalizeRatingDate(ratingDate).Format("2006-01-02")))
	b.WriteString(formatParticipantOrder(sessionParticipants(participants)))
	b.WriteString("\n\nReply with ")
	b.WriteString(strconv.Itoa(len(participants)))
	b.WriteString(" score(s) in this order, separated by spaces.\nExample: ")

	for i := range participants {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString("5")
	}

	return b.String()
}

func parseParticipantScores(text string, expected int) ([]int, error) {
	fields := strings.Fields(text)
	if len(fields) != expected {
		return nil, fmt.Errorf("expected %d score(s), received %d", expected, len(fields))
	}

	scores := make([]int, 0, len(fields))
	for _, field := range fields {
		score, err := strconv.Atoi(field)
		if err != nil {
			return nil, fmt.Errorf("all scores must be integers between 1 and 5")
		}
		if score < 1 || score > 5 {
			return nil, fmt.Errorf("scores must be between 1 and 5")
		}
		scores = append(scores, score)
	}

	return scores, nil
}

func formatRatingValidationError(reason string, participants []ratingSessionParticipant, ratingDate time.Time) string {
	return fmt.Sprintf(
		"Could not save ratings for %s: %s\n\nParticipant order:\n%s\n\nReply with %d score(s) in that order, separated by spaces.",
		normalizeRatingDate(ratingDate).Format("2006-01-02"),
		reason,
		formatParticipantOrder(participants),
		len(participants),
	)
}

func formatRatingSuccess(participants []ratingSessionParticipant, ratingDate time.Time) string {
	return fmt.Sprintf(
		"Saved ratings for %s.\n\nParticipant order:\n%s\n\nSend another set of scores in the same order to overwrite today's ratings.",
		normalizeRatingDate(ratingDate).Format("2006-01-02"),
		formatParticipantOrder(participants),
	)
}

func formatParticipantOrder(participants []ratingSessionParticipant) string {
	lines := make([]string, 0, len(participants))
	for i, participant := range participants {
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, participant.Name))
	}
	return strings.Join(lines, "\n")
}

// BuildMonthlyRatingsWinnersAnnouncement renders the month-end digest on the last calendar day only.
func (h *Handlers) BuildMonthlyRatingsWinnersAnnouncement(now time.Time) (*tgbotapi.MessageConfig, bool, error) {
	normalizedNow := normalizeRatingDate(now)
	if !isLastDayOfMonth(normalizedNow) {
		return nil, false, nil
	}

	h.ratingsMu.Lock()
	defer h.ratingsMu.Unlock()

	totals, err := h.Store.GetMonthlyParticipantTotals(context.Background(), normalizedNow.Year(), normalizedNow.Month())
	if err != nil {
		return nil, false, err
	}

	msg := tgbotapi.NewMessage(h.GroupID, formatMonthlyRatingsWinnersDigest(totals, normalizedNow))
	return &msg, true, nil
}

func (h *Handlers) filterRatingParticipants(users []*store.User) []*store.User {
	filtered := make([]*store.User, 0, len(users))
	for _, user := range users {
		if user == nil {
			continue
		}
		if user.IsAdmin {
			continue
		}
		if h.AdminID != 0 && user.TelegramUserID == h.AdminID {
			continue
		}
		filtered = append(filtered, user)
	}
	return filtered
}

func sessionParticipants(users []*store.User) []ratingSessionParticipant {
	participants := make([]ratingSessionParticipant, 0, len(users))
	for _, user := range users {
		if user == nil {
			continue
		}
		participants = append(participants, ratingSessionParticipant{
			ID:   user.ID,
			Name: user.FirstName,
		})
	}
	return participants
}

func mergeCalendarParticipants(participants []*store.User, ratings []*store.ParticipantDailyRating) []*store.User {
	merged := make([]*store.User, 0, len(participants))
	seen := make(map[int64]struct{}, len(participants))
	for _, participant := range participants {
		if participant == nil {
			continue
		}
		merged = append(merged, participant)
		seen[participant.ID] = struct{}{}
	}

	var ratedOnly []*store.User
	for _, rating := range ratings {
		if rating == nil || rating.ParticipantID == 0 {
			continue
		}
		if _, ok := seen[rating.ParticipantID]; ok {
			continue
		}
		seen[rating.ParticipantID] = struct{}{}
		ratedOnly = append(ratedOnly, &store.User{
			ID:        rating.ParticipantID,
			FirstName: rating.ParticipantName,
		})
	}

	merged = append(merged, ratedOnly...)
	slices.SortFunc(merged, func(a, b *store.User) int {
		if cmp := strings.Compare(strings.ToLower(a.FirstName), strings.ToLower(b.FirstName)); cmp != 0 {
			return cmp
		}
		switch {
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		default:
			return 0
		}
	})

	return merged
}

func sessionParticipantsFromSession(session *Session) ([]ratingSessionParticipant, bool) {
	raw, ok := session.GetData(ratingSessionParticipantsKey)
	if !ok {
		return nil, false
	}

	participants, ok := raw.([]ratingSessionParticipant)
	return participants, ok
}

func ratingDateFromSession(session *Session) (time.Time, bool) {
	raw, ok := session.GetData(ratingSessionDateKey)
	if !ok {
		return time.Time{}, false
	}

	date, ok := raw.(time.Time)
	if !ok {
		return time.Time{}, false
	}

	return normalizeRatingDate(date), true
}

func normalizeRatingDate(date time.Time) time.Time {
	loc, err := time.LoadLocation(ratingTimezoneName)
	if err != nil {
		loc = time.UTC
	}

	localDate := date.In(loc)
	return time.Date(localDate.Year(), localDate.Month(), localDate.Day(), 0, 0, 0, 0, time.UTC)
}

func isLastDayOfMonth(date time.Time) bool {
	normalized := normalizeRatingDate(date)
	return normalized.AddDate(0, 0, 1).Month() != normalized.Month()
}

func ratingSubmissionWindowOpen(ratingDate time.Time, now time.Time) bool {
	normalizedDate := normalizeRatingDate(ratingDate)
	if !normalizeRatingDate(now).Equal(normalizedDate) {
		return false
	}
	if !isLastDayOfMonth(normalizedDate) {
		return true
	}

	loc, err := time.LoadLocation(ratingTimezoneName)
	if err != nil {
		loc = time.UTC
	}
	localNow := now.In(loc)
	cutoff := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 21, 0, 0, 0, loc)
	return localNow.Before(cutoff)
}

func formatRatingsCalendar(participants []*store.User, ratings []*store.ParticipantDailyRating, now time.Time) string {
	normalizedNow := normalizeRatingDate(now)
	sessionOrder := sessionParticipants(participants)
	table := buildRatingsCalendarTable(sessionOrder, ratings, normalizedNow)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Participant ratings for %s\n", normalizedNow.Format("January 2006")))
	b.WriteString(fmt.Sprintf("Showing %s through %s.\n\n", time.Date(normalizedNow.Year(), normalizedNow.Month(), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02"), normalizedNow.Format("2006-01-02")))
	b.WriteString("<pre>")
	b.WriteString(html.EscapeString(table))
	b.WriteString("</pre>\n")
	b.WriteString("Missing scores are shown as -.")
	return b.String()
}

func buildRatingsCalendarTable(participants []ratingSessionParticipant, ratings []*store.ParticipantDailyRating, now time.Time) string {
	const missingScore = "-"

	dateWidth := len("2006-01-02")
	nameWidths := make([]int, len(participants))
	for i, participant := range participants {
		if len(participant.Name) > len(missingScore) {
			nameWidths[i] = len(participant.Name)
		} else {
			nameWidths[i] = len(missingScore)
		}
	}

	ratingByDayAndParticipant := make(map[string]map[int64]int, len(ratings))
	for _, rating := range ratings {
		if rating == nil {
			continue
		}
		dateKey := normalizeRatingDate(rating.RatingDate).Format("2006-01-02")
		if _, ok := ratingByDayAndParticipant[dateKey]; !ok {
			ratingByDayAndParticipant[dateKey] = make(map[int64]int)
		}
		ratingByDayAndParticipant[dateKey][rating.ParticipantID] = rating.Score
	}

	formatCell := func(value string, width int) string {
		return fmt.Sprintf("%-*s", width, value)
	}

	var lines []string
	headerCells := []string{formatCell("Date", dateWidth)}
	for i, participant := range participants {
		headerCells = append(headerCells, formatCell(participant.Name, nameWidths[i]))
	}
	lines = append(lines, strings.Join(headerCells, "  "))

	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	for day := monthStart; !day.After(now); day = day.AddDate(0, 0, 1) {
		dateKey := day.Format("2006-01-02")
		row := []string{formatCell(dateKey, dateWidth)}
		dailyRatings := ratingByDayAndParticipant[dateKey]
		for i, participant := range participants {
			cell := missingScore
			if score, ok := dailyRatings[participant.ID]; ok {
				cell = strconv.Itoa(score)
			}
			row = append(row, formatCell(cell, nameWidths[i]))
		}
		lines = append(lines, strings.Join(row, "  "))
	}

	return strings.Join(lines, "\n")
}

func formatDailyAndMonthlySummary(dailyRatings []*store.ParticipantDailyRating, totals []*store.ParticipantMonthlyTotal, now time.Time) string {
	normalizedNow := normalizeRatingDate(now)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("<b>Daily Ratings for %s</b>\n", normalizedNow.Format("2006-01-02")))

	for _, rating := range dailyRatings {
		b.WriteString(fmt.Sprintf("%s: %d\n", html.EscapeString(rating.ParticipantName), rating.Score))
	}

	b.WriteString(fmt.Sprintf("\n<b>Monthly Standings (%s)</b>\n", normalizedNow.Format("January 2006")))
	if len(totals) == 0 {
		b.WriteString("No participant ratings were recorded this month.")
	} else {
		for i, total := range totals {
			b.WriteString(fmt.Sprintf("%d. %s - %d point(s)", i+1, html.EscapeString(total.ParticipantName), total.TotalScore))
			if i < len(totals)-1 {
				b.WriteString("\n")
			}
		}
	}

	return b.String()
}

func formatMonthlyRatingsWinnersDigest(totals []*store.ParticipantMonthlyTotal, now time.Time) string {
	normalizedNow := normalizeRatingDate(now)

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Monthly participant ratings for %s\n\n", normalizedNow.Format("January 2006")))

	if len(totals) == 0 {
		b.WriteString("No participant ratings were recorded this month.")
		return b.String()
	}

	b.WriteString("Winners:\n")
	places := []string{"1st", "2nd", "3rd"}
	for i := 0; i < len(totals) && i < len(places); i++ {
		b.WriteString(fmt.Sprintf("%s: %s with %d point(s)\n", places[i], totals[i].ParticipantName, totals[i].TotalScore))
	}

	b.WriteString("\nTotals:\n")
	for i, total := range totals {
		b.WriteString(fmt.Sprintf("%d. %s - %d point(s)", i+1, total.ParticipantName, total.TotalScore))
		if total.DaysRated > 0 {
			b.WriteString(fmt.Sprintf(" across %d rated day(s)", total.DaysRated))
		}
		if i < len(totals)-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}
