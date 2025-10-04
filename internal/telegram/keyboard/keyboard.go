package keyboard

import (
	"fmt"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	ActionPrevMonth = "prev_month"
	ActionNextMonth = "next_month"
	ActionSelectDay = "select_day"
	ActionIgnore    = "ignore"
)

// Calendar creates an inline keyboard markup for a given month and year.
// It includes navigation buttons for the previous and next months.
func Calendar(t time.Time) tgbotapi.InlineKeyboardMarkup {
	year, month, _ := t.Date()

	// Header: << Month Year >>
	header := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("«", fmt.Sprintf("%s:%s", ActionPrevMonth, t.Format("2006-01"))),
		tgbotapi.NewInlineKeyboardButtonData(t.Format("Jan 2006"), ActionIgnore),
		tgbotapi.NewInlineKeyboardButtonData("»", fmt.Sprintf("%s:%s", ActionNextMonth, t.Format("2006-01"))),
	}

	// Days of the week
	daysOfWeek := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("Mo", ActionIgnore),
		tgbotapi.NewInlineKeyboardButtonData("Tu", ActionIgnore),
		tgbotapi.NewInlineKeyboardButtonData("We", ActionIgnore),
		tgbotapi.NewInlineKeyboardButtonData("Th", ActionIgnore),
		tgbotapi.NewInlineKeyboardButtonData("Fr", ActionIgnore),
		tgbotapi.NewInlineKeyboardButtonData("Sa", ActionIgnore),
		tgbotapi.NewInlineKeyboardButtonData("Su", ActionIgnore),
	}

	keyboard := [][]tgbotapi.InlineKeyboardButton{header, daysOfWeek}

	// Calendar days
	firstDay := time.Date(year, month, 1, 0, 0, 0, 0, t.Location())
	lastDay := firstDay.AddDate(0, 1, -1)

	// Align the first day with the correct day of the week (Monday is 1)
	offset := int(firstDay.Weekday()) - 1
	if offset < 0 {
		offset = 6 // Sunday
	}

	row := make([]tgbotapi.InlineKeyboardButton, 7)
	day := 1
	for day <= lastDay.Day() {
		for i := 0; i < 7; i++ {
			if (len(keyboard) == 2 && i < offset) || day > lastDay.Day() {
				// Empty space before the first day or after the last day
				row[i] = tgbotapi.NewInlineKeyboardButtonData(" ", ActionIgnore)
			} else {
				date := time.Date(year, month, day, 0, 0, 0, 0, t.Location())
				row[i] = tgbotapi.NewInlineKeyboardButtonData(
					fmt.Sprintf("%d", day),
					fmt.Sprintf("%s:%s", ActionSelectDay, date.Format("2006-01-02")),
				)
				day++
			}
		}
		keyboard = append(keyboard, row)
		row = make([]tgbotapi.InlineKeyboardButton, 7)
	}

	return tgbotapi.NewInlineKeyboardMarkup(keyboard...)
}

// SimpleMenu creates a simple inline keyboard with a single row of buttons.
// The buttons are provided as a map of text to callback data.
func SimpleMenu(buttons map[string]string) tgbotapi.InlineKeyboardMarkup {
	var row []tgbotapi.InlineKeyboardButton
	for text, data := range buttons {
		row = append(row, tgbotapi.NewInlineKeyboardButtonData(text, data))
	}
	return tgbotapi.NewInlineKeyboardMarkup(row)
}