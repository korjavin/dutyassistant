package handlers

import (
	"fmt"
	"strings"
	"time"

	"github.com/korjavin/dutyassistant/internal/telegram/keyboard"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	scheduleMessage = "Here is the duty schedule for %s."
)

// HandleSchedule handles the /schedule command, displaying a calendar.
func (h *Handlers) HandleSchedule(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	now := time.Now()
	text := fmt.Sprintf(scheduleMessage, now.Format("January 2006"))

	msg := tgbotapi.NewMessage(m.Chat.ID, text)
	msg.ReplyMarkup = keyboard.Calendar(now)
	return msg, nil
}

// HandleCalendarCallback handles callbacks from the schedule calendar.
func (h *Handlers) HandleCalendarCallback(q *tgbotapi.CallbackQuery) (tgbotapi.EditMessageTextConfig, error) {
	// e.g., "prev_month:2023-05"
	parts := strings.Split(q.Data, ":")
	action := parts[0]
	dateStr := parts[1]

	var t time.Time
	var err error

	// The date string for prev/next month is "YYYY-MM"
	if action == keyboard.ActionPrevMonth || action == keyboard.ActionNextMonth {
		t, err = time.Parse("2006-01", dateStr)
	} else {
		return tgbotapi.EditMessageTextConfig{}, fmt.Errorf("unknown action: %s", action)
	}
	if err != nil {
		return tgbotapi.EditMessageTextConfig{}, fmt.Errorf("failed to parse date: %w", err)
	}

	var newTime time.Time
	if action == keyboard.ActionPrevMonth {
		newTime = t.AddDate(0, -1, 0)
	} else {
		newTime = t.AddDate(0, 1, 0)
	}

	text := fmt.Sprintf(scheduleMessage, newTime.Format("January 2006"))
	newMarkup := keyboard.Calendar(newTime)

	edit := tgbotapi.NewEditMessageText(
		q.Message.Chat.ID,
		q.Message.MessageID,
		text,
	)
	edit.ReplyMarkup = &newMarkup
	return edit, nil
}