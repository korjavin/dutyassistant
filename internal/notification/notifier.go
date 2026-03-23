package notification

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"os"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/llm"
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
	llmClient *llm.Client
	// now is a function that returns the current time. It's used for testing.
	now func() time.Time
}

// NewNotifier creates and new Notifier.
func NewNotifier(s store.Store, sched Scheduler, bot TelegramBot, chatID int64, cronSpec string, loc *time.Location, llmClient *llm.Client) *Notifier {
	return &Notifier{
		store:     s,
		scheduler: sched,
		bot:       bot,
		location:  loc,
		chatID:    chatID,
		cronSpec:  cronSpec,
		llmClient: llmClient,
		now:       time.Now, // Use real time by default
	}
}

// Start initializes and starts the cron scheduler.
func (n *Notifier) Start() {
	slog.Info(fmt.Sprintf("Starting notifier with schedule '%s' in %s timezone", n.cronSpec, n.location))

	n.cron = cron.New(cron.WithLocation(n.location))
	_, err := n.cron.AddFunc(n.cronSpec, n.checkAndNotify)
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to add cron job: %v", err))
		os.Exit(1)
	}
	n.cron.Start()
}

// Stop gracefully stops the cron scheduler.
func (n *Notifier) Stop() {
	slog.Info("Stopping notifier...")
	if n.cron != nil {
		ctx := n.cron.Stop()
		<-ctx.Done()
	}
	slog.Info("Notifier stopped.")
}

// checkAndNotify is the core function executed by the cron job.
// It checks for tomorrow's duty, assigns one if needed, and sends notifications.
func (n *Notifier) checkAndNotify() {
	ctx := context.Background()
	slog.Info("Cron job triggered: checking for tomorrow's duty.")

	// Determine tomorrow's date in the service's configured timezone.
	nowInLocation := n.now().In(n.location)
	tomorrow := nowInLocation.Add(24 * time.Hour)

	var duty *store.Duty

	// 1. Check if a duty is already assigned for tomorrow.
	duty, err := n.store.GetDutyByDate(ctx, tomorrow)
	if err != nil {
		// We expect an error if no duty is found. Here we assume any error means "not found".
		// A more robust implementation would check for specific store.ErrNotFound.
		slog.Info(fmt.Sprintf("No duty found for %s. Attempting to assign one.", tomorrow.Format("2006-01-02")))
	}

	if duty == nil {
		// 2. If no duty, trigger round-robin assignment.
		newDuty, assignErr := n.scheduler.AssignDutyRoundRobin(ctx, tomorrow)
		if assignErr != nil {
			slog.Error(fmt.Sprintf("ERROR: Failed to auto-assign duty for %s: %v", tomorrow.Format("2006-01-02"), assignErr))
			// Optionally, send an error notification to an admin. For now, we just log.
			return
		}
		duty = newDuty
	}

	// 3. Send notifications if a duty is confirmed.
	if duty != nil {
		// Send message to the group chat
		groupMessageText := FormatDutyAssignedMessage(duty)
		if n.llmClient != nil {
			groupMessageText = n.llmClient.RefineMessage(ctx, "congratulate duty assignee proudly", groupMessageText)
		}

		groupMsg := tgbotapi.NewMessage(n.chatID, groupMessageText)
		groupMsg.ParseMode = tgbotapi.ModeHTML

		if _, err := n.bot.Send(groupMsg); err != nil {
			slog.Error(fmt.Sprintf("ERROR: Failed to send group notification to chat ID %d: %v", n.chatID, err))
		} else {
			slog.Info(fmt.Sprintf("Successfully sent group notification for duty on %s.", tomorrow.Format("2006-01-02")))
		}

		// Send DM to the assigned user
		if duty.User != nil && duty.User.TelegramUserID != 0 {
			dmMessageText := FormatDMToAssignee(duty)
			if n.llmClient != nil {
				dmMessageText = n.llmClient.RefineMessage(ctx, "friendly congratulatory DM to person assigned chore", dmMessageText)
			}

			dmMsg := tgbotapi.NewMessage(duty.User.TelegramUserID, dmMessageText)
			dmMsg.ParseMode = tgbotapi.ModeHTML

			if _, err := n.bot.Send(dmMsg); err != nil {
				slog.Error(fmt.Sprintf("ERROR: Failed to send DM to user %s (ID: %d): %v", duty.User.FirstName, duty.User.TelegramUserID, err))
			} else {
				slog.Info(fmt.Sprintf("Successfully sent DM to user %s for duty on %s.", duty.User.FirstName, tomorrow.Format("2006-01-02")))
			}
		} else {
			slog.Warn(fmt.Sprintf("WARNING: Cannot send DM - user data is incomplete for duty on %s", tomorrow.Format("2006-01-02")))
		}
	}
}

// SendDailyChoreSummary sends a daily summary of overdue chores.
func SendDailyChoreSummary(ctx context.Context, bot *tgbotapi.BotAPI, db store.Store, groupID int64, isCron bool, timezone string) error {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc, _ = time.LoadLocation("Europe/Berlin")
	}
	todayStr := time.Now().In(loc).Format("2006-01-02")
	if isCron {
		lastSent, _ := db.GetLastChoreDigestDate(ctx)
		if lastSent == todayStr {
			slog.Info(fmt.Sprintf("Daily chore summary already sent today (%s), skipping.", todayStr))
			return nil
		}
	}

	chores, err := db.GetOverdueChores(ctx)
	if err != nil {
		return fmt.Errorf("failed to get overdue chores: %w", err)
	}

	if len(chores) == 0 {
		if groupID != 0 {
			msg := tgbotapi.NewMessage(groupID, "No overdue chores ✅")
			if _, err := bot.Send(msg); err != nil {
				return fmt.Errorf("failed to send 'no chores' message: %w", err)
			}
		}
		if isCron {
			if err := db.SetLastChoreDigestDate(ctx, todayStr); err != nil {
				slog.Error(fmt.Sprintf("Failed to set last chore digest date: %v", err))
			}
		}
		return nil
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

		choreLine := fmt.Sprintf("• %s — %s (deadline: %s (+%d d))", html.EscapeString(chore.Description), userMention, deadline.Format("02.01.2006"), daysOverdue)

		if daysOverdue >= 3 {
			critical = append(critical, choreLine)
		} else if daysOverdue >= 1 {
			medium = append(medium, choreLine)
		} else {
			today = append(today, choreLine)
			// Escalation: Send personal DM reminder for chores that expired today
			if isCron && chore.User != nil && chore.User.TelegramUserID != 0 {
				dmText := fmt.Sprintf("⏰ <b>Chore Reminder</b>\n\nThe deadline for <i>%s</i> expired today. Please complete it as soon as possible!", html.EscapeString(chore.Description))
				msg := tgbotapi.NewMessage(chore.User.TelegramUserID, dmText)
				msg.ParseMode = tgbotapi.ModeHTML
				_, _ = bot.Send(msg)
			}
		}
	}

	if groupID != 0 {
		var summaryBuilder strings.Builder
		summaryBuilder.WriteString("⚠️ <b>Overdue chores:</b>\n\n")

		if len(critical) > 0 {
			summaryBuilder.WriteString("🔴 <b>Critical (3+d):</b>\n")
			for _, line := range critical {
				summaryBuilder.WriteString(line + "\n")
			}
			summaryBuilder.WriteString("\n")
		}

		if len(medium) > 0 {
			summaryBuilder.WriteString("🟠 <b>Overdue (1-2d):</b>\n")
			for _, line := range medium {
				summaryBuilder.WriteString(line + "\n")
			}
			summaryBuilder.WriteString("\n")
		}

		if len(today) > 0 {
			summaryBuilder.WriteString("🟢 <b>Due today:</b>\n")
			for _, line := range today {
				summaryBuilder.WriteString(line + "\n")
			}
		}

		msg := tgbotapi.NewMessage(groupID, summaryBuilder.String())
		msg.ParseMode = tgbotapi.ModeHTML
		_, err = bot.Send(msg)
		if err != nil {
			slog.Error(fmt.Sprintf("Failed to send chore summary to group: %v", err))
			return err
		}
	}

	if isCron {
		if err := db.SetLastChoreDigestDate(ctx, todayStr); err != nil {
			slog.Error(fmt.Sprintf("Failed to set last chore digest date: %v", err))
		}
	}

	return nil
}

func renderBarChart(value, maxValue float64, width int) string {
	if maxValue == 0 || width <= 0 {
		return strings.Repeat("░", width)
	}
	filledCount := int((value / maxValue) * float64(width))
	if filledCount > width {
		filledCount = width
	}
	return strings.Repeat("█", filledCount) + strings.Repeat("░", width-filledCount)
}

func determineWinner(stats []*store.UserWeeklyStats) *store.UserWeeklyStats {
	if len(stats) == 0 {
		return nil
	}

	// Data should already be sorted by completed_count DESC, but we do a full pass
	// to find the winner using highest CompletedCount, then lowest AvgLateSeconds as tie-breaker.
	winner := stats[0]
	for i := 1; i < len(stats); i++ {
		if stats[i].CompletedCount > winner.CompletedCount {
			winner = stats[i]
		} else if stats[i].CompletedCount == winner.CompletedCount {
			if stats[i].AvgLateSeconds < winner.AvgLateSeconds {
				winner = stats[i]
			}
		}
	}

	return winner
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

	since := time.Now().AddDate(0, 0, -7)
	weeklyStats, err := db.GetUserWeeklyStats(ctx, since)
	if err != nil {
		return fmt.Errorf("failed to fetch user weekly stats: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("📊 <b>Weekly Chore Statistics</b>\n\n")

	sb.WriteString("🔥 <b>Top 5 Overdue Chores:</b>\n")
	if len(topOverdue) == 0 {
		sb.WriteString("No overdue chores recorded.\n")
	} else {
		for i, chore := range topOverdue {
			fmt.Fprintf(&sb, "%d. %s (%d times)\n", i+1, html.EscapeString(chore.Description), chore.Count)
		}
	}
	sb.WriteString("\n")

	sb.WriteString("🏆 <b>Top Performers this week:</b>\n")
	if len(weeklyStats) == 0 {
		sb.WriteString("No completed chores recorded this week.\n")
	} else {
		for i, stat := range weeklyStats {
			fmt.Fprintf(&sb, "%d. %s — %d done\n", i+1, html.EscapeString(stat.Name), stat.CompletedCount)
		}

		winner := determineWinner(weeklyStats)
		if winner != nil {
			fmt.Fprintf(&sb, "🥇 Winner: %s\n", html.EscapeString(winner.Name))
		}
	}

	msg := tgbotapi.NewMessage(groupID, sb.String())
	msg.ParseMode = tgbotapi.ModeHTML
	_, err = bot.Send(msg)
	return err
}
