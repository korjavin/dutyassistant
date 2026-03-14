package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/store"
)

// HandleCancelInteractive handles the interactive cancel menu for admins
func (h *Handlers) HandleCancelInteractive(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	chores, err := h.Store.ListActiveChores(context.Background())
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to get active chores for cancel menu: %v", err))
		return tgbotapi.NewMessage(m.Chat.ID, "❌ Failed to load items to cancel."), nil
	}

	rChores, err := h.Store.GetActiveRecurringChores(context.Background())
	if err != nil {
		slog.Error(fmt.Sprintf("Failed to get active recurring chores for cancel menu: %v", err))
		return tgbotapi.NewMessage(m.Chat.ID, "❌ Failed to load items to cancel."), nil
	}

	// Also get future scheduled duties (from today onwards)
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// We might fetch duties for current and next month to be safe
	duties, err := h.Store.GetDutiesByMonth(context.Background(), today.Year(), today.Month())
	var upcomingDuties []*store.Duty
	if err == nil {
		for _, d := range duties {
			if !d.DutyDate.Before(today) {
				upcomingDuties = append(upcomingDuties, d)
			}
		}
	}

	nextMonth := today.AddDate(0, 1, 0)
	nextMonthDuties, errNext := h.Store.GetDutiesByMonth(context.Background(), nextMonth.Year(), nextMonth.Month())
	if errNext == nil {
		for _, d := range nextMonthDuties {
			upcomingDuties = append(upcomingDuties, d)
		}
	}

	if len(chores) == 0 && len(rChores) == 0 && len(upcomingDuties) == 0 {
		return tgbotapi.NewMessage(m.Chat.ID, "✅ There are no active chores, recurring chores, or upcoming duties to cancel right now."), nil
	}

	var keyboard [][]tgbotapi.InlineKeyboardButton

	// Add future duties
	for _, d := range upcomingDuties {
		if d.User == nil {
			continue // skip orphan records
		}
		dateStr := d.DutyDate.Format("Jan 02")
		btnText := fmt.Sprintf("Duty: %s on %s", d.User.FirstName, dateStr)
		cbData := fmt.Sprintf("cancel_assignment:D%d:%s", d.UserID, d.DutyDate.Format("2006-01-02"))
		keyboard = append(keyboard, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(btnText, cbData)))
	}

	for _, c := range chores {
		desc := c.Description
		if len(desc) > 30 {
			desc = desc[:27] + "..."
		}
		btnText := fmt.Sprintf("Task: %s", desc)
		cbData := fmt.Sprintf("cancel_assignment:A%d", c.ID)
		keyboard = append(keyboard, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(btnText, cbData)))
	}

	for _, r := range rChores {
		desc := r.Description
		if len(desc) > 30 {
			desc = desc[:27] + "..."
		}
		btnText := fmt.Sprintf("Recurring: %s", desc)
		cbData := fmt.Sprintf("cancel_assignment:R%d", r.ID)
		keyboard = append(keyboard, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(btnText, cbData)))
	}

	keyboard = append(keyboard, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("❌ Cancel", "cancel_flow")))
	markup := tgbotapi.NewInlineKeyboardMarkup(keyboard...)

	msg := tgbotapi.NewMessage(m.Chat.ID, "Select an item to cancel:")
	msg.ReplyMarkup = &markup
	return msg, nil
}

// HandleCancelAssignmentCallback handles the confirmation prompt for cancelling an assignment
func (h *Handlers) HandleCancelAssignmentCallback(q *tgbotapi.CallbackQuery) (tgbotapi.Chattable, error) {
	isAdmin, err := h.checkAdmin(q.From.ID)
	if err != nil || !isAdmin {
		return nil, nil // Ignore non-admin clicks silently
	}

	parts := strings.Split(q.Data, ":")
	if len(parts) < 2 {
		return nil, nil
	}

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Confirm", strings.Replace(q.Data, "cancel_assignment", "cancel_assignment_confirm", 1)),
			tgbotapi.NewInlineKeyboardButtonData("❌ Cancel", "cancel_flow"),
		),
	)

	var prompt string
	if strings.HasPrefix(parts[1], "D") {
		// Duty cancellation D<UserID>:<Date>
		if len(parts) < 3 {
			return nil, nil
		}
		dateStr := parts[2]
		prompt = fmt.Sprintf("Are you sure you want to cancel the duty on %s?", dateStr)
	} else if strings.HasPrefix(parts[1], "A") {
		prompt = fmt.Sprintf("Are you sure you want to cancel task %s?", parts[1])
	} else if strings.HasPrefix(parts[1], "R") {
		prompt = fmt.Sprintf("Are you sure you want to cancel recurring chore %s?", parts[1])
	} else {
		prompt = "Are you sure you want to cancel this item?"
	}

	editMsg := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, prompt)
	editMsg.ReplyMarkup = &keyboard
	return editMsg, nil
}

// HandleCancelAssignmentConfirmCallback actually cancels the assignment
func (h *Handlers) HandleCancelAssignmentConfirmCallback(q *tgbotapi.CallbackQuery) (tgbotapi.Chattable, error) {
	isAdmin, err := h.checkAdmin(q.From.ID)
	if err != nil || !isAdmin {
		return nil, nil // Ignore non-admin clicks silently
	}

	parts := strings.Split(q.Data, ":")
	if len(parts) < 2 {
		return nil, nil
	}

	idStr := parts[1]
	var actionErr error
	var msgText string

	if strings.HasPrefix(idStr, "D") {
		// Cancel duty
		if len(parts) < 3 {
			return nil, nil
		}
		dateStr := parts[2]
		date, parseErr := time.Parse("2006-01-02", dateStr)
		if parseErr != nil {
			actionErr = parseErr
		} else {
			actionErr = h.Store.DeleteDuty(context.Background(), date)
			if actionErr == nil {
				msgText = fmt.Sprintf("✅ Duty on %s cancelled successfully.", dateStr)
			}
		}
	} else if strings.HasPrefix(idStr, "R") {
		// Cancel recurring chore
		id, parseErr := strconv.Atoi(strings.TrimPrefix(idStr, "R"))
		if parseErr == nil {
			actionErr = h.Store.CancelRecurringChore(context.Background(), int64(id))
			if actionErr == nil {
				msgText = "✅ Periodic chore cancelled successfully."
			}
		} else {
			actionErr = parseErr
		}
	} else if strings.HasPrefix(idStr, "A") {
		// Cancel active chore
		id, parseErr := strconv.Atoi(strings.TrimPrefix(idStr, "A"))
		if parseErr == nil {
			chore, cancelErr := h.Store.CancelChore(context.Background(), int64(id))
			actionErr = cancelErr
			if actionErr == nil {
				if h.ChoreReminderManager != nil && chore != nil && chore.ReminderID != "" {
					h.ChoreReminderManager.CancelChore(chore.ReminderID)
				}
				msgText = "✅ Regular chore cancelled successfully."
			}
		} else {
			actionErr = parseErr
		}
	}

	if actionErr != nil {
		slog.Error(fmt.Sprintf("Error cancelling item %s: %v", idStr, actionErr))
		msgText = "❌ Failed to cancel the item."
	}

	editMsg := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, msgText)
	editMsg.ReplyMarkup = nil
	return editMsg, nil
}
