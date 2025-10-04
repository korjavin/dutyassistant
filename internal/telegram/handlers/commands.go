package handlers

import (
	"context"
	"fmt"
	"log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	startMessage = "Welcome to the Roster Bot! I can help you manage your duty schedule.\n\n" +
		"Use /schedule to see the current schedule.\n" +
		"Use /volunteer to sign up for a duty.\n" +
		"Use /help to see all available commands."

	helpMessage = "Here are the available commands:\n\n" +
		"/start - Show the welcome message.\n" +
		"/help - Show this help message.\n" +
		"/status - Show your current duty statistics.\n" +
		"/schedule - View the duty schedule for the current month.\n" +
		"/volunteer - Volunteer for a duty on a specific date.\n\n" +
		"*Admin Commands:*\n" +
		"/assign <username> <date> - Assign a user to a duty.\n" +
		"/modify <date> <new_username> - Change the user for a duty.\n" +
		"/users - List all users and their status.\n" +
		"/toggle_active <username> - Toggle a user's participation in the rotation."

	statusMessage = "Duty Status for %s:\n\n" +
		"Total Duties Assigned: %d\n" +
		"Duties this month: %d\n" +
		"Next scheduled duty: %s"
)

// HandleStart handles the /start command.
func (h *Handlers) HandleStart(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	msg := tgbotapi.NewMessage(m.Chat.ID, startMessage)
	return msg, nil
}

// HandleHelp handles the /help command.
func (h *Handlers) HandleHelp(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	msg := tgbotapi.NewMessage(m.Chat.ID, helpMessage)
	msg.ParseMode = tgbotapi.ModeMarkdown
	return msg, nil
}

// HandleStatus handles the /status command.
func (h *Handlers) HandleStatus(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	user, err := h.Store.GetUserByTelegramID(context.Background(), m.From.ID)
	if err != nil {
		// This could be a new user who hasn't been added to the system yet.
		return tgbotapi.NewMessage(m.Chat.ID, "Could not find your user profile. Are you registered?"), nil
	}

	stats, err := h.Store.GetUserStats(context.Background(), user.ID)
	if err != nil {
		// Log the error and return a generic failure message
		log.Printf("Error getting user stats for user %d: %v", user.ID, err)
		return tgbotapi.NewMessage(m.Chat.ID, "Sorry, I couldn't retrieve your stats at this time."), nil
	}

	nextDuty := stats.NextDutyDate
	if nextDuty == "" {
		nextDuty = "Not scheduled"
	}

	message := fmt.Sprintf(statusMessage, m.From.FirstName, stats.TotalDuties, stats.DutiesThisMonth, nextDuty)
	msg := tgbotapi.NewMessage(m.Chat.ID, message)
	return msg, nil
}