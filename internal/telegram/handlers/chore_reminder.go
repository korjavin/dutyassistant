package handlers

import (
	"context"
	"crypto/rand"
	"fmt"
	"html"
	"log/slog"
	"math/big"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
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
}

// NewChoreReminderManager creates a new chore reminder manager
func NewChoreReminderManager(bot *tgbotapi.BotAPI, db store.Store, groupID int64) *ChoreReminderManager {
	crm := &ChoreReminderManager{
		activeChores: make(map[string]*ChoreAssignment),
		bot:          bot,
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
	randomSuffix, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		randomSuffix = big.NewInt(0)
	}
	return fmt.Sprintf("%d_%d_%06d", userID, timestamp.UnixNano(), randomSuffix.Int64())
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
		slog.Error(fmt.Sprintf("Failed to send initial DM to user %s (%d): %v", assignment.UserName, assignment.UserID, err))
		// Don't store assignment if DM failed - prevents memory leak
		return err
	}

	// Only store the assignment AFTER successful DM send
	crm.mu.Lock()
	crm.activeChores[assignment.ReminderID] = assignment
	crm.mu.Unlock()

	slog.Info(fmt.Sprintf("Sent initial DM to %s for chore: %s", assignment.UserName, assignment.Description))

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
		slog.Info(fmt.Sprintf("Chore assignment %s not found, possibly already completed", reminderID))
		return
	}

	if crm.bot == nil {
		slog.Info("Bot not configured, cannot send reminder")
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
		slog.Error(fmt.Sprintf("Failed to send reminder to user %d: %v", assignment.UserID, err))
	} else {
		slog.Info(fmt.Sprintf("Sent reminder to user %s for chore: %s", assignment.UserName, assignment.Description))
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
	slog.Info(fmt.Sprintf("Scheduled re-reminder for chore %s in 15 minutes", reminderID))
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
	slog.Info(fmt.Sprintf("Marked chore %s as completed", reminderID))
}

// ReassignChore updates the in-memory assignment to point at a new user.
// Must be called after the database row has been updated so that future
// reminder DMs and completion callbacks reach the new assignee.
func (crm *ChoreReminderManager) ReassignChore(reminderID string, newUserID int64, newUserName string) {
	crm.mu.Lock()
	defer crm.mu.Unlock()

	if assignment, exists := crm.activeChores[reminderID]; exists {
		assignment.UserID = newUserID
		assignment.UserName = newUserName
		slog.Info(fmt.Sprintf("Reassigned in-memory chore %s to user %s (%d)", reminderID, newUserName, newUserID))
	}
}

// CancelChore removes a cancelled chore from active tracking
func (crm *ChoreReminderManager) CancelChore(reminderID string) {
	crm.mu.Lock()
	defer crm.mu.Unlock()

	delete(crm.activeChores, reminderID)
	slog.Info(fmt.Sprintf("Removed cancelled chore %s from tracking", reminderID))
}

// SendCompletionToGroup sends a completion message to the group chat
func (crm *ChoreReminderManager) SendCompletionToGroup(assignment *ChoreAssignment) error {
	if crm.bot == nil {
		return fmt.Errorf("bot not configured")
	}

	if assignment.GroupID == 0 {
		slog.Info("No group configured for chore completion announcement")
		return nil
	}

	// Escape HTML at display time (stored values are unescaped)
	escapedName := html.EscapeString(assignment.UserName)
	escapedDesc := html.EscapeString(assignment.Description)

	// Use compact format for completion messages without LLM refinement
	// to ensure they stay brief and reduce channel noise
	completionMsg := fmt.Sprintf("✅ <b>%s</b> completed: <i>%s</i>", escapedName, escapedDesc)

	msg := tgbotapi.NewMessage(assignment.GroupID, completionMsg)
	msg.ParseMode = tgbotapi.ModeHTML

	if _, err := crm.bot.Send(msg); err != nil {
		slog.Error(fmt.Sprintf("Failed to send completion message to group %d: %v", assignment.GroupID, err))
		return err
	}

	slog.Info(fmt.Sprintf("Sent completion message to group for chore: %s", assignment.Description))
	return nil
}

func (crm *ChoreReminderManager) loadActiveChores(db store.Store, groupID int64) {
	chores, err := db.GetActiveChores(context.Background())
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to load active chores from database: %v", err))
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
	slog.Info(fmt.Sprintf("Loaded %d active chores from database", len(crm.activeChores)))
}
