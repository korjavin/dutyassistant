package handlers

import (
	"context"
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// HandleList handles the /list command for admins. Format: /list chore
func (h *Handlers) HandleList(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	// 1. Admin check
	isAdmin, err := h.checkAdmin(m.From.ID)
	if err != nil || !isAdmin {
		return tgbotapi.NewMessage(m.Chat.ID, adminOnlyMessage), nil
	}

	args := strings.TrimSpace(m.CommandArguments())

	if strings.ToLower(args) == "chore" {
		chores, err := h.Store.GetActiveRecurringChores(context.Background())
		if err != nil {
			return tgbotapi.NewMessage(m.Chat.ID, "❌ Failed to retrieve recurring chores."), nil
		}

		if len(chores) == 0 {
			return tgbotapi.NewMessage(m.Chat.ID, "No active recurring chores found."), nil
		}

		var sb strings.Builder
		sb.WriteString("📋 <b>Active Recurring Chores:</b>\n\n")

		berlinLoc, err := time.LoadLocation("Europe/Berlin")
		if err != nil {
			berlinLoc = time.UTC // fallback
		}

		for _, chore := range chores {
			escapedDesc := html.EscapeString(chore.Description)
			nextRunStr := chore.NextRunAt.In(berlinLoc).Format("2006-01-02 15:04 MST")
			sb.WriteString(fmt.Sprintf("<b>ID:</b> <code>%d</code>\n", chore.ID))
			sb.WriteString(fmt.Sprintf("<b>Description:</b> <i>%s</i>\n", escapedDesc))
			sb.WriteString(fmt.Sprintf("<b>Interval:</b> every %d days\n", chore.Interval))
			sb.WriteString(fmt.Sprintf("<b>Next Run:</b> %s\n\n", nextRunStr))
		}

		msg := tgbotapi.NewMessage(m.Chat.ID, sb.String())
		msg.ParseMode = tgbotapi.ModeHTML
		return msg, nil
	}

	return tgbotapi.NewMessage(m.Chat.ID, "Unknown list command. Use /list chore to list periodic chores."), nil
}

// HandleCancel handles the /cancel command for admins. Format: /cancel chore <id>
func (h *Handlers) HandleCancel(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	// 1. Admin check
	isAdmin, err := h.checkAdmin(m.From.ID)
	if err != nil || !isAdmin {
		return tgbotapi.NewMessage(m.Chat.ID, adminOnlyMessage), nil
	}

	args := strings.TrimSpace(m.CommandArguments())
	parts := strings.Fields(args)

	if len(parts) == 2 && strings.ToLower(parts[0]) == "chore" {
		choreID, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return tgbotapi.NewMessage(m.Chat.ID, "❌ Invalid chore ID format. Use /cancel chore <id>"), nil
		}

		chore, err := h.Store.GetRecurringChore(context.Background(), choreID)
		if err != nil {
			return tgbotapi.NewMessage(m.Chat.ID, "❌ Failed to retrieve recurring chore."), nil
		}

		if chore == nil || !chore.IsActive {
			return tgbotapi.NewMessage(m.Chat.ID, "❌ Recurring chore not found or already cancelled."), nil
		}

		if err := h.Store.CancelRecurringChore(context.Background(), choreID); err != nil {
			return tgbotapi.NewMessage(m.Chat.ID, "❌ Failed to cancel recurring chore."), nil
		}

		msg := tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("✅ <b>Periodic chore cancelled:</b>\n<i>%s</i>", html.EscapeString(chore.Description)))
		msg.ParseMode = tgbotapi.ModeHTML
		return msg, nil
	}

	return tgbotapi.NewMessage(m.Chat.ID, "Unknown cancel command. Use /cancel chore <id>"), nil
}
