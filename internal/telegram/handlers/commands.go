package handlers

import (
	"context"
	"fmt"
	"log"

	"github.com/korjavin/dutyassistant/internal/store"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	startMessage = "Welcome to the Roster Bot! I can help you manage your duty schedule.\n\n" +
		"Use /schedule to see the current schedule.\n" +
		"Use /volunteer to sign up for a duty.\n" +
		"Use /help to see all available commands."

	helpMessage = "Here are the available commands:\n\n" +
		"/start - Show the welcome message and register you.\n" +
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

	genericErrorMessage = "Sorry, something went wrong. Please try again later."
)

// HandleStart creates a new user if they don't exist, or updates their name if it has changed.
func (h *Handlers) HandleStart(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	log.Printf("[HandleStart] User %d (%s) triggered /start", m.From.ID, m.From.FirstName)

	user, err := h.Store.GetUserByTelegramID(context.Background(), m.From.ID)
	if err != nil {
		log.Printf("[HandleStart] Error getting user %d: %v", m.From.ID, err)
		return tgbotapi.MessageConfig{}, fmt.Errorf("database error: %w", err)
	}

	if user == nil {
		// User doesn't exist, create them
		log.Printf("[HandleStart] User %d not found, creating new user", m.From.ID)
		newUser := &store.User{
			TelegramUserID: m.From.ID,
			FirstName:      m.From.FirstName,
			IsActive:       true,
			IsAdmin:        false,
		}
		if createErr := h.Store.CreateUser(context.Background(), newUser); createErr != nil {
			log.Printf("[HandleStart] FAILED to create user %d: %v", m.From.ID, createErr)
			return tgbotapi.MessageConfig{}, fmt.Errorf("failed to create user: %w", createErr)
		}
		log.Printf("[HandleStart] Successfully created user %d with ID %d", m.From.ID, newUser.ID)
	} else if user.FirstName != m.From.FirstName {
		// User exists, update their name if it's different
		log.Printf("[HandleStart] Updating user %d name from '%s' to '%s'", m.From.ID, user.FirstName, m.From.FirstName)
		user.FirstName = m.From.FirstName
		if updateErr := h.Store.UpdateUser(context.Background(), user); updateErr != nil {
			log.Printf("[HandleStart] Failed to update user's first name: %v", updateErr)
		}
	} else {
		log.Printf("[HandleStart] User %d already exists, no changes needed", m.From.ID)
	}

	msg := tgbotapi.NewMessage(m.Chat.ID, startMessage)
	return msg, nil
}

// HandleHelp provides a list of available commands.
func (h *Handlers) HandleHelp(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	msg := tgbotapi.NewMessage(m.Chat.ID, helpMessage)
	msg.ParseMode = tgbotapi.ModeMarkdown
	return msg, nil
}

// HandleStatus fetches and displays the user's duty statistics.
func (h *Handlers) HandleStatus(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	user, err := h.Store.GetUserByTelegramID(context.Background(), m.From.ID)
	if err != nil || user == nil {
		return tgbotapi.NewMessage(m.Chat.ID, "Could not find your user profile. Please use /start first."), nil
	}

	stats, err := h.Store.GetUserStats(context.Background(), user.ID)
	if err != nil {
		log.Printf("Error getting user stats for user %d: %v", user.ID, err)
		return tgbotapi.NewMessage(m.Chat.ID, genericErrorMessage), nil
	}

	nextDuty := stats.NextDutyDate
	if nextDuty == "" {
		nextDuty = "Not scheduled"
	}

	message := fmt.Sprintf(statusMessage, m.From.FirstName, stats.TotalDuties, stats.DutiesThisMonth, nextDuty)
	msg := tgbotapi.NewMessage(m.Chat.ID, message)
	return msg, nil
}