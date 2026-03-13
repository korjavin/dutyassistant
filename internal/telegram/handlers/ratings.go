package handlers

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/store"
)

const (
	ratingSessionParticipantsKey = "participants"
	ratingSessionDateKey         = "rating_date"
)

type ratingSessionParticipant struct {
	ID   int64
	Name string
}

// StartDailyRatingsSession prepares the admin prompt and stores the participant order in session state.
func (h *Handlers) StartDailyRatingsSession(chatID, userID int64, ratingDate time.Time) (tgbotapi.MessageConfig, error) {
	isAdmin, err := h.checkAdmin(userID)
	if err != nil || !isAdmin {
		return tgbotapi.NewMessage(chatID, adminOnlyMessage), nil
	}

	participants, err := h.Store.GetParticipantsForRating(context.Background())
	if err != nil {
		return tgbotapi.NewMessage(chatID, genericErrorMessage), nil
	}
	if len(participants) == 0 {
		return tgbotapi.NewMessage(chatID, "No active participants are available for rating right now."), nil
	}

	h.SessionManager.StartSession(chatID, userID, SessionTypeDailyRatings)
	session, _ := h.SessionManager.GetSession(chatID)
	session.SetData(ratingSessionParticipantsKey, sessionParticipants(participants))
	session.SetData(ratingSessionDateKey, normalizeRatingDate(ratingDate))

	return tgbotapi.NewMessage(chatID, buildDailyRatingsPrompt(participants, ratingDate)), nil
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

	h.SessionManager.TouchSession(m.Chat.ID)
	return tgbotapi.NewMessage(m.Chat.ID, formatRatingSuccess(participants, ratingDate)), nil
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
	return time.Date(date.UTC().Year(), date.UTC().Month(), date.UTC().Day(), 0, 0, 0, 0, time.UTC)
}
