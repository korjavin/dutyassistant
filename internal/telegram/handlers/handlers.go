package handlers

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/scheduler"
	"github.com/korjavin/dutyassistant/internal/store"
)

// Handlers holds dependencies for command handlers, such as the database store
// and the business logic scheduler. This approach centralizes dependencies.
type Handlers struct {
	Store     store.Store
	Scheduler scheduler.SchedulerInterface
	GroupID   int64            // Group ID for announcements
	AdminID   int64            // Telegram user ID of the admin from ADMIN_ID env var
	Bot       *tgbotapi.BotAPI // Bot API instance for sending notifications
}

// SetBot sets the bot API instance for the handlers.
func (h *Handlers) SetBot(bot *tgbotapi.BotAPI) {
	h.Bot = bot
}

// New creates a new Handlers instance with the provided dependencies.
func New(s store.Store, sch scheduler.SchedulerInterface, groupID int64) *Handlers {
	return &Handlers{
		Store:     s,
		Scheduler: sch,
		GroupID:   groupID,
	}
}

// NewWithAdminID creates a new Handlers instance with admin ID configured.
func NewWithAdminID(s store.Store, sch scheduler.SchedulerInterface, groupID, adminID int64) *Handlers {
	return &Handlers{
		Store:     s,
		Scheduler: sch,
		GroupID:   groupID,
		AdminID:   adminID,
	}
}
