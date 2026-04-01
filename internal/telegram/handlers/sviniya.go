package handlers

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"strconv"
	"strings"

	"github.com/korjavin/dutyassistant/internal/store"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// HandleSpend handles the /spend command - allows users to spend a sviniya with a description.
// If an argument is provided, it processes immediately.
// If no argument, checks balance and starts interactive session if balance > 0.
func (h *Handlers) HandleSpend(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	slog.Info(fmt.Sprintf("[HandleSpend] User %d (%s) triggered /spend with args: '%s'", m.From.ID, m.From.FirstName, m.CommandArguments()))

	args := strings.TrimSpace(m.CommandArguments())

	// If argument provided, process immediately without pre-checking balance
	// The atomic DecrementSviniyaBalance will handle insufficient balance
	if args != "" {
		if session, exists := h.SessionManager.GetSession(m.Chat.ID); exists {
			// Don't interrupt another user's session or a different session type
			if session.UserID != m.From.ID || session.Type != SessionTypeSpendSviniya {
				return tgbotapi.NewMessage(m.Chat.ID, "Another user has an active session in this chat. Please wait for them to finish."), nil
			}
		}
		return h.processSpend(m, args)
	}

	// No argument - get user info and balance for interactive session
	user, err := h.Store.GetUserByTelegramID(context.Background(), m.From.ID)
	if err != nil || user == nil {
		slog.Error(fmt.Sprintf("[HandleSpend] Error getting user by Telegram ID %d: %v", m.From.ID, err))
		return tgbotapi.NewMessage(m.Chat.ID, "Failed to retrieve your user information."), nil
	}

	// No argument - check for existing session before starting a new one
	if session, exists := h.SessionManager.GetSession(m.Chat.ID); exists {
		// There's an existing session - don't clobber it
		if session.UserID == m.From.ID {
			return tgbotapi.NewMessage(m.Chat.ID, "You already have an active session. Send /cancel to end it first."), nil
		}
		return tgbotapi.NewMessage(m.Chat.ID, "Another user has an active session in this chat. Please wait for them to finish."), nil
	}

	// No argument - get balance and start interactive session
	balance, err := h.Store.GetSviniyaBalance(context.Background(), user.ID)
	if err != nil {
		slog.Error(fmt.Sprintf("[HandleSpend] Error getting sviniya balance for user %d: %v", user.ID, err))
		return tgbotapi.NewMessage(m.Chat.ID, "Failed to check your sviniya balance."), nil
	}

	// Check if user has any balance before starting interactive session
	if balance == nil || balance.Balance <= 0 {
		return tgbotapi.NewMessage(m.Chat.ID, "Sorry, you have no sviniyas on your balance to spend."), nil
	}

	// Start interactive session
	h.SessionManager.StartSession(m.Chat.ID, m.From.ID, SessionTypeSpendSviniya)
	promptMsg := tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("You have %d sviniya(s). What would you like to spend it on? Send a brief description.\n\nSend /cancel to abort.", balance.Balance))
	return promptMsg, nil
}

// HandleSpendInteractive handles messages during an active spend sviniya session.
func (h *Handlers) HandleSpendInteractive(m *tgbotapi.Message) (tgbotapi.Chattable, error) {
	slog.Info(fmt.Sprintf("[HandleSpendInteractive] User %d (%s) in spend session", m.From.ID, m.From.FirstName))

	// Verify the message is from the user who started the session
	session, exists := h.SessionManager.GetSession(m.Chat.ID)
	if !exists {
		return tgbotapi.NewMessage(m.Chat.ID, "No active spend session. Use /spend to start."), nil
	}
	if session.UserID != m.From.ID {
		// Ignore messages from non-owners - don't interrupt the conversation
		return nil, nil
	}

	// Check for cancel command
	if m.IsCommand() && m.Command() == "cancel" {
		h.SessionManager.EndSession(m.Chat.ID)
		return tgbotapi.NewMessage(m.Chat.ID, "Spend sviniya cancelled."), nil
	}

	// Ensure there's text to process as description
	if m.Text == "" {
		return tgbotapi.NewMessage(m.Chat.ID, "Please send a text description of what you're spending the sviniya on, or use /cancel to abort."), nil
	}

	// Treat text as description and process
	return h.processSpend(m, m.Text)
}

// processSpend handles the actual spending logic - decrements balance and sends announcement to group.
func (h *Handlers) processSpend(m *tgbotapi.Message, description string) (tgbotapi.MessageConfig, error) {
	ctx := context.Background()

	// Get user info to get their internal ID and name
	user, err := h.Store.GetUserByTelegramID(ctx, m.From.ID)
	if err != nil || user == nil {
		slog.Error(fmt.Sprintf("[processSpend] Error getting user by Telegram ID %d: %v", m.From.ID, err))
		return tgbotapi.NewMessage(m.Chat.ID, "Failed to retrieve your user information."), nil
	}

	// Decrement balance FIRST to prevent false announcements
	err = h.Store.DecrementSviniyaBalance(ctx, user.ID)
	if err != nil {
		slog.Error(fmt.Sprintf("[processSpend] Error decrementing sviniya balance for user %d: %v", user.ID, err))
		// End the session since the operation failed
		h.SessionManager.EndSession(m.Chat.ID)
		// Check if it's an insufficient balance error
		if errors.Is(err, store.ErrInsufficientBalance) {
			return tgbotapi.NewMessage(m.Chat.ID, "Sorry, you have no sviniyas on your balance to spend."), nil
		}
		return tgbotapi.NewMessage(m.Chat.ID, "Failed to spend sviniya."), nil
	}

	// Build announcement message
	escapedName := html.EscapeString(user.FirstName)
	intent := fmt.Sprintf("User %s is spending a sviniya. Create a fun announcement.", escapedName)
	vanilla := fmt.Sprintf("%s spent a sviniya on: %s", escapedName, html.EscapeString(description))

	var announcementText string
	if h.LLMClient != nil {
		announcementText = h.LLMClient.RefineMessage(ctx, intent, vanilla)
	} else {
		announcementText = vanilla
	}

	// Send announcement to group AFTER successful decrement
	announcementSent := false
	if h.Bot != nil && h.GroupID != 0 {
		groupMsg := tgbotapi.NewMessage(h.GroupID, announcementText)
		groupMsg.ParseMode = tgbotapi.ModeHTML
		if _, err := h.Bot.Send(groupMsg); err != nil {
			slog.Error(fmt.Sprintf("[processSpend] Failed to send announcement to group %d: %v", h.GroupID, err))
			// Don't fail the spend if announcement fails - balance was already decremented
		} else {
			slog.Info(fmt.Sprintf("[processSpend] Sent sviniya spend announcement to group %d", h.GroupID))
			announcementSent = true
		}
	} else {
		slog.Warn("[processSpend] Bot or GroupID not configured, skipping group announcement")
	}

	// End session only if it's a spend session belonging to this user
	if session, exists := h.SessionManager.GetSession(m.Chat.ID); exists {
		if session.UserID == m.From.ID && session.Type == SessionTypeSpendSviniya {
			h.SessionManager.EndSession(m.Chat.ID)
		}
	}

	// Confirm to user
	if announcementSent {
		return tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("Spent 1 sviniya on: %s\n\nAnnouncement sent to the group!", description)), nil
	}
	return tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("Spent 1 sviniya on: %s", description)), nil
}

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
		escapedName := html.EscapeString(balance.UserName)

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
	if balance < 0 {
		return tgbotapi.NewMessage(m.Chat.ID, "Balance cannot be negative.\nExample: /set_sviniya_balance Ivan 3"), nil
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
