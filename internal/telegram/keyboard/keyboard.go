package keyboard

import (
	"fmt"
	"strings"
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
// It marks days with duties with emoji indicators and builds a user legend.
func Calendar(t time.Time, duties []*store.Duty) tgbotapi.InlineKeyboardMarkup {
	dutyMap := make(map[int]*store.Duty)
	userAssignments := make(map[int64]map[store.AssignmentType]bool) // Track user->assignment types

	for _, duty := range duties {
		dutyMap[duty.DutyDate.Day()] = duty

		// Track which assignment types each user has
		if userAssignments[duty.UserID] == nil {
			userAssignments[duty.UserID] = make(map[store.AssignmentType]bool)
		}
		userAssignments[duty.UserID][duty.AssignmentType] = true
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

				// Format: day number + emoji (compact for Telegram button width limits)
				var dayText string
				isToday := date.Year() == today.Year() && date.Month() == today.Month() && date.Day() == today.Day()

				if duty, ok := dutyMap[day]; ok {
					// Show day number and emoji only
					emoji := ""
					switch duty.AssignmentType {
					case store.AssignmentTypeVoluntary:
						emoji = "🟢"
					case store.AssignmentTypeAdmin:
						emoji = "🔵"
					case store.AssignmentTypeRoundRobin:
						emoji = "⚪"
					}

					if isToday {
						dayText = fmt.Sprintf("·%d%s", day, emoji)
					} else {
						dayText = fmt.Sprintf("%d%s", day, emoji)
					}
				} else {
					// No duty - show day number, mark today with dot prefix
					if isToday {
						dayText = fmt.Sprintf("·%d", day)
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

	// Build user legend showing who has duties and with which assignment types
	var userLegend []string
	usersSeen := make(map[int64]bool)

	for _, duty := range duties {
		if usersSeen[duty.UserID] {
			continue
		}
		usersSeen[duty.UserID] = true

		// Collect all emojis for this user
		var emojis []string
		if userAssignments[duty.UserID][store.AssignmentTypeVoluntary] {
			emojis = append(emojis, "🟢")
		}
		if userAssignments[duty.UserID][store.AssignmentTypeAdmin] {
			emojis = append(emojis, "🔵")
		}
		if userAssignments[duty.UserID][store.AssignmentTypeRoundRobin] {
			emojis = append(emojis, "⚪")
		}

		// Build legend entry: "🟢🔵Name" or "🟢Name"
		legendEntry := fmt.Sprintf("%s%s", strings.Join(emojis, ""), duty.User.FirstName)
		userLegend = append(userLegend, legendEntry)
	}

	// Add legend type explanation
	legendType := tgbotapi.NewInlineKeyboardButtonData("🟢=Volunteer 🔵=Admin ⚪=Auto", ActionIgnore)
	keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{legendType})

	// Add user legend rows (2 users per row to fit)
	if len(userLegend) > 0 {
		for i := 0; i < len(userLegend); i += 2 {
			var row []tgbotapi.InlineKeyboardButton
			row = append(row, tgbotapi.NewInlineKeyboardButtonData(userLegend[i], ActionIgnore))
			if i+1 < len(userLegend) {
				row = append(row, tgbotapi.NewInlineKeyboardButtonData(userLegend[i+1], ActionIgnore))
			}
			keyboard = append(keyboard, row)
		}
	}

	return tgbotapi.NewInlineKeyboardMarkup(keyboard...)
}