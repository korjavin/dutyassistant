package handlers

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/korjavin/dutyassistant/internal/telegram/keyboard"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	scheduleMessage = "Duty schedule for %s"
)

// HandleSchedule handles the /schedule command, displaying a calendar with duty information.
func (h *Handlers) HandleSchedule(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	now := time.Now()

	duties, err := h.Store.GetDutiesByMonth(context.Background(), now.Year(), now.Month())
	if err != nil {
		return tgbotapi.MessageConfig{}, fmt.Errorf("could not get duties for schedule: %w", err)
	}

	text := fmt.Sprintf(scheduleMessage, now.Format("January 2006"))
	markup := keyboard.Calendar(now, duties)

	msg := tgbotapi.NewMessage(m.Chat.ID, text)
	msg.ReplyMarkup = markup
	return msg, nil
}

// HandleCalendarCallback handles callbacks for month navigation in the schedule view.
func (h *Handlers) HandleCalendarCallback(q *tgbotapi.CallbackQuery) (tgbotapi.EditMessageTextConfig, error) {
	parts := strings.Split(q.Data, ":")
	if len(parts) != 2 {
		return tgbotapi.EditMessageTextConfig{}, fmt.Errorf("invalid callback data format: %s", q.Data)
	}
	dateStr := parts[1]

	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return tgbotapi.EditMessageTextConfig{}, fmt.Errorf("failed to parse date from callback: %w", err)
	}

	var newTime time.Time
	if parts[0] == keyboard.ActionPrevMonth {
		newTime = t.AddDate(0, -1, 0)
	} else if parts[0] == keyboard.ActionNextMonth {
		newTime = t.AddDate(0, 1, 0)
	} else {
		return tgbotapi.EditMessageTextConfig{}, fmt.Errorf("unexpected action in calendar callback: %s", parts[0])
	}

	duties, err := h.Store.GetDutiesByMonth(context.Background(), newTime.Year(), newTime.Month())
	if err != nil {
		// Log the error but still show the calendar
		log.Printf("Could not get duties for schedule refresh: %v", err)
		duties = []*store.Duty{} // Send empty slice to render an empty calendar
	}

	text := fmt.Sprintf(scheduleMessage, newTime.Format("January 2006"))
	newMarkup := keyboard.Calendar(newTime, duties)

	edit := tgbotapi.NewEditMessageText(
		q.Message.Chat.ID,
		q.Message.MessageID,
		text,
	)
	edit.ReplyMarkup = &newMarkup
	return edit, nil
}