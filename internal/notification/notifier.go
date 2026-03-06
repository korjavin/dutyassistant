package notification

import (
	"context"
	"html"
	"strings"
	"fmt"
	"log"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/robfig/cron/v3"
)

// Scheduler defines the interface for duty assignment operations.
type Scheduler interface {
	AssignDutyRoundRobin(ctx context.Context, date time.Time) (*store.Duty, error)
}

// TelegramBot defines the interface for sending Telegram messages.
type TelegramBot interface {
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
}

// Notifier manages scheduled duty notifications.
type Notifier struct {
	store     store.Store
	scheduler Scheduler
	bot       TelegramBot
	cron      *cron.Cron
	location  *time.Location
	chatID    int64
	cronSpec  string
	// now is a function that returns the current time. It's used for testing.
	now func() time.Time
}

// NewNotifier creates and new Notifier.
func NewNotifier(s store.Store, sched Scheduler, bot TelegramBot, chatID int64, cronSpec string, loc *time.Location) *Notifier {
	return &Notifier{
		store:     s,
		scheduler: sched,
		bot:       bot,
		location:  loc,
		chatID:    chatID,
		cronSpec:  cronSpec,
		now:       time.Now, // Use real time by default
	}
}

// Start initializes and starts the cron scheduler.
func (n *Notifier) Start() {
	log.Printf("Starting notifier with schedule '%s' in %s timezone", n.cronSpec, n.location)

	n.cron = cron.New(cron.WithLocation(n.location))
	_, err := n.cron.AddFunc(n.cronSpec, n.checkAndNotify)
	if err != nil {
		log.Fatalf("Failed to add cron job: %v", err)
	}
	n.cron.Start()
}

// Stop gracefully stops the cron scheduler.
func (n *Notifier) Stop() {
	log.Println("Stopping notifier...")
	if n.cron != nil {
		ctx := n.cron.Stop()
		<-ctx.Done()
	}
	log.Println("Notifier stopped.")
}

// checkAndNotify is the core function executed by the cron job.
// It checks for tomorrow's duty, assigns one if needed, and sends notifications.
func (n *Notifier) checkAndNotify() {
	ctx := context.Background()
	log.Println("Cron job triggered: checking for tomorrow's duty.")

	// Determine tomorrow's date in the service's configured timezone.
	nowInLocation := n.now().In(n.location)
	tomorrow := nowInLocation.Add(24 * time.Hour)

	var duty *store.Duty

	// 1. Check if a duty is already assigned for tomorrow.
	duty, err := n.store.GetDutyByDate(ctx, tomorrow)
	if err != nil {
		// We expect an error if no duty is found. Here we assume any error means "not found".
		// A more robust implementation would check for specific store.ErrNotFound.
		log.Printf("No duty found for %s. Attempting to assign one.", tomorrow.Format("2006-01-02"))
	}

	if duty == nil {
		// 2. If no duty, trigger round-robin assignment.
		newDuty, assignErr := n.scheduler.AssignDutyRoundRobin(ctx, tomorrow)
		if assignErr != nil {
			log.Printf("ERROR: Failed to auto-assign duty for %s: %v", tomorrow.Format("2006-01-02"), assignErr)
			// Optionally, send an error notification to an admin. For now, we just log.
			return
		}
		duty = newDuty
	}

	// 3. Send notifications if a duty is confirmed.
	if duty != nil {
		// Send message to the group chat
		groupMessageText := FormatDutyAssignedMessage(duty)
		groupMsg := tgbotapi.NewMessage(n.chatID, groupMessageText)
		groupMsg.ParseMode = tgbotapi.ModeMarkdownV2

		if _, err := n.bot.Send(groupMsg); err != nil {
			log.Printf("ERROR: Failed to send group notification to chat ID %d: %v", n.chatID, err)
		} else {
			log.Printf("Successfully sent group notification for duty on %s.", tomorrow.Format("2006-01-02"))
		}

		// Send DM to the assigned user
		if duty.User != nil && duty.User.TelegramUserID != 0 {
			dmMessageText := FormatDMToAssignee(duty)
			dmMsg := tgbotapi.NewMessage(duty.User.TelegramUserID, dmMessageText)
			dmMsg.ParseMode = tgbotapi.ModeMarkdownV2

			if _, err := n.bot.Send(dmMsg); err != nil {
				log.Printf("ERROR: Failed to send DM to user %s (ID: %d): %v", duty.User.FirstName, duty.User.TelegramUserID, err)
			} else {
				log.Printf("Successfully sent DM to user %s for duty on %s.", duty.User.FirstName, tomorrow.Format("2006-01-02"))
			}
		} else {
			log.Printf("WARNING: Cannot send DM - user data is incomplete for duty on %s", tomorrow.Format("2006-01-02"))
		}
	}
}
// SendDailyChoreSummary sends a daily summary of overdue chores.
func SendDailyChoreSummary(ctx context.Context, bot *tgbotapi.BotAPI, db store.Store, groupID int64, isCron bool, timezone string) error {
	todayStr := time.Now().UTC().Format("2006-01-02")
	if isCron {
		lastSent, _ := db.GetLastChoreDigestDate(ctx)
		if lastSent == todayStr {
			log.Printf("Daily chore summary already sent today (%s), skipping.", todayStr)
			return nil
		}
	}

	chores, err := db.GetOverdueChores(ctx)
	if err != nil {
		return fmt.Errorf("failed to get overdue chores: %w", err)
	}

	if len(chores) == 0 {
		if groupID != 0 {
			msg := tgbotapi.NewMessage(groupID, "Просроченных chores нет ✅")
			_, _ = bot.Send(msg)
		}
		if isCron {
			if err := db.SetLastChoreDigestDate(ctx, todayStr); err != nil {
				log.Printf("Failed to set last chore digest date: %v", err)
			}
		}
		return nil
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc, _ = time.LoadLocation("Europe/Berlin")
	}
	now := time.Now().In(loc)
	nowDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	var critical []string
	var medium []string
	var today []string

	for _, chore := range chores {
		deadline := chore.DeadlineAt.In(loc)
		deadlineDate := time.Date(deadline.Year(), deadline.Month(), deadline.Day(), 0, 0, 0, 0, loc)

		daysOverdue := int(nowDate.Sub(deadlineDate).Hours() / 24)
		if daysOverdue < 0 {
			daysOverdue = 0
		}

		var userMention string
		if chore.User != nil {
			if chore.User.TelegramUserID != 0 {
				userMention = fmt.Sprintf("<a href=\"tg://user?id=%d\">%s</a>", chore.User.TelegramUserID, html.EscapeString(chore.User.FirstName))
			} else {
				userMention = html.EscapeString(chore.User.FirstName)
			}
		} else {
			userMention = "Unknown"
		}

		choreLine := fmt.Sprintf("• %s — %s (дедлайн: %s, просрочка: %d дн.)", html.EscapeString(chore.Description), userMention, deadline.Format("02.01.2006"), daysOverdue)

		if daysOverdue >= 3 {
			critical = append(critical, choreLine)
		} else if daysOverdue >= 1 {
			medium = append(medium, choreLine)
		} else {
			today = append(today, choreLine)
			// Escalation: Send personal DM reminder for chores that expired today
			if isCron && chore.User != nil && chore.User.TelegramUserID != 0 {
				dmText := fmt.Sprintf("⏰ <b>Напоминание о задаче</b>\n\nСрок выполнения задачи <i>%s</i> истек сегодня. Пожалуйста, завершите её как можно скорее!", html.EscapeString(chore.Description))
				msg := tgbotapi.NewMessage(chore.User.TelegramUserID, dmText)
				msg.ParseMode = tgbotapi.ModeHTML
				_, _ = bot.Send(msg)
			}
		}
	}

	if groupID != 0 {
		var summaryBuilder strings.Builder
		summaryBuilder.WriteString("📊 <b>Сводка по просроченным chores</b>\n\n")

		if len(critical) > 0 {
			summaryBuilder.WriteString("🔴 <b>Критично (3+ дней):</b>\n")
			for _, line := range critical {
				summaryBuilder.WriteString(line + "\n")
			}
			summaryBuilder.WriteString("\n")
		}

		if len(medium) > 0 {
			summaryBuilder.WriteString("🟠 <b>Средне (1–2 дня):</b>\n")
			for _, line := range medium {
				summaryBuilder.WriteString(line + "\n")
			}
			summaryBuilder.WriteString("\n")
		}

		if len(today) > 0 {
			summaryBuilder.WriteString("🟢 <b>Истек срок сегодня:</b>\n")
			for _, line := range today {
				summaryBuilder.WriteString(line + "\n")
			}
		}

		msg := tgbotapi.NewMessage(groupID, summaryBuilder.String())
		msg.ParseMode = tgbotapi.ModeHTML
		_, err = bot.Send(msg)
		if err != nil {
			log.Printf("Failed to send chore summary to group: %v", err)
			return err
		}
	}

	if isCron {
		if err := db.SetLastChoreDigestDate(ctx, todayStr); err != nil {
			log.Printf("Failed to set last chore digest date: %v", err)
		}
	}

	return nil
}

// SendWeeklyChoreStats sends a weekly statistics report of chores.
func SendWeeklyChoreStats(ctx context.Context, bot *tgbotapi.BotAPI, db store.Store, groupID int64) error {
	if groupID == 0 {
		return nil
	}

	topOverdue, err := db.GetTopOverdueChores(ctx, 5)
	if err != nil {
		return fmt.Errorf("failed to fetch top overdue chores: %w", err)
	}

	topUsers, err := db.GetTopCompletedChoresUsers(ctx, 5)
	if err != nil {
		return fmt.Errorf("failed to fetch top completed chores users: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("📊 <b>Weekly Chore Statistics</b>\n\n")

	sb.WriteString("🔥 <b>Top 5 Overdue Chores:</b>\n")
	if len(topOverdue) == 0 {
		sb.WriteString("No overdue chores recorded.\n")
	} else {
		for i, chore := range topOverdue {
			sb.WriteString(fmt.Sprintf("%d. %s (%d times)\n", i+1, html.EscapeString(chore.Description), chore.Count))
		}
	}
	sb.WriteString("\n")

	sb.WriteString("🏆 <b>Top 5 Users (Completed Chores):</b>\n")
	if len(topUsers) == 0 {
		sb.WriteString("No completed chores recorded.\n")
	} else {
		for i, stat := range topUsers {
			sb.WriteString(fmt.Sprintf("%d. %s (%d completed)\n", i+1, html.EscapeString(stat.Name), stat.Count))
		}
	}

	msg := tgbotapi.NewMessage(groupID, sb.String())
	msg.ParseMode = tgbotapi.ModeHTML
	_, err = bot.Send(msg)
	return err
}
