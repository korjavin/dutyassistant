package handlers

import (
	"context"
	"fmt"
	"html"
	"log"
	"math/rand"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/llm"
	"github.com/korjavin/dutyassistant/internal/store"
)

// ChoreAssignment represents an active chore assignment
type ChoreAssignment struct {
	UserID      int64
	UserName    string
	Description string
	AssignedAt  time.Time
	GroupID     int64
	ReminderID  string // Unique ID for tracking this assignment
}

// ChoreReminderManager manages chore reminders and tracking
type ChoreReminderManager struct {
	activeChores map[string]*ChoreAssignment // key is reminderID
	mu           sync.RWMutex
	bot          *tgbotapi.BotAPI
	llmClient    *llm.Client
}

// NewChoreReminderManager creates a new chore reminder manager
func NewChoreReminderManager(bot *tgbotapi.BotAPI, db store.Store, groupID int64, llmClient *llm.Client) *ChoreReminderManager {
	crm := &ChoreReminderManager{
		activeChores: make(map[string]*ChoreAssignment),
		bot:          bot,
		llmClient:    llmClient,
	}
	if db != nil {
		crm.loadActiveChores(db, groupID)
	}
	return crm
}

// GenerateReminderID creates a unique ID for a chore assignment
// Uses nanosecond precision + random component to prevent collisions
func GenerateReminderID(userID int64, timestamp time.Time) string {
	// Add random 6-digit suffix to prevent collisions even if called at exact same nanosecond
	randomSuffix := rand.Intn(1000000)
	return fmt.Sprintf("%d_%d_%06d", userID, timestamp.UnixNano(), randomSuffix)
}

// SendInitialDM sends the initial DM to the assigned user and schedules the first reminder
// Only stores the assignment in memory after successful DM send to prevent memory leaks
func (crm *ChoreReminderManager) SendInitialDM(assignment *ChoreAssignment) error {
	if crm.bot == nil {
		return fmt.Errorf("bot not configured")
	}

	// Send initial DM FIRST (before storing)
	// Escape HTML at display time (assignment.Description is stored unescaped)
	escapedDesc := html.EscapeString(assignment.Description)
	initialMsg := fmt.Sprintf(
		"🎉 <b>Congratulations!</b>\n\nYou've got a new chore:\n\n<i>%s</i>\n\nUse the buttons below when you're done or if you want to snooze the reminder.",
		escapedDesc,
	)

	msg := tgbotapi.NewMessage(assignment.UserID, initialMsg)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.ReplyMarkup = choreReminderKeyboard(assignment.ReminderID)

	if _, err := crm.bot.Send(msg); err != nil {
		log.Printf("Failed to send initial DM to user %s (%d): %v", assignment.UserName, assignment.UserID, err)
		// Don't store assignment if DM failed - prevents memory leak
		return err
	}

	// Only store the assignment AFTER successful DM send
	crm.mu.Lock()
	crm.activeChores[assignment.ReminderID] = assignment
	crm.mu.Unlock()

	log.Printf("Sent initial DM to %s for chore: %s", assignment.UserName, assignment.Description)

	// Schedule reminder in 10 minutes
	time.AfterFunc(10*time.Minute, func() {
		crm.SendReminderWithButtons(assignment.ReminderID)
	})

	return nil
}

// SendReminderWithButtons sends a reminder with inline buttons
func (crm *ChoreReminderManager) SendReminderWithButtons(reminderID string) {
	crm.mu.RLock()
	assignment, exists := crm.activeChores[reminderID]
	crm.mu.RUnlock()

	if !exists {
		log.Printf("Chore assignment %s not found, possibly already completed", reminderID)
		return
	}

	if crm.bot == nil {
		log.Printf("Bot not configured, cannot send reminder")
		return
	}

	// Escape HTML at display time (assignment.Description is stored unescaped)
	escapedDesc := html.EscapeString(assignment.Description)
	reminderText := fmt.Sprintf(
		"⏰ <b>Chore Reminder</b>\n\nHave you finished this task?\n\n<i>%s</i>",
		escapedDesc,
	)

	msg := tgbotapi.NewMessage(assignment.UserID, reminderText)
	msg.ParseMode = tgbotapi.ModeHTML
	msg.ReplyMarkup = choreReminderKeyboard(reminderID)

	if _, err := crm.bot.Send(msg); err != nil {
		log.Printf("Failed to send reminder to user %d: %v", assignment.UserID, err)
	} else {
		log.Printf("Sent reminder to user %s for chore: %s", assignment.UserName, assignment.Description)
	}
}

func choreReminderKeyboard(reminderID string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ I've done it", fmt.Sprintf("chore_done:%s", reminderID)),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⏰ Remind me in 15 min", fmt.Sprintf("chore_remind:%s", reminderID)),
		),
	)
}

// ScheduleReReminder schedules another reminder in 15 minutes
func (crm *ChoreReminderManager) ScheduleReReminder(reminderID string) {
	time.AfterFunc(15*time.Minute, func() {
		crm.SendReminderWithButtons(reminderID)
	})
	log.Printf("Scheduled re-reminder for chore %s in 15 minutes", reminderID)
}

// GetAssignment retrieves a chore assignment by ID
func (crm *ChoreReminderManager) GetAssignment(reminderID string) (*ChoreAssignment, bool) {
	crm.mu.RLock()
	defer crm.mu.RUnlock()

	assignment, exists := crm.activeChores[reminderID]
	return assignment, exists
}

// CompleteChore marks a chore as completed and removes it from active tracking
func (crm *ChoreReminderManager) CompleteChore(reminderID string) {
	crm.mu.Lock()
	defer crm.mu.Unlock()

	delete(crm.activeChores, reminderID)
	log.Printf("Marked chore %s as completed", reminderID)
}

// CancelChore removes a cancelled chore from active tracking
func (crm *ChoreReminderManager) CancelChore(reminderID string) {
	crm.mu.Lock()
	defer crm.mu.Unlock()

	delete(crm.activeChores, reminderID)
	log.Printf("Removed cancelled chore %s from tracking", reminderID)
}

// SendCompletionToGroup sends a completion message to the group chat
func (crm *ChoreReminderManager) SendCompletionToGroup(assignment *ChoreAssignment) error {
	if crm.bot == nil {
		return fmt.Errorf("bot not configured")
	}

	if assignment.GroupID == 0 {
		log.Printf("No group configured for chore completion announcement")
		return nil
	}

	// Escape HTML at display time (stored values are unescaped)
	escapedName := html.EscapeString(assignment.UserName)
	escapedDesc := html.EscapeString(assignment.Description)

	completionMsg := fmt.Sprintf(
		"✅ <b>Chore Completed!</b>\n\nUser <b>%s</b> finished chore:\n\n<i>%s</i>\n\n"+
			"Let's check and be proud of them, or give them some admin assignment otherwise! 😊",
		escapedName,
		escapedDesc,
	)

	if crm.llmClient != nil {
		completionMsg = crm.llmClient.RefineMessage(context.Background(), "celebrate chore completion, be proud of them", completionMsg)
	}

	msg := tgbotapi.NewMessage(assignment.GroupID, completionMsg)
	msg.ParseMode = tgbotapi.ModeHTML

	if _, err := crm.bot.Send(msg); err != nil {
		log.Printf("Failed to send completion message to group %d: %v", assignment.GroupID, err)
		return err
	}

	log.Printf("Sent completion message to group for chore: %s", assignment.Description)
	return nil
}

func (crm *ChoreReminderManager) loadActiveChores(db store.Store, groupID int64) {
	chores, err := db.GetActiveChores(context.Background())
	if err != nil {
		log.Printf("Failed to load active chores from database: %v", err)
		return
	}

	crm.mu.Lock()
	defer crm.mu.Unlock()

	for _, chore := range chores {
		if chore.User != nil {
			crm.activeChores[chore.ReminderID] = &ChoreAssignment{
				UserID:      chore.User.TelegramUserID,
				UserName:    chore.User.FirstName,
				Description: chore.Description,
				AssignedAt:  chore.AssignedAt,
				GroupID:     groupID,
				ReminderID:  chore.ReminderID,
			}
		}
	}
	log.Printf("Loaded %d active chores from database", len(crm.activeChores))
}
