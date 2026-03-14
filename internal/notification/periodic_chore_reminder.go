package notification

import (
	"context"
	"log"
	"math/rand"
	"time"

	"github.com/korjavin/dutyassistant/internal/store"
)

// BotSender represents the interface required to send messages via a Bot.
type BotSender interface {
	SendMessageHTML(chatID int64, text string) error
}

// StartPeriodicChoreReminders launches a background goroutine that occasionally
// checks for active chores and sends a DM to assigned users within a kid-friendly
// time window (11:00 to 18:00 in the given timezone).
func StartPeriodicChoreReminders(ctx context.Context, bot BotSender, s store.Store, timezone string) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		log.Printf("[PeriodicReminders] Invalid timezone %s, defaulting to UTC: %v", timezone, err)
		loc = time.UTC
	}

	log.Printf("[PeriodicReminders] Starting periodic chore reminders (Timezone: %s)", loc.String())

	for {
		nextRun := nextReminderTime(time.Now().In(loc), loc)
		waitDuration := time.Until(nextRun)

		log.Printf("[PeriodicReminders] Next run scheduled at: %v (in %v)", nextRun.Format(time.RFC3339), waitDuration)

		select {
		case <-ctx.Done():
			log.Println("[PeriodicReminders] Context done, stopping periodic chore reminders")
			return
		case <-time.After(waitDuration):
			sendChoreReminders(ctx, time.Now().In(loc), bot, s, loc)
		}
	}
}

// nextReminderTime calculates the next time to send reminders.
// It adds a random 3-6 hours to the current time.
// If the result is before 11:00, it advances to 11:00 same day.
// If the result is after 18:00, it advances to 11:00 next day + small random offset (0-30m).
func nextReminderTime(now time.Time, loc *time.Location) time.Time {
	// Random duration between 3 and 6 hours
	randHours := 3 + rand.Float64()*3
	next := now.Add(time.Duration(randHours * float64(time.Hour)))

	year, month, day := next.Date()
	hour := next.Hour()

	if hour < 11 {
		// Advance to 11:00 same day
		return time.Date(year, month, day, 11, 0, 0, 0, loc)
	}

	if hour >= 18 {
		// Advance to 11:00 next day + 0-30 min random offset
		randMinutes := time.Duration(rand.Intn(30)) * time.Minute
		nextDay := next.AddDate(0, 0, 1)
		return time.Date(nextDay.Year(), nextDay.Month(), nextDay.Day(), 11, 0, 0, 0, loc).Add(randMinutes)
	}

	return next
}

func sendChoreReminders(ctx context.Context, now time.Time, bot BotSender, s store.Store, loc *time.Location) {
	hour := now.Hour()

	// Double check we are in the 11-18 window
	if hour < 11 || hour >= 18 {
		log.Printf("[PeriodicReminders] Skipping send, current time %v is outside the 11:00-18:00 window", now)
		return
	}

	chores, err := s.GetActiveChores(ctx)
	if err != nil {
		log.Printf("[PeriodicReminders] Error retrieving active chores: %v", err)
		return
	}

	if len(chores) == 0 {
		return
	}

	userChores := make(map[int64][]*store.Chore)
	for _, chore := range chores {
		if chore.User != nil && chore.User.TelegramUserID != 0 {
			userChores[chore.User.TelegramUserID] = append(userChores[chore.User.TelegramUserID], chore)
		}
	}

	for telegramID, list := range userChores {
		msg := FormatPeriodicChoreReminder(list)
		if msg == "" {
			continue
		}

		err := bot.SendMessageHTML(telegramID, msg)
		if err != nil {
			log.Printf("[PeriodicReminders] Failed to send reminder to %d: %v", telegramID, err)
		} else {
			log.Printf("[PeriodicReminders] Sent reminder to %d for %d active chores", telegramID, len(list))
		}
	}
}
