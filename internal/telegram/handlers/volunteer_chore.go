package handlers

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// HandleVolunteerChore shows a list of active chores not assigned to the current user,
// allowing them to volunteer to take one.
func (h *Handlers) HandleVolunteerChore(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	user, err := h.Store.GetUserByTelegramID(context.Background(), m.From.ID)
	if err != nil || user == nil {
		return tgbotapi.NewMessage(m.Chat.ID, "Could not find your user profile. Please use /start first."), nil
	}

	chores, err := h.Store.GetActiveChores(context.Background())
	if err != nil {
		slog.Error(fmt.Sprintf("[HandleVolunteerChore] Failed to get active chores: %v", err))
		return tgbotapi.NewMessage(m.Chat.ID, "Sorry, something went wrong while fetching chores."), nil
	}

	// Filter out chores already assigned to this user
	var available []*choreEntry
	for _, c := range chores {
		if c.UserID != user.ID {
			available = append(available, &choreEntry{id: c.ID, description: c.Description, assignedTo: c.User.FirstName, deadline: c.DeadlineAt})
		}
	}

	if len(available) == 0 {
		return tgbotapi.NewMessage(m.Chat.ID, "🎉 There are no chores available for you to take right now."), nil
	}

	tz := os.Getenv("CHORE_TIMEZONE")
	if tz == "" {
		tz = "Europe/Berlin"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.Local
	}

	var sb strings.Builder
	sb.WriteString("🙋 <b>Available chores to take:</b>\n\n")

	var rows [][]tgbotapi.InlineKeyboardButton
	for i, c := range available {
		deadline := c.deadline.In(loc).Format("2006-01-02")
		sb.WriteString(fmt.Sprintf("%d. <i>%s</i>\n   Assigned to: %s | Deadline: %s\n\n",
			i+1,
			html.EscapeString(c.description),
			html.EscapeString(c.assignedTo),
			deadline,
		))
		rows = append(rows, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("%d. %s", i+1, truncate(c.description, 30)),
				fmt.Sprintf("vc_select:%d", c.id),
			),
		})
	}

	rows = append(rows, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("❌ Cancel", "vc_cancel"),
	})

	sb.WriteString("Select a chore below to volunteer for it:")

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
	msg := tgbotapi.NewMessage(m.Chat.ID, sb.String())
	msg.ParseMode = tgbotapi.ModeHTML
	msg.ReplyMarkup = keyboard
	return msg, nil
}

// HandleVolunteerChoreSelectCallback handles a chore selection and shows confirmation.
func (h *Handlers) HandleVolunteerChoreSelectCallback(q *tgbotapi.CallbackQuery) (tgbotapi.EditMessageTextConfig, error) {
	parts := strings.SplitN(q.Data, ":", 2)
	if len(parts) != 2 {
		return tgbotapi.EditMessageTextConfig{}, fmt.Errorf("invalid callback data")
	}

	choreID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		edit := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, "❌ Invalid chore selection.")
		return edit, nil
	}

	// Verify caller exists
	user, err := h.Store.GetUserByTelegramID(context.Background(), q.From.ID)
	if err != nil || user == nil {
		edit := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, "❌ Could not find your user profile. Please use /start first.")
		return edit, nil
	}

	chore, err := h.Store.GetChoreByID(context.Background(), choreID)
	if err != nil || chore == nil {
		edit := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, "❌ Chore not found or already completed.")
		return edit, nil
	}

	// Prevent taking own chore
	if chore.UserID == user.ID {
		edit := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, "⚠️ This chore is already assigned to you.")
		return edit, nil
	}

	tz := os.Getenv("CHORE_TIMEZONE")
	if tz == "" {
		tz = "Europe/Berlin"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.Local
	}

	deadline := chore.DeadlineAt.In(loc).Format("2006-01-02")
	text := fmt.Sprintf(
		"🙋 <b>Confirm taking this chore:</b>\n\n"+
			"<i>%s</i>\n\n"+
			"Currently assigned to: <b>%s</b>\n"+
			"Deadline: <b>%s</b>\n\n"+
			"Do you want to reassign this chore to yourself?",
		html.EscapeString(chore.Description),
		html.EscapeString(chore.User.FirstName),
		deadline,
	)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		[]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("✅ Yes, take it", fmt.Sprintf("vc_confirm:%d", choreID)),
			tgbotapi.NewInlineKeyboardButtonData("❌ Cancel", "vc_cancel"),
		},
	)

	edit := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, text)
	edit.ParseMode = tgbotapi.ModeHTML
	edit.ReplyMarkup = &keyboard
	return edit, nil
}

// HandleVolunteerChoreConfirmCallback handles the confirmation of taking a chore.
func (h *Handlers) HandleVolunteerChoreConfirmCallback(q *tgbotapi.CallbackQuery) (tgbotapi.EditMessageTextConfig, error) {
	parts := strings.SplitN(q.Data, ":", 2)
	if len(parts) != 2 {
		return tgbotapi.EditMessageTextConfig{}, fmt.Errorf("invalid callback data")
	}

	choreID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		edit := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, "❌ Invalid chore selection.")
		return edit, nil
	}

	user, err := h.Store.GetUserByTelegramID(context.Background(), q.From.ID)
	if err != nil || user == nil {
		edit := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, "❌ Could not find your user profile. Please use /start first.")
		return edit, nil
	}

	chore, err := h.Store.GetChoreByID(context.Background(), choreID)
	if err != nil || chore == nil {
		edit := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, "❌ Chore not found or already completed.")
		return edit, nil
	}

	if chore.UserID == user.ID {
		edit := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, "⚠️ This chore is already assigned to you.")
		return edit, nil
	}

	previousAssignee := chore.User.FirstName

	if err := h.Store.UpdateChoreUserID(context.Background(), choreID, user.ID); err != nil {
		slog.Error(fmt.Sprintf("[HandleVolunteerChoreConfirmCallback] Failed to reassign chore %d to user %d: %v", choreID, user.ID, err))
		edit := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, "❌ Failed to take chore. It may have been completed or cancelled.")
		return edit, nil
	}

	// Keep the in-memory reminder state consistent so future DMs and
	// completion callbacks reach the new assignee, not the old one.
	if h.ChoreReminderManager != nil {
		h.ChoreReminderManager.ReassignChore(chore.ReminderID, user.TelegramUserID, user.FirstName)
	}

	slog.Info(fmt.Sprintf("[HandleVolunteerChoreConfirmCallback] User %d (%s) took chore %d from %s", user.ID, user.FirstName, choreID, previousAssignee))

	text := fmt.Sprintf(
		"✅ <b>Chore assigned to you!</b>\n\n"+
			"<i>%s</i>\n\n"+
			"You have successfully taken this chore from <b>%s</b>.\n"+
			"Good luck! 💪",
		html.EscapeString(chore.Description),
		html.EscapeString(previousAssignee),
	)

	edit := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, text)
	edit.ParseMode = tgbotapi.ModeHTML
	return edit, nil
}

// HandleVolunteerChoreCancelCallback handles the cancellation of the volunteer chore flow.
func (h *Handlers) HandleVolunteerChoreCancelCallback(q *tgbotapi.CallbackQuery) (tgbotapi.EditMessageTextConfig, error) {
	edit := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, "❌ Cancelled.")
	return edit, nil
}

// choreEntry is a helper struct for building the chore list UI.
type choreEntry struct {
	id          int64
	description string
	assignedTo  string
	deadline    time.Time
}

// truncate shortens a string to maxLen runes, appending "…" if truncated.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "…"
}
