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
	volunteerMessage      = "Please select a date to volunteer for duty."
	volunteerSuccessMessage = "Thank you for volunteering for duty on %s!"
	volunteerFailureMessage = "Sorry, we couldn't process your request to volunteer for duty on %s. It might already be taken."
)

// HandleVolunteer initiates the process for a user to volunteer for a duty.
func (h *Handlers) HandleVolunteer(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	msg := tgbotapi.NewMessage(m.Chat.ID, volunteerMessage)
	msg.ReplyMarkup = keyboard.Calendar(time.Now())
	return msg, nil
}

// HandleVolunteerCallback handles the selection of a date from the volunteer calendar.
func (h *Handlers) HandleVolunteerCallback(q *tgbotapi.CallbackQuery) (tgbotapi.EditMessageTextConfig, error) {
	// e.g., "select_day:2023-05-20"
	parts := strings.Split(q.Data, ":")
	dateStr := parts[1]

	// Get the user who clicked the button
	user, err := h.Store.GetUserByTelegramID(context.Background(), q.From.ID)
	if err != nil {
		// Handle case where user is not in the database
		// For now, we'll just return an error message
		text := fmt.Sprintf("Could not find user with Telegram ID %d", q.From.ID)
		edit := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, text)
		return edit, nil
	}

	// Call the scheduler to volunteer the user for the selected date
	err = h.Scheduler.VolunteerForDuty(context.Background(), user, dateStr)

	var text string
	if err != nil {
		text = fmt.Sprintf(volunteerFailureMessage, dateStr)
	} else {
		text = fmt.Sprintf(volunteerSuccessMessage, dateStr)
	}

	// Create a new message configuration to replace the calendar
	edit := tgbotapi.NewEditMessageText(
		q.Message.Chat.ID,
		q.Message.MessageID,
		text,
	)
	// Remove the inline keyboard by not setting ReplyMarkup
	return edit, nil
}