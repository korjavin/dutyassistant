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
// Assigns each user a number and shows number+emoji on calendar days.
func Calendar(t time.Time, duties []*store.Duty) tgbotapi.InlineKeyboardMarkup {
	dutyMap := make(map[int]*store.Duty)
	userAssignments := make(map[int64]map[store.AssignmentType]bool) // Track user->assignment types
	userNumbers := make(map[int64]int)                               // Assign each user a number
	userList := []*store.User{}                                      // Preserve order

	// Assign numbers to users in order they appear
	userCounter := 1
	for _, duty := range duties {
		dutyMap[duty.DutyDate.Day()] = duty

		// Track which assignment types each user has
		if userAssignments[duty.UserID] == nil {
			userAssignments[duty.UserID] = make(map[store.AssignmentType]bool)
		}
		userAssignments[duty.UserID][duty.AssignmentType] = true

		// Assign user number if not already assigned
		if userNumbers[duty.UserID] == 0 {
			userNumbers[duty.UserID] = userCounter
			userList = append(userList, duty.User)
			userCounter++
		}
	}

	// Number circles: ① ② ③ ④ ⑤ ⑥ ⑦ ⑧ ⑨
	numberCircles := []string{"①", "②", "③", "④", "⑤", "⑥", "⑦", "⑧", "⑨", "⑩"}

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
					// Show day number and user number circle
					userNum := userNumbers[duty.UserID]
					var numberCircle string
					if userNum > 0 && userNum <= len(numberCircles) {
						numberCircle = numberCircles[userNum-1]
					} else {
						numberCircle = fmt.Sprintf("%d", userNum)
					}

					if isToday {
						dayText = fmt.Sprintf("·%d%s", day, numberCircle)
					} else {
						dayText = fmt.Sprintf("%d%s", day, numberCircle)
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

	// Add legend type explanation
	legendType := tgbotapi.NewInlineKeyboardButtonData("🟢=Volunteer 🔵=Admin ⚪=Auto", ActionIgnore)
	keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{legendType})

	// Build user legend showing number -> name + emojis
	for idx, user := range userList {
		userNum := idx + 1
		var numberCircle string
		if userNum <= len(numberCircles) {
			numberCircle = numberCircles[userNum-1]
		} else {
			numberCircle = fmt.Sprintf("%d", userNum)
		}

		// Collect all emojis for this user
		var emojis []string
		if userAssignments[user.ID][store.AssignmentTypeVoluntary] {
			emojis = append(emojis, "🟢")
		}
		if userAssignments[user.ID][store.AssignmentTypeAdmin] {
			emojis = append(emojis, "🔵")
		}
		if userAssignments[user.ID][store.AssignmentTypeRoundRobin] {
			emojis = append(emojis, "⚪")
		}

		// Build legend entry: "① 🟢Name (V:2 A:1)" with queue counts
		legendEntry := fmt.Sprintf("%s %s%s", numberCircle, strings.Join(emojis, ""), user.FirstName)

		// Add queue counts if present
		var queueInfo []string
		if user.VolunteerQueueDays > 0 {
			queueInfo = append(queueInfo, fmt.Sprintf("V:%d", user.VolunteerQueueDays))
		}
		if user.AdminQueueDays > 0 {
			queueInfo = append(queueInfo, fmt.Sprintf("A:%d", user.AdminQueueDays))
		}
		if len(queueInfo) > 0 {
			legendEntry += fmt.Sprintf(" (%s)", strings.Join(queueInfo, " "))
		}

		legendButton := tgbotapi.NewInlineKeyboardButtonData(legendEntry, ActionIgnore)
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{legendButton})
	}

	return tgbotapi.NewInlineKeyboardMarkup(keyboard...)
}