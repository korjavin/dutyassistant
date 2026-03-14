package handlers

import (
	"context"
	"fmt"
	"html"
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// HandleChoreDoneCallback handles the "I've done it" button callback
func (h *Handlers) HandleChoreDoneCallback(q *tgbotapi.CallbackQuery) (tgbotapi.EditMessageTextConfig, error) {
	// Extract reminderID from callback data: "chore_done:reminderID"
	parts := strings.Split(q.Data, ":")
	if len(parts) != 2 {
		log.Printf("Invalid callback data format: %s", q.Data)
		edit := tgbotapi.NewEditMessageText(
			q.Message.Chat.ID,
			q.Message.MessageID,
			"❌ Error: Invalid callback data.",
		)
		return edit, nil
	}

	reminderID := parts[1]

	if h.ChoreReminderManager == nil {
		edit := tgbotapi.NewEditMessageText(
			q.Message.Chat.ID,
			q.Message.MessageID,
			"❌ Error: Reminder manager not configured.",
		)
		return edit, nil
	}

	// Get the assignment
	assignment, exists := h.ChoreReminderManager.GetAssignment(reminderID)
	if !exists {
		edit := tgbotapi.NewEditMessageText(
			q.Message.Chat.ID,
			q.Message.MessageID,
			"❌ This chore assignment was not found, cancelled, or has expired.",
		)
		return edit, nil
	}

	// Send completion message to group
	if err := h.ChoreReminderManager.SendCompletionToGroup(assignment); err != nil {
		log.Printf("Failed to send completion message to group: %v", err)
	}

	// Mark as completed
	h.ChoreReminderManager.CompleteChore(reminderID)

	if err := h.Store.CompleteChoreByReminderID(context.Background(), reminderID); err != nil {
		log.Printf("Failed to complete chore in database: %v", err)
	}

	// Update the message to show completion
	// Escape HTML at display time (assignment.Description is stored unescaped)
	escapedDesc := html.EscapeString(assignment.Description)
	edit := tgbotapi.NewEditMessageText(
		q.Message.Chat.ID,
		q.Message.MessageID,
		fmt.Sprintf("✅ <b>Great job!</b>\n\nYou marked the chore as completed:\n\n<i>%s</i>\n\n"+
			"The group has been notified!", escapedDesc),
	)
	edit.ParseMode = tgbotapi.ModeHTML

	return edit, nil
}

// HandleChoreRemindCallback handles the "Remind me in 15 min" button callback
func (h *Handlers) HandleChoreRemindCallback(q *tgbotapi.CallbackQuery) (tgbotapi.EditMessageTextConfig, error) {
	// Extract reminderID from callback data: "chore_remind:reminderID"
	parts := strings.Split(q.Data, ":")
	if len(parts) != 2 {
		log.Printf("Invalid callback data format: %s", q.Data)
		edit := tgbotapi.NewEditMessageText(
			q.Message.Chat.ID,
			q.Message.MessageID,
			"❌ Error: Invalid callback data.",
		)
		return edit, nil
	}

	reminderID := parts[1]

	if h.ChoreReminderManager == nil {
		edit := tgbotapi.NewEditMessageText(
			q.Message.Chat.ID,
			q.Message.MessageID,
			"❌ Error: Reminder manager not configured.",
		)
		return edit, nil
	}

	// Verify the assignment exists
	assignment, exists := h.ChoreReminderManager.GetAssignment(reminderID)
	if !exists {
		edit := tgbotapi.NewEditMessageText(
			q.Message.Chat.ID,
			q.Message.MessageID,
			"❌ This chore assignment was not found, cancelled, or has expired.",
		)
		return edit, nil
	}

	// Schedule another reminder
	h.ChoreReminderManager.ScheduleReReminder(reminderID)

	// Update the message to confirm
	// Escape HTML at display time (assignment.Description is stored unescaped)
	escapedDesc := html.EscapeString(assignment.Description)
	edit := tgbotapi.NewEditMessageText(
		q.Message.Chat.ID,
		q.Message.MessageID,
		fmt.Sprintf("⏰ <b>Reminder Scheduled</b>\n\nI'll remind you again in 15 minutes about:\n\n<i>%s</i>", escapedDesc),
	)
	edit.ParseMode = tgbotapi.ModeHTML

	return edit, nil
}
