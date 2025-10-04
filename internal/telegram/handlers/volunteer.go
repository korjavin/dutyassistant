package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/korjavin/dutyassistant/internal/telegram/keyboard"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	volunteerMessage             = "Please select a date to volunteer for duty."
	volunteerSuccessMessage      = "Thank you for volunteering for duty on %s!"
	volunteerFailureMessage      = "Sorry, we couldn't process your request to volunteer for duty on %s. It might already be taken or an error occurred."
	volunteerUserNotFoundMessage = "Could not find your user profile. Please use /start first."
)

// HandleVolunteer initiates the process for a user to volunteer for a duty.
func (h *Handlers) HandleVolunteer(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	msg := tgbotapi.NewMessage(m.Chat.ID, volunteerMessage)
	// We pass nil for duties because the volunteer calendar doesn't need to show existing duties.
	msg.ReplyMarkup = keyboard.Calendar(time.Now(), nil)
	return msg, nil
}

// HandleVolunteerCallback handles the selection of a date from the volunteer calendar.
func (h *Handlers) HandleVolunteerCallback(q *tgbotapi.CallbackQuery) (tgbotapi.EditMessageTextConfig, error) {
	parts := strings.Split(q.Data, ":")
	if len(parts) != 2 {
		return tgbotapi.EditMessageTextConfig{}, fmt.Errorf("invalid callback data format: %s", q.Data)
	}
	dateStr := parts[1]

	dutyDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return tgbotapi.EditMessageTextConfig{}, fmt.Errorf("failed to parse date from volunteer callback: %w", err)
	}

	user, err := h.Store.GetUserByTelegramID(context.Background(), q.From.ID)
	if err != nil || user == nil {
		text := volunteerUserNotFoundMessage
		return tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, text), nil
	}

	// Call the scheduler to volunteer the user for the selected date
	err = h.Scheduler.VolunteerForDuty(context.Background(), user, dutyDate)

	var text string
	if err != nil {
		text = fmt.Sprintf(volunteerFailureMessage, dateStr)
	} else {
		text = fmt.Sprintf(volunteerSuccessMessage, dateStr)
	}

	edit := tgbotapi.NewEditMessageText(
		q.Message.Chat.ID,
		q.Message.MessageID,
		text,
	)
	// Remove the inline keyboard after selection
	edit.ReplyMarkup = nil
	return edit, nil
}