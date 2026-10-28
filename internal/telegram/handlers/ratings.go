package handlers

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/store"
)

const ratingTimezoneName = "Europe/Berlin"

// ratingSessionParticipant is a participant's column in the ratings calendar.
type ratingSessionParticipant struct {
	ID   int64
	Name string
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

	if len(totals) > 0 {
		winner := totals[0]
		// Check if we've already granted a sviniya for this month
		// Grant sviniya to winner atomically (checks, records, and adds balance in one transaction)
		err := h.Store.GrantSviniyaForMonth(context.Background(), normalizedNow.Year(), normalizedNow.Month(), winner.ParticipantID)
		if err != nil {
			if errors.Is(err, store.ErrSviniyaAlreadyGranted) {
				// Already granted for this month - log and continue
				slog.Info(fmt.Sprintf("Sviniya already granted for %s %d, skipping duplicate grant",
					normalizedNow.Month(), normalizedNow.Year()))
			} else {
				slog.Error("error granting sviniya to monthly rating winner", "winner", winner.ParticipantName, "err", err)
				// Include a note in the announcement that sviniya grant failed
				msg := tgbotapi.NewMessage(h.GroupID, formatMonthlyRatingsWinnersDigest(totals, normalizedNow, true))
				return &msg, true, nil
			}
		} else {
			slog.Info(fmt.Sprintf("Granted 1 sviniya to %s for %s %d",
				winner.ParticipantName, normalizedNow.Month(), normalizedNow.Year()))
		}
	}

	msg := tgbotapi.NewMessage(h.GroupID, formatMonthlyRatingsWinnersDigest(totals, normalizedNow, false))
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

func formatRatingsCalendar(participants []*store.User, ratings []*store.ParticipantDailyRating, now time.Time) string {
	normalizedNow := normalizeRatingDate(now)
	sessionOrder := sessionParticipants(participants)
	table := buildRatingsCalendarTable(sessionOrder, ratings, normalizedNow)

	var b strings.Builder
	fmt.Fprintf(&b, "Participant ratings for %s\n", normalizedNow.Format("January 2006"))
	fmt.Fprintf(&b, "Showing %s through %s.\n\n", time.Date(normalizedNow.Year(), normalizedNow.Month(), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02"), normalizedNow.Format("2006-01-02"))
	b.WriteString("<pre>")
	b.WriteString(html.EscapeString(table))
	b.WriteString("</pre>\n")
	b.WriteString("Missing scores are shown as -.")
	return b.String()
}

type calendarCell struct {
	Score  int
	HasEar bool
}

func buildRatingsCalendarTable(participants []ratingSessionParticipant, ratings []*store.ParticipantDailyRating, now time.Time) string {
	const missingScore = "-"
	const earScore = "5e"

	dateWidth := len("2006-01-02")
	nameWidths := make([]int, len(participants))
	for i, participant := range participants {
		nameWidths[i] = max(len(participant.Name), len(earScore))
	}

	ratingByDayAndParticipant := make(map[string]map[int64]calendarCell, len(ratings))
	for _, rating := range ratings {
		if rating == nil {
			continue
		}
		dateKey := normalizeRatingDate(rating.RatingDate).Format("2006-01-02")
		if _, ok := ratingByDayAndParticipant[dateKey]; !ok {
			ratingByDayAndParticipant[dateKey] = make(map[int64]calendarCell)
		}
		ratingByDayAndParticipant[dateKey][rating.ParticipantID] = calendarCell{Score: rating.Score, HasEar: rating.HasEar}
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
			if c, ok := dailyRatings[participant.ID]; ok {
				cell = strconv.Itoa(c.Score)
				if c.HasEar {
					cell += "e"
				}
			}
			row = append(row, formatCell(cell, nameWidths[i]))
		}
		lines = append(lines, strings.Join(row, "  "))
	}

	return strings.Join(lines, "\n")
}

// formatDailyAndMonthlySummary renders the group message with the day's scores
// (each followed by the bot's reason, if any, and its own score when the
// admin gave another one) and the month's standings.
func formatDailyAndMonthlySummary(dailyRatings []*store.ParticipantDailyRating, totals []*store.ParticipantMonthlyTotal, now time.Time, reasons map[int64]ratingReason) string {
	normalizedNow := normalizeRatingDate(now)

	var b strings.Builder
	fmt.Fprintf(&b, "<b>Daily Ratings for %s</b>\n", normalizedNow.Format("2006-01-02"))

	for _, rating := range dailyRatings {
		scoreStr := strconv.Itoa(rating.Score)
		if rating.HasEar {
			scoreStr += "e"
		}
		fmt.Fprintf(&b, "%s: %s", html.EscapeString(rating.ParticipantName), scoreStr)
		if reason := reasons[rating.ParticipantID]; reason.Text != "" {
			b.WriteString(" — ")
			if reason.Suggested > 0 {
				fmt.Fprintf(&b, "🤖 bot suggested %d: ", reason.Suggested)
			}
			fmt.Fprintf(&b, "<i>%s</i>", html.EscapeString(reason.Text))
		}
		b.WriteString("\n")
	}

	fmt.Fprintf(&b, "\n<b>Monthly Standings (%s)</b>\n", normalizedNow.Format("January 2006"))
	if len(totals) == 0 {
		b.WriteString("No participant ratings were recorded this month.")
	} else {
		for i, total := range totals {
			fmt.Fprintf(&b, "%d. %s - %d point(s)", i+1, html.EscapeString(total.ParticipantName), total.TotalScore)
			if total.EarCount > 0 {
				fmt.Fprintf(&b, ", %d ear(s)", total.EarCount)
			}
			if i < len(totals)-1 {
				b.WriteString("\n")
			}
		}
	}

	return b.String()
}

func formatMonthlyRatingsWinnersDigest(totals []*store.ParticipantMonthlyTotal, now time.Time, sviniyaGrantFailed bool) string {
	normalizedNow := normalizeRatingDate(now)

	var b strings.Builder
	fmt.Fprintf(&b, "Monthly participant ratings for %s\n\n", normalizedNow.Format("January 2006"))

	if len(totals) == 0 {
		b.WriteString("No participant ratings were recorded this month.")
		return b.String()
	}

	b.WriteString("Winners:\n")
	places := []string{"1st", "2nd", "3rd"}
	for i := 0; i < len(totals) && i < len(places); i++ {
		escapedName := html.EscapeString(totals[i].ParticipantName)
		fmt.Fprintf(&b, "%s: %s with %d point(s)", places[i], escapedName, totals[i].TotalScore)
		if totals[i].EarCount > 0 {
			fmt.Fprintf(&b, ", %d ear(s)", totals[i].EarCount)
		}
		b.WriteString("\n")
	}

	b.WriteString("\nTotals:\n")
	for i, total := range totals {
		escapedName := html.EscapeString(total.ParticipantName)
		fmt.Fprintf(&b, "%d. %s - %d point(s)", i+1, escapedName, total.TotalScore)
		if total.EarCount > 0 {
			fmt.Fprintf(&b, ", %d ear(s)", total.EarCount)
		}
		if total.DaysRated > 0 {
			fmt.Fprintf(&b, " across %d rated day(s)", total.DaysRated)
		}
		if i < len(totals)-1 {
			b.WriteString("\n")
		}
	}

	if sviniyaGrantFailed && len(totals) > 0 {
		b.WriteString("\n\nNote: Sviniya grant to the winner failed. Please check with an admin.")
	}

	return b.String()
}
