package notification

import (
	"context"
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
// It checks for tomorrow's duty, assigns one if needed, and sends a notification.
func (n *Notifier) checkAndNotify() {
	ctx := context.Background()
	log.Println("Cron job triggered: checking for tomorrow's duty.")

	// Determine tomorrow's date in the service's configured timezone.
	nowInLocation := n.now().In(n.location)
	tomorrow := nowInLocation.Add(24 * time.Hour)

	var messageText string
	var dutyAssigned bool

	// 1. Check if a duty is already assigned for tomorrow.
	duty, err := n.store.GetDutyByDate(ctx, tomorrow)
	if err != nil {
		// We expect an error if no duty is found. Here we assume any error means "not found".
		// A more robust implementation would check for specific store.ErrNotFound.
		log.Printf("No duty found for %s. Attempting to assign one.", tomorrow.Format("2006-01-02"))
	}

	if duty != nil {
		// Duty already exists, format a reminder message.
		messageText = FormatDutyAssignedMessage(duty)
		dutyAssigned = true
	} else {
		// 2. If no duty, trigger round-robin assignment.
		newDuty, assignErr := n.scheduler.AssignDutyRoundRobin(ctx, tomorrow)
		if assignErr != nil {
			log.Printf("ERROR: Failed to auto-assign duty for %s: %v", tomorrow.Format("2006-01-02"), assignErr)
			// Optionally, send an error notification to an admin. For now, we just log.
			return
		}
		// Format an auto-assignment message.
		messageText = FormatDutyAutoAssignedMessage(newDuty)
		dutyAssigned = true
	}

	// 3. Send the notification if a duty is confirmed.
	if dutyAssigned {
		msg := tgbotapi.NewMessage(n.chatID, messageText)
		msg.ParseMode = tgbotapi.ModeMarkdownV2

		if _, err := n.bot.Send(msg); err != nil {
			log.Printf("ERROR: Failed to send Telegram notification to chat ID %d: %v", n.chatID, err)
		} else {
			log.Printf("Successfully sent notification for duty on %s.", tomorrow.Format("2006-01-02"))
		}
	}
}