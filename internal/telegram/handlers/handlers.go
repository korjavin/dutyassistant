package handlers

import (
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/scheduler"
	"github.com/korjavin/dutyassistant/internal/store"
)

// TimeNow allows overriding time.Now in tests.
var TimeNow = time.Now

// Handlers holds dependencies for command handlers, such as the database store
// and the business logic scheduler. This approach centralizes dependencies.
type Handlers struct {
	Store                store.Store
	Scheduler            scheduler.SchedulerInterface
	GroupID              int64                 // Group ID for announcements
	AdminID              int64                 // Telegram user ID of the admin from ADMIN_ID env var
	Bot                  *tgbotapi.BotAPI      // Bot API instance for sending notifications
	SessionManager       *SessionManager       // Session manager for interactive commands
	ChoreReminderManager *ChoreReminderManager // Chore reminder manager
	ratingsMu            sync.Mutex
}

// SetBot sets the bot API instance for the handlers.
func (h *Handlers) SetBot(bot *tgbotapi.BotAPI) {
	h.Bot = bot
	// Initialize ChoreReminderManager when bot is set
	if h.ChoreReminderManager == nil {
		h.ChoreReminderManager = NewChoreReminderManager(bot, h.Store, h.GroupID)
	}
}

// New creates a new Handlers instance with the provided dependencies.
func New(s store.Store, sch scheduler.SchedulerInterface, groupID int64) *Handlers {
	return &Handlers{
		Store:          s,
		Scheduler:      sch,
		GroupID:        groupID,
		SessionManager: NewSessionManager(),
	}
}

// NewWithAdminID creates a new Handlers instance with admin ID configured.
func NewWithAdminID(s store.Store, sch scheduler.SchedulerInterface, groupID, adminID int64) *Handlers {
	return &Handlers{
		Store:          s,
		Scheduler:      sch,
		GroupID:        groupID,
		AdminID:        adminID,
		SessionManager: NewSessionManager(),
	}
}
