package handlers

import (
	"context"
	"fmt"
	"github.com/korjavin/dutyassistant/internal/notification"
	"html"
	"log"
	"os"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/store"
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
		"/volunteer <days> - Add days to your volunteer queue.\n" +
		"/explain - Explain how the last assignment was made.\n" +
		"/chore - View your currently assigned chores.\n\n" +
		"*Admin Commands:*\n" +
		"/chore <description> [/<N>d] - Assign a chore to a random active user (optional: make it periodic every N days).\n" +
		"/list chore - List active periodic chores.\n" +
		"/cancel chore <id> - Cancel a periodic chore.\n" +
		"/assign <username> <days> - Add days to user's admin queue.\n" +
		"/unassign <username> <days> - Remove days from user's admin queue.\n" +
		"/change <date> <username> - Change assigned user for a date.\n" +
		"/offduty <username> <start> <end> - Set off-duty period (YYYY-MM-DD).\n" +
		"/vacation [on|off] - Toggle vacation mode (pauses all scheduling).\n" +
		"/users - List all users and their status.\n" +
		"/toggle\\_active <username> - Toggle a user's participation in the rotation."

	statusMessage = "<b>Duty Status for %s:</b>\n\n" +
		"📊 <b>Statistics:</b>\n" +
		"  • Total duties: %d\n" +
		"  • This month: %d\n" +
		"  • Next duty: %s\n\n" +
		"📋 <b>Queues:</b>\n" +
		"  • Volunteer queue: %d day(s)\n" +
		"  • Admin queue: %d day(s)\n\n" +
		"%s"

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

		// Check if this user is the admin
		isAdmin := h.AdminID != 0 && m.From.ID == h.AdminID

		newUser := &store.User{
			TelegramUserID: m.From.ID,
			FirstName:      m.From.FirstName,
			IsActive:       !isAdmin, // Admin should be inactive by default
			IsAdmin:        isAdmin,
		}
		if createErr := h.Store.CreateUser(context.Background(), newUser); createErr != nil {
			log.Printf("[HandleStart] FAILED to create user %d: %v", m.From.ID, createErr)
			return tgbotapi.MessageConfig{}, fmt.Errorf("failed to create user: %w", createErr)
		}
		log.Printf("[HandleStart] Successfully created user %d with ID %d (IsAdmin=%v, IsActive=%v)", m.From.ID, newUser.ID, newUser.IsAdmin, newUser.IsActive)
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

	// Check off-duty status
	offDutyText := ""
	if user.OffDutyStart != nil && user.OffDutyEnd != nil {
		offDutyText = fmt.Sprintf("🏖 <b>Off-duty:</b> %s to %s",
			user.OffDutyStart.Format("2006-01-02"),
			user.OffDutyEnd.Format("2006-01-02"))
	}

	message := fmt.Sprintf(statusMessage,
		m.From.FirstName,
		stats.TotalDuties,
		stats.DutiesThisMonth,
		nextDuty,
		user.VolunteerQueueDays,
		user.AdminQueueDays,
		offDutyText)

	msg := tgbotapi.NewMessage(m.Chat.ID, message)
	msg.ParseMode = tgbotapi.ModeHTML
	return msg, nil
}

// HandleExplain provides an explanation of how the last assignment was made.
func (h *Handlers) HandleExplain(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	log.Printf("[HandleExplain] User %d triggered /explain", m.From.ID)

	explanation, err := h.Scheduler.ExplainLastAssignment(context.Background())
	if err != nil {
		log.Printf("[HandleExplain] Error explaining last assignment: %v", err)
		return tgbotapi.NewMessage(m.Chat.ID, "Не удалось получить объяснение: "+err.Error()), nil
	}

	msg := tgbotapi.NewMessage(m.Chat.ID, explanation)
	return msg, nil
}

// HandleOverdue handles the /overdue command for admins.
func (h *Handlers) HandleOverdue(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	isAdmin, err := h.checkAdmin(m.From.ID)
	if err != nil || !isAdmin {
		return tgbotapi.NewMessage(m.Chat.ID, adminOnlyMessage), nil
	}

	err = notification.SendDailyChoreSummary(context.Background(), h.Bot, h.Store, m.Chat.ID, false, os.Getenv("CHORE_TIMEZONE"))
	if err != nil {
		return tgbotapi.NewMessage(m.Chat.ID, "Failed to generate overdue report."), nil
	}

	// SendDailyChoreSummary already sent the message to m.Chat.ID, so return a small ack.
	return tgbotapi.NewMessage(m.Chat.ID, "Report generated successfully. 👆"), nil
}

// HandleChoreStats handles the /chore_stats command for admins.
func (h *Handlers) HandleChoreStats(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	isAdmin, err := h.checkAdmin(m.From.ID)
	if err != nil || !isAdmin {
		return tgbotapi.NewMessage(m.Chat.ID, adminOnlyMessage), nil
	}

	topOverdue, err := h.Store.GetTopOverdueChores(context.Background(), 5)
	if err != nil {
		return tgbotapi.NewMessage(m.Chat.ID, "Failed to fetch overdue stats."), nil
	}

	topUsers, err := h.Store.GetTopCompletedChoresUsers(context.Background(), 5)
	if err != nil {
		return tgbotapi.NewMessage(m.Chat.ID, "Failed to fetch user stats."), nil
	}

	var sb strings.Builder
	sb.WriteString("📊 <b>Chore Statistics</b>\n\n")

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

	msg := tgbotapi.NewMessage(m.Chat.ID, sb.String())
	msg.ParseMode = tgbotapi.ModeHTML
	return msg, nil
}
