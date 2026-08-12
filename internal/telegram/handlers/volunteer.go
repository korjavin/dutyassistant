package handlers

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/korjavin/dutyassistant/internal/scheduler"
)

const (
	volunteerSuccessMessage      = "Thank you for volunteering! Added %d day(s) to your volunteer queue."
	volunteerFailureMessage      = "Sorry, we couldn't process your volunteer request. Error: %v"
	volunteerUserNotFoundMessage = "Could not find your user profile. Please use /start first."
)

// HandleVolunteer allows a user to volunteer for duty. Format: /volunteer [days]
func (h *Handlers) HandleVolunteer(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	args := m.CommandArguments()

	// If no arguments provided, show inline keyboard with day options
	if strings.TrimSpace(args) == "" {
		var buttons [][]tgbotapi.InlineKeyboardButton
		row := []tgbotapi.InlineKeyboardButton{}
		for days := 1; days <= 7; days++ {
			row = append(row, tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("%d", days),
				fmt.Sprintf("volunteer_days:%d", days),
			))
			if days%4 == 0 || days == 7 {
				buttons = append(buttons, row)
				row = []tgbotapi.InlineKeyboardButton{}
			}
		}
		// Add custom option
		buttons = append(buttons, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("✏️ Custom", "volunteer_custom"),
		})

		keyboard := tgbotapi.NewInlineKeyboardMarkup(buttons...)
		msg := tgbotapi.NewMessage(m.Chat.ID, "🙋 <b>Volunteer for duty!</b>\n\nHow many days would you like to volunteer for?")
		msg.ParseMode = tgbotapi.ModeHTML
		msg.ReplyMarkup = keyboard
		return msg, nil
	}

	var days int
	_, err := fmt.Sscanf(args, "%d", &days)
	if err != nil || days <= 0 {
		msg := tgbotapi.NewMessage(m.Chat.ID,
			fmt.Sprintf("⚠️ '%s' is not a valid number of days.\n\n"+
				"Please use a positive number.\n\n"+
				"Example: <code>/volunteer 3</code>", args))
		msg.ParseMode = tgbotapi.ModeHTML
		return msg, nil
	}

	user, err := h.Store.GetUserByTelegramID(context.Background(), m.From.ID)
	if err != nil || user == nil {
		return tgbotapi.NewMessage(m.Chat.ID, volunteerUserNotFoundMessage), nil
	}

	err = h.Scheduler.VolunteerForDuty(context.Background(), user, days)
	if err != nil {
		return tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("❌ "+volunteerFailureMessage, err)), nil
	}

	return tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("✅ "+volunteerSuccessMessage, days)), nil
}

// HandleVolunteerDaysCallback handles the callback when days are selected from inline keyboard
func (h *Handlers) HandleVolunteerDaysCallback(q *tgbotapi.CallbackQuery) (tgbotapi.EditMessageTextConfig, error) {
	parts := strings.Split(q.Data, ":")
	if len(parts) != 2 {
		return tgbotapi.EditMessageTextConfig{}, fmt.Errorf("invalid callback data")
	}

	var days int
	if _, err := fmt.Sscanf(parts[1], "%d", &days); err != nil {
		return tgbotapi.EditMessageTextConfig{}, fmt.Errorf("invalid callback data: %v", err)
	}

	user, err := h.Store.GetUserByTelegramID(context.Background(), q.From.ID)
	if err != nil || user == nil {
		edit := tgbotapi.NewEditMessageText(q.Message.Chat.ID, q.Message.MessageID, "❌ "+volunteerUserNotFoundMessage)
		return edit, nil
	}

	err = h.Scheduler.VolunteerForDuty(context.Background(), user, days)
	if err != nil {
		edit := tgbotapi.NewEditMessageText(
			q.Message.Chat.ID,
			q.Message.MessageID,
			fmt.Sprintf("❌ "+volunteerFailureMessage, err),
		)
		return edit, nil
	}

	edit := tgbotapi.NewEditMessageText(
		q.Message.Chat.ID,
		q.Message.MessageID,
		fmt.Sprintf("✅ "+volunteerSuccessMessage, days),
	)
	return edit, nil
}

// HandleSetVolunteer lets an admin set a participant's volunteer queue directly.
// Format: /set_volunteer <name> <days>
func (h *Handlers) HandleSetVolunteer(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	isAdmin, err := h.checkAdmin(m.From.ID)
	if err != nil || !isAdmin {
		return tgbotapi.NewMessage(m.Chat.ID, adminOnlyMessage), nil
	}

	usage := fmt.Sprintf("Usage: /set_volunteer <name> <days>\nExample: <code>/set_volunteer Ivan 3</code> (0-%d)", scheduler.MaxVolunteerQueueDays)
	args := strings.Fields(m.CommandArguments())
	if len(args) != 2 {
		msg := tgbotapi.NewMessage(m.Chat.ID, usage)
		msg.ParseMode = tgbotapi.ModeHTML
		return msg, nil
	}

	days, err := strconv.Atoi(args[1])
	if err != nil || days < 0 || days > scheduler.MaxVolunteerQueueDays {
		msg := tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("⚠️ Days must be a number between 0 and %d.\n\n%s", scheduler.MaxVolunteerQueueDays, usage))
		msg.ParseMode = tgbotapi.ModeHTML
		return msg, nil
	}

	user, err := h.Store.GetUserByName(context.Background(), args[0])
	if err != nil || user == nil {
		return tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("❌ "+userNotFoundMessage, args[0])), nil
	}

	user.VolunteerQueueDays = days
	if err := h.Store.UpdateUser(context.Background(), user); err != nil {
		return tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("❌ Failed to update volunteer queue: %v", err)), nil
	}

	return tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("✅ Volunteer queue for %s set to %d day(s).", user.FirstName, days)), nil
}

// HandleVolunteerCustomCallback handles the custom day input request
func (h *Handlers) HandleVolunteerCustomCallback(q *tgbotapi.CallbackQuery) (tgbotapi.EditMessageTextConfig, error) {
	edit := tgbotapi.NewEditMessageText(
		q.Message.Chat.ID,
		q.Message.MessageID,
		"🙋 <b>Volunteer for duty!</b>\n\nPlease type the number of days:\n\n<code>/volunteer [days]</code>",
	)
	edit.ParseMode = tgbotapi.ModeHTML
	return edit, nil
}
