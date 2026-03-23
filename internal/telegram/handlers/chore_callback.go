package handlers

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// HandleChoreDoneCallback handles the "I've done it" button callback
func (h *Handlers) HandleChoreDoneCallback(q *tgbotapi.CallbackQuery) (tgbotapi.EditMessageTextConfig, error) {
	// Extract reminderID from callback data: "chore_done:reminderID"
	parts := strings.Split(q.Data, ":")
	if len(parts) != 2 {
		slog.Info(fmt.Sprintf("Invalid callback data format: %s", q.Data))
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

	// Only the current assignee may mark the chore done.
	if q.From.ID != assignment.UserID {
		edit := tgbotapi.NewEditMessageText(
			q.Message.Chat.ID,
			q.Message.MessageID,
			"⚠️ This chore is no longer assigned to you and cannot be completed from this message.",
		)
		return edit, nil
	}

	// Send completion message to group
	if err := h.ChoreReminderManager.SendCompletionToGroup(assignment); err != nil {
		slog.Error(fmt.Sprintf("Failed to send completion message to group: %v", err))
	}

	// Mark as completed
	h.ChoreReminderManager.CompleteChore(reminderID)

	if err := h.Store.CompleteChoreByReminderID(context.Background(), reminderID); err != nil {
		slog.Error(fmt.Sprintf("Failed to complete chore in database: %v", err))
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

// HandleChoreListDoneCallback handles the "Mark as Done" button callback from the /chore list
func (h *Handlers) HandleChoreListDoneCallback(q *tgbotapi.CallbackQuery) (tgbotapi.EditMessageTextConfig, error) {
	parts := strings.Split(q.Data, ":")
	if len(parts) != 2 {
		slog.Info(fmt.Sprintf("Invalid callback data format: %s", q.Data))
		edit := tgbotapi.NewEditMessageText(
			q.Message.Chat.ID,
			q.Message.MessageID,
			"❌ Error: Invalid callback data.",
		)
		return edit, nil
	}

	choreIDStr := parts[1]
	var choreID int64
	if _, err := fmt.Sscanf(choreIDStr, "%d", &choreID); err != nil {
		slog.Error(fmt.Sprintf("Failed to parse chore ID from %s: %v", choreIDStr, err))
		edit := tgbotapi.NewEditMessageText(
			q.Message.Chat.ID,
			q.Message.MessageID,
			"❌ Error: Invalid chore ID.",
		)
		return edit, nil
	}

	ctx := context.Background()

	// Get chore
	chore, err := h.Store.GetChoreByID(ctx, choreID)
	if err != nil || chore == nil {
		edit := tgbotapi.NewEditMessageText(
			q.Message.Chat.ID,
			q.Message.MessageID,
			"❌ This chore assignment was not found, cancelled, or has expired.",
		)
		return edit, nil
	}

	// Check if already completed or cancelled
	if chore.CompletedAt != nil || chore.CancelledAt != nil {
		edit := tgbotapi.NewEditMessageText(
			q.Message.Chat.ID,
			q.Message.MessageID,
			"❌ This chore has already been completed or cancelled.",
		)
		return edit, nil
	}

	// Get user
	user, err := h.Store.GetUserByTelegramID(ctx, q.From.ID)
	if err != nil || user == nil {
		edit := tgbotapi.NewEditMessageText(
			q.Message.Chat.ID,
			q.Message.MessageID,
			"❌ Error: User not found.",
		)
		return edit, nil
	}

	// Only the assigned user can mark it as done
	if user.ID != chore.UserID {
		edit := tgbotapi.NewEditMessageText(
			q.Message.Chat.ID,
			q.Message.MessageID,
			"⚠️ This chore is no longer assigned to you and cannot be completed from this message.",
		)
		return edit, nil
	}

	// Complete chore
	if err := h.Store.CompleteChoreByReminderID(ctx, chore.ReminderID); err != nil {
		slog.Error(fmt.Sprintf("Failed to complete chore in database: %v", err))
		edit := tgbotapi.NewEditMessageText(
			q.Message.Chat.ID,
			q.Message.MessageID,
			"❌ Error: Failed to update chore status.",
		)
		return edit, nil
	}

	if h.ChoreReminderManager != nil {
		h.ChoreReminderManager.CompleteChore(chore.ReminderID)

		// Notify group
		assignment := &ChoreAssignment{
			UserID:      q.From.ID,
			UserName:    q.From.FirstName,
			Description: chore.Description,
			GroupID:     h.GroupID,
		}
		if err := h.ChoreReminderManager.SendCompletionToGroup(assignment); err != nil {
			slog.Error(fmt.Sprintf("Failed to send completion message to group: %v", err))
		}
	}

	escapedDesc := html.EscapeString(chore.Description)
	edit := tgbotapi.NewEditMessageText(
		q.Message.Chat.ID,
		q.Message.MessageID,
		fmt.Sprintf("✅ <b>Great job!</b>\n\nYou marked the chore as completed:\n\n<i>%s</i>\n\nThe group has been notified!", escapedDesc),
	)
	edit.ParseMode = tgbotapi.ModeHTML

	return edit, nil
}

// HandleChoreRemindCallback handles the "Remind me in 15 min" button callback
func (h *Handlers) HandleChoreRemindCallback(q *tgbotapi.CallbackQuery) (tgbotapi.EditMessageTextConfig, error) {
	// Extract reminderID from callback data: "chore_remind:reminderID"
	parts := strings.Split(q.Data, ":")
	if len(parts) != 2 {
		slog.Info(fmt.Sprintf("Invalid callback data format: %s", q.Data))
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

	// Only the current assignee may snooze their reminder.
	if q.From.ID != assignment.UserID {
		edit := tgbotapi.NewEditMessageText(
			q.Message.Chat.ID,
			q.Message.MessageID,
			"⚠️ This chore is no longer assigned to you.",
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
