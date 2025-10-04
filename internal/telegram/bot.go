package telegram

import (
	"context"
	"log"
	"strings"

	"github.com/korjavin/dutyassistant/internal/telegram/handlers"
	"github.com/korjavin/dutyassistant/internal/telegram/keyboard"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Bot represents the Telegram bot application.
type Bot struct {
	api      *tgbotapi.BotAPI
	handlers *handlers.Handlers
}

// NewBot creates a new Bot instance.
func NewBot(apiToken string, h *handlers.Handlers) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(apiToken)
	if err != nil {
		return nil, err
	}
	api.Debug = false // Set to true for verbose logging
	log.Printf("Authorized on account %s", api.Self.UserName)

	return &Bot{
		api:      api,
		handlers: h,
	}, nil
}

// Start begins listening for and processing updates from Telegram.
func (b *Bot) Start(ctx context.Context) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	for {
		select {
		case update := <-updates:
			b.handleUpdate(update)
		case <-ctx.Done():
			return
		}
	}
}

// handleUpdate is the central dispatcher for all incoming updates.
func (b *Bot) handleUpdate(update tgbotapi.Update) {
	var err error
	var response tgbotapi.Chattable

	switch {
	case update.Message != nil && update.Message.IsCommand():
		response, err = b.handleCommand(update.Message)
	case update.CallbackQuery != nil:
		response, err = b.handleCallbackQuery(update.CallbackQuery)
	}

	if err != nil {
		log.Printf("Error handling update: %v", err)
		var chatID int64
		if update.Message != nil {
			chatID = update.Message.Chat.ID
		} else if update.CallbackQuery != nil {
			chatID = update.CallbackQuery.Message.Chat.ID
		}
		if chatID != 0 {
			response = tgbotapi.NewMessage(chatID, "An unexpected error occurred. Please try again.")
		} else {
			response = nil
		}
	}

	if response != nil {
		if _, err := b.api.Send(response); err != nil {
			log.Printf("Error sending response: %v", err)
		}
	}
}

// handleCommand routes a command to the appropriate handler.
func (b *Bot) handleCommand(m *tgbotapi.Message) (tgbotapi.Chattable, error) {
	switch m.Command() {
	case "start":
		return b.handlers.HandleStart(m)
	case "help":
		return b.handlers.HandleHelp(m)
	case "status":
		return b.handlers.HandleStatus(m)
	case "schedule":
		return b.handlers.HandleSchedule(m)
	case "volunteer":
		return b.handlers.HandleVolunteer(m)
	case "assign":
		return b.handlers.HandleAssign(m)
	case "modify":
		return b.handlers.HandleModify(m)
	case "users":
		return b.handlers.HandleUsers(m)
	case "toggle_active":
		return b.handlers.HandleToggleActive(m)
	default:
		msg := tgbotapi.NewMessage(m.Chat.ID, "Unknown command. Use /help for a list of commands.")
		return msg, nil
	}
}

// handleCallbackQuery routes a callback query to the appropriate handler.
func (b *Bot) handleCallbackQuery(q *tgbotapi.CallbackQuery) (tgbotapi.Chattable, error) {
	// Answer the callback query to remove the "loading" state on the user's side.
	callback := tgbotapi.NewCallback(q.ID, "")
	if _, err := b.api.Request(callback); err != nil {
		log.Printf("failed to answer callback query: %v", err)
	}

	action := strings.Split(q.Data, ":")[0]

	switch action {
	case keyboard.ActionPrevMonth, keyboard.ActionNextMonth:
		// These callbacks can come from either the /schedule or /volunteer calendars.
		// We can try to guess the context from the message text.
		if strings.Contains(q.Message.Text, "volunteer") {
			// It's a volunteer calendar, we don't need to show duties.
			return b.handlers.HandleCalendarCallback(q) // Simplified version for volunteer
		}
		return b.handlers.HandleCalendarCallback(q)
	case keyboard.ActionSelectDay:
		return b.handlers.HandleVolunteerCallback(q)
	case keyboard.ActionIgnore:
		return nil, nil // Do nothing for ignore actions
	default:
		log.Printf("Unknown callback action: %s", action)
		return nil, nil
	}
}