package keyboard

import (
	"fmt"
	"time"

	"github.com/korjavin/dutyassistant/internal/store"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	ActionPrevMonth = "prev_month"
	ActionNextMonth = "next_month"
	ActionSelectDay = "select_day"
	ActionIgnore    = "ignore"
)

// Calendar creates an inline keyboard markup for a given month and year.
// It marks days with duties with emoji indicators and first 3 letters of name.
func Calendar(t time.Time, duties []*store.Duty) tgbotapi.InlineKeyboardMarkup {
	dutyMap := make(map[int]*store.Duty)
	for _, duty := range duties {
		dutyMap[duty.DutyDate.Day()] = duty
	}

	year, month, _ := t.Date()

	// Header: << Month Year >>
	header := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("«", fmt.Sprintf("%s:%s", ActionPrevMonth, t.Format("2006-01-02"))),
		tgbotapi.NewInlineKeyboardButtonData(t.Format("Jan 2006"), ActionIgnore),
		tgbotapi.NewInlineKeyboardButtonData("»", fmt.Sprintf("%s:%s", ActionNextMonth, t.Format("2006-01-02"))),
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

	firstDay := time.Date(year, month, 1, 0, 0, 0, 0, t.Location())
	lastDay := firstDay.AddDate(0, 1, -1)

	offset := int(firstDay.Weekday())
	if offset == 0 { // Sunday
		offset = 6
	} else {
		offset-- // Monday is 0
	}

	// Get today for marking
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	row := make([]tgbotapi.InlineKeyboardButton, 7)
	day := 1
	for day <= lastDay.Day() {
		for i := 0; i < 7; i++ {
			if (len(keyboard) == 2 && i < offset) || day > lastDay.Day() {
				row[i] = tgbotapi.NewInlineKeyboardButtonData(" ", ActionIgnore)
			} else {
				date := time.Date(year, month, day, 0, 0, 0, 0, t.Location())

				// Format: just emoji + 3 letters, or day number with marker for today
				var dayText string
				if duty, ok := dutyMap[day]; ok {
					// Show emoji and first 3 letters
					emoji := ""
					switch duty.AssignmentType {
					case store.AssignmentTypeVoluntary:
						emoji = "🟢"
					case store.AssignmentTypeAdmin:
						emoji = "🔵"
					case store.AssignmentTypeRoundRobin:
						emoji = "⚪"
					}
					shortName := duty.User.FirstName
					if len(shortName) > 3 {
						shortName = shortName[:3]
					}

					// Mark today with [brackets]
					if date.Year() == today.Year() && date.Month() == today.Month() && date.Day() == today.Day() {
						dayText = fmt.Sprintf("[%s%s]", emoji, shortName)
					} else {
						dayText = fmt.Sprintf("%s%s", emoji, shortName)
					}
				} else {
					// No duty - show day number, mark today
					if date.Year() == today.Year() && date.Month() == today.Month() && date.Day() == today.Day() {
						dayText = fmt.Sprintf("[%d]", day)
					} else {
						dayText = fmt.Sprintf("%d", day)
					}
				}

				row[i] = tgbotapi.NewInlineKeyboardButtonData(
					dayText,
					fmt.Sprintf("%s:%s", ActionSelectDay, date.Format("2006-01-02")),
				)
				day++
			}
		}
		keyboard = append(keyboard, row)
		row = make([]tgbotapi.InlineKeyboardButton, 7)
	}

	// Add legend footer
	legend := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("🟢=Volunteer 🔵=Admin ⚪=Auto", ActionIgnore),
	}
	keyboard = append(keyboard, legend)

	return tgbotapi.NewInlineKeyboardMarkup(keyboard...)
}