package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// HandleSviniya handles the /sviniya command - displays all user balances.
func (h *Handlers) HandleSviniya(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	slog.Info(fmt.Sprintf("[HandleSviniya] User %d (%s) triggered /sviniya", m.From.ID, m.From.FirstName))

	balances, err := h.Store.GetAllSviniyaBalances(context.Background())
	if err != nil {
		slog.Error(fmt.Sprintf("[HandleSviniya] Error getting sviniya balances: %v", err))
		return tgbotapi.NewMessage(m.Chat.ID, "Failed to fetch sviniya balances."), nil
	}

	if len(balances) == 0 {
		msg := tgbotapi.NewMessage(m.Chat.ID, "No sviniya balances found.")
		return msg, nil
	}

	var sb strings.Builder
	sb.WriteString("🐷 <b>Sviniya Balances</b>\n\n")

	for _, balance := range balances {
		escapedName := strings.ReplaceAll(balance.UserName, "<", "&lt;")
		escapedName = strings.ReplaceAll(escapedName, ">", "&gt;")
		escapedName = strings.ReplaceAll(escapedName, "&", "&amp;")

		plural := "sviniyas"
		if balance.Balance == 1 {
			plural = "sviniya"
		}
		sb.WriteString(fmt.Sprintf("%s: %d %s\n", escapedName, balance.Balance, plural))
	}

	msg := tgbotapi.NewMessage(m.Chat.ID, sb.String())
	msg.ParseMode = tgbotapi.ModeHTML
	return msg, nil
}

// HandleSetSviniyaBalance handles the /set_sviniya_balance admin command.
// Usage: /set_sviniya_balance <name> <num>
func (h *Handlers) HandleSetSviniyaBalance(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	slog.Info(fmt.Sprintf("[HandleSetSviniyaBalance] User %d (%s) triggered /set_sviniya_balance", m.From.ID, m.From.FirstName))

	isAdmin, err := h.checkAdmin(m.From.ID)
	if err != nil || !isAdmin {
		return tgbotapi.NewMessage(m.Chat.ID, adminOnlyMessage), nil
	}

	args := strings.TrimSpace(m.CommandArguments())
	if args == "" {
		return tgbotapi.NewMessage(m.Chat.ID, "Usage: /set_sviniya_balance <name> <num>\nExample: /set_sviniya_balance Ivan 3"), nil
	}

	parts := strings.SplitN(args, " ", 2)
	if len(parts) != 2 {
		return tgbotapi.NewMessage(m.Chat.ID, "Usage: /set_sviniya_balance <name> <num>\nExample: /set_sviniya_balance Ivan 3"), nil
	}

	userName := parts[0]
	balanceStr := parts[1]

	balance, err := strconv.Atoi(balanceStr)
	if err != nil {
		return tgbotapi.NewMessage(m.Chat.ID, "Invalid balance value. Please provide a number.\nExample: /set_sviniya_balance Ivan 3"), nil
	}

	user, err := h.Store.GetUserByName(context.Background(), userName)
	if err != nil {
		slog.Error(fmt.Sprintf("[HandleSetSviniyaBalance] Error getting user by name '%s': %v", userName, err))
		return tgbotapi.NewMessage(m.Chat.ID, "Failed to find user."), nil
	}
	if user == nil {
		return tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("User '%s' not found.", userName)), nil
	}

	err = h.Store.SetSviniyaBalance(context.Background(), user.ID, balance)
	if err != nil {
		slog.Error(fmt.Sprintf("[HandleSetSviniyaBalance] Error setting sviniya balance for user %d: %v", user.ID, err))
		return tgbotapi.NewMessage(m.Chat.ID, "Failed to set sviniya balance."), nil
	}

	slog.Info(fmt.Sprintf("[HandleSetSviniyaBalance] Set sviniya balance for user %d (%s) to %d", user.ID, userName, balance))
	return tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("Set sviniya balance for %s to %d.", userName, balance)), nil
}
