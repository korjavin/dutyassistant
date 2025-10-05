package handlers

import (
	"context"
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	volunteerSuccessMessage      = "Thank you for volunteering! Added %d day(s) to your volunteer queue."
	volunteerFailureMessage      = "Sorry, we couldn't process your volunteer request. Error: %v"
	volunteerUserNotFoundMessage = "Could not find your user profile. Please use /start first."
)

// HandleVolunteer allows a user to volunteer for duty. Format: /volunteer [days]
func (h *Handlers) HandleVolunteer(m *tgbotapi.Message) (tgbotapi.MessageConfig, error) {
	args := m.CommandArguments()

	// If no arguments provided, show helpful prompt
	if strings.TrimSpace(args) == "" {
		msg := tgbotapi.NewMessage(m.Chat.ID,
			"🙋 <b>Volunteer for duty!</b>\n\n"+
			"How many days would you like to volunteer for?\n\n"+
			"Usage: <code>/volunteer days</code>\n\n"+
			"Examples:\n"+
			"  • <code>/volunteer 1</code> - volunteer for 1 day\n"+
			"  • <code>/volunteer 3</code> - volunteer for 3 days")
		msg.ParseMode = tgbotapi.ModeHTML
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