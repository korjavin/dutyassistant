package handlers

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/store"
)

const (
	registrationSentMessage     = "👋 Hi! Your request to join has been sent to the admin. I'll message you once it's approved."
	registrationPendingMessage  = "⏳ Your request to join is still waiting for the admin's approval."
	registrationRejectedMessage = "Sorry, your request to join was declined. Please contact the admin if this is a mistake."
	registrationApprovedMessage = "🎉 The admin approved you, welcome!\n\n" + startMessage

	// renotifyAfter is how long a pending request waits before another /start
	// reminds the admin about it (in case the first notification was missed).
	renotifyAfter = time.Hour
)

// requestRegistration handles /start from an unknown user: instead of
// creating them, it records a request and asks the admin to approve it.
func (h *Handlers) requestRegistration(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	ctx := context.Background()
	req, err := h.Store.GetRegistrationRequest(ctx, m.From.ID)
	if err != nil {
		slog.Error(fmt.Sprintf("[Registration] Error getting request for %d: %v", m.From.ID, err))
		return tgbotapi.MessageConfig{}, fmt.Errorf("database error: %w", err)
	}

	if req != nil && req.Status == store.RegistrationRejected {
		return tgbotapi.NewMessage(m.Chat.ID, registrationRejectedMessage), nil
	}
	if req != nil && TimeNow().Sub(req.RequestedAt) < renotifyAfter {
		return tgbotapi.NewMessage(m.Chat.ID, registrationPendingMessage), nil
	}

	req = &store.RegistrationRequest{
		TelegramUserID: m.From.ID,
		FirstName:      m.From.FirstName,
		LastName:       m.From.LastName,
		Username:       m.From.UserName,
		Status:         store.RegistrationPending,
		RequestedAt:    TimeNow(),
	}
	if err := h.Store.UpsertRegistrationRequest(ctx, req); err != nil {
		slog.Error(fmt.Sprintf("[Registration] Error saving request for %d: %v", m.From.ID, err))
		return tgbotapi.MessageConfig{}, fmt.Errorf("failed to save registration request: %w", err)
	}
	h.notifyAdminOfRegistration(req)
	slog.Info(fmt.Sprintf("[Registration] Request from %d (%s) sent to admin", m.From.ID, m.From.FirstName))
	return tgbotapi.NewMessage(m.Chat.ID, registrationSentMessage), nil
}

func (h *Handlers) notifyAdminOfRegistration(req *store.RegistrationRequest) {
	if h.Bot == nil || h.AdminID == 0 {
		slog.Warn("[Registration] Bot or admin not configured, cannot ask for approval")
		return
	}
	msg := tgbotapi.NewMessage(h.AdminID, "🆕 <b>New user wants to join</b>\n\n"+describeRegistration(req)+"\n\nApprove only people from your family.")
	msg.ParseMode = tgbotapi.ModeHTML
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("✅ Approve", fmt.Sprintf("reg_ok:%d", req.TelegramUserID)),
		tgbotapi.NewInlineKeyboardButtonData("❌ Reject", fmt.Sprintf("reg_no:%d", req.TelegramUserID)),
	))
	if _, err := h.Bot.Send(msg); err != nil {
		slog.Error(fmt.Sprintf("[Registration] Failed to notify admin: %v", err))
	}
}

// describeRegistration renders who is asking, as HTML.
func describeRegistration(req *store.RegistrationRequest) string {
	name := strings.TrimSpace(req.FirstName + " " + req.LastName)
	var sb strings.Builder
	fmt.Fprintf(&sb, "Name: <b>%s</b>\n", html.EscapeString(name))
	if req.Username != "" {
		fmt.Fprintf(&sb, "Username: @%s\n", html.EscapeString(req.Username))
	}
	fmt.Fprintf(&sb, "Telegram ID: <code>%d</code>", req.TelegramUserID)
	return sb.String()
}

// HandleRegistrationCallback handles the admin's Approve/Reject buttons
// ("reg_ok:<telegramID>" / "reg_no:<telegramID>").
func (h *Handlers) HandleRegistrationCallback(q *tgbotapi.CallbackQuery) (tgbotapi.Chattable, error) {
	if q.From == nil || q.Message == nil || !h.IsAdmin(q.From.ID) {
		return nil, nil // Ignore non-admin clicks silently
	}
	parts := strings.Split(q.Data, ":")
	if len(parts) != 2 {
		return nil, nil
	}
	telegramID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return nil, nil
	}
	approve := parts[0] == "reg_ok"

	edit := func(text string) tgbotapi.Chattable {
		e := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, text)
		e.ParseMode = tgbotapi.ModeHTML
		return e
	}

	ctx := context.Background()
	req, err := h.Store.GetRegistrationRequest(ctx, telegramID)
	if err != nil {
		slog.Error(fmt.Sprintf("[Registration] Error getting request for %d: %v", telegramID, err))
		return edit("❌ Failed to load the request."), nil
	}
	if req == nil || req.Status != store.RegistrationPending {
		return edit("This request was already handled."), nil
	}
	who := describeRegistration(req)

	if !approve {
		req.Status = store.RegistrationRejected
		if err := h.Store.UpsertRegistrationRequest(ctx, req); err != nil {
			slog.Error(fmt.Sprintf("[Registration] Error rejecting %d: %v", telegramID, err))
			return edit("❌ Failed to reject the request."), nil
		}
		h.dmUser(telegramID, registrationRejectedMessage)
		slog.Info(fmt.Sprintf("[Registration] Admin rejected %d", telegramID))
		return edit("❌ <b>Rejected</b>\n\n" + who), nil
	}

	existing, err := h.Store.GetUserByTelegramID(ctx, telegramID)
	if err != nil {
		slog.Error(fmt.Sprintf("[Registration] Error checking user %d: %v", telegramID, err))
		return edit("❌ Failed to approve the request."), nil
	}
	if existing == nil {
		newUser := &store.User{TelegramUserID: telegramID, FirstName: req.FirstName, IsActive: true}
		if err := h.Store.CreateUser(ctx, newUser); err != nil {
			slog.Error(fmt.Sprintf("[Registration] Failed to create user %d: %v", telegramID, err))
			return edit("❌ Failed to create the user."), nil
		}
	}
	if err := h.Store.DeleteRegistrationRequest(ctx, telegramID); err != nil {
		slog.Error(fmt.Sprintf("[Registration] Failed to delete request %d: %v", telegramID, err))
	}
	h.dmUser(telegramID, registrationApprovedMessage)
	slog.Info(fmt.Sprintf("[Registration] Admin approved %d", telegramID))
	return edit("✅ <b>Approved</b> — now an active participant (use /toggle_active to exclude them from duties).\n\n" + who), nil
}

func (h *Handlers) dmUser(telegramID int64, text string) {
	if h.Bot == nil {
		return
	}
	if _, err := h.Bot.Send(tgbotapi.NewMessage(telegramID, text)); err != nil {
		slog.Error(fmt.Sprintf("[Registration] Failed to message user %d: %v", telegramID, err))
	}
}
