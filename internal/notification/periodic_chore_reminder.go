package notification

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/korjavin/dutyassistant/internal/store"
)

// BotSender represents the interface required to send messages via a Bot.
type BotSender interface {
	SendMessageHTML(chatID int64, text string) error
}

// StartPeriodicChoreReminders launches a background goroutine that occasionally
// checks for active chores and sends a DM to assigned users within a kid-friendly
// time window (16:00 to 19:00 in the given timezone).
func StartPeriodicChoreReminders(ctx context.Context, bot BotSender, s store.Store, timezone string) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		slog.Error("[PeriodicReminders] Invalid timezone, defaulting to UTC", "timezone", timezone, "error", err)
		loc = time.UTC
	}

	slog.Info("[PeriodicReminders] Starting periodic chore reminders", "timezone", loc.String())

	for {
		nextRun := nextReminderTime(time.Now().In(loc), loc)
		waitDuration := time.Until(nextRun)

		slog.Info("[PeriodicReminders] Next run scheduled", "at", nextRun.Format(time.RFC3339), "in", waitDuration)

		select {
		case <-ctx.Done():
			slog.Info("[PeriodicReminders] Context done, stopping periodic chore reminders")
			return
		case <-time.After(waitDuration):
			sendChoreReminders(ctx, time.Now().In(loc), bot, s, loc)
		}
	}
}

// nextReminderTime calculates the next time to send reminders.
// It adds a random 3-6 hours to the current time.
// If the result is before 16:00, it advances to 16:00 same day.
// If the result is after 19:00, it advances to 16:00 next day + small random offset (0-30m).
func nextReminderTime(now time.Time, loc *time.Location) time.Time {
	// Random duration between 3 and 6 hours
	randHours := 3 + rand.Float64()*3
	next := now.Add(time.Duration(randHours * float64(time.Hour)))

	year, month, day := next.Date()
	hour := next.Hour()

	if hour < 16 {
		// Advance to 16:00 same day + random minutes (0-30)
		randMinutes := time.Duration(rand.IntN(30)) * time.Minute
		return time.Date(year, month, day, 16, 0, 0, 0, loc).Add(randMinutes)
	}

	if hour >= 19 {
		// Advance to 16:00 next day + 0-30 min random offset
		randMinutes := time.Duration(rand.IntN(30)) * time.Minute
		nextDay := next.AddDate(0, 0, 1)
		return time.Date(nextDay.Year(), nextDay.Month(), nextDay.Day(), 16, 0, 0, 0, loc).Add(randMinutes)
	}

	return next
}

func sendChoreReminders(ctx context.Context, now time.Time, bot BotSender, s store.Store, loc *time.Location) {
	hour := now.Hour()

	// Double check we are in the 16-19 window
	if hour < 16 || hour >= 19 {
		slog.Info("[PeriodicReminders] Skipping send, current time is outside the 16:00-19:00 window", "time", now)
		return
	}

	chores, err := s.GetActiveChores(ctx)
	if err != nil {
		slog.Error("[PeriodicReminders] Error retrieving active chores", "error", err)
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
		msg := FormatPeriodicChoreReminder(list, loc)
		if msg == "" {
			continue
		}

		err := bot.SendMessageHTML(telegramID, msg)
		if err != nil {
			slog.Error("[PeriodicReminders] Failed to send reminder", "telegramID", telegramID, "error", err)
		} else {
			slog.Info("[PeriodicReminders] Sent reminder", "telegramID", telegramID, "activeChoresCount", len(list))
		}
	}
}
