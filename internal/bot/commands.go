package bot

import (
	"context"
	"fmt"
	"log"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (b *Bot) handleStartHelp(msg *tgbotapi.Message) {
	text := "🤖 *Welcome to the Duty Assistant Bot!*\n\n" +
		"Here are the commands you can use:\n\n" +
		"👤 *User Commands:*\n" +
		"/status - View your duty statistics and queue status\n" +
		"/schedule - View the current month's duty schedule\n" +
		"/volunteer - Volunteer for duty (interactive)\n" +
		"/chore - View your assigned active chores\n" +
		"/explain - Explain how the most recent dish hero duty was assigned\n\n" +
		"🛠 *Admin Commands:*\n" +
		"/assign - Assign days to a user's admin queue\n" +
		"/unassign - Remove days from a user's admin queue\n" +
		"/chore <desc> [/<N>d] - Create a one-off or periodic chore\n" +
		"/list - View active periodic or regular chores\n" +
		"/cancel - Cancel a duty or chore\n" +
		"/complete - Mark an active chore as completed\n" +
		"/modify (or /change) - Change duty assignment for a date\n" +
		"/offduty - Set off-duty period for a user\n" +
		"/toggleactive - Toggle user active/inactive status\n" +
		"/vacation - Toggle vacation mode to pause assignments\n" +
		"/users - List all users with their queues and status\n" +
		"/ratings - Show the current month's participant rating calendar\n"

	reply := tgbotapi.NewMessage(msg.Chat.ID, text)
	reply.ParseMode = tgbotapi.ModeMarkdown
	b.api.Send(reply)
}

func (b *Bot) handleStatus(msg *tgbotapi.Message) {
	user, err := b.repo.GetUserByTelegramID(context.Background(), msg.From.ID)
	if err != nil || user == nil {
		reply := tgbotapi.NewMessage(msg.Chat.ID, "User not found.")
		b.api.Send(reply)
		return
	}

	stats, err := b.repo.GetUserStats(context.Background(), user.ID)
	if err != nil {
		log.Printf("Failed to get stats: %v", err)
		return
	}

	text := fmt.Sprintf("📊 *Your Status:*\n\nTotal Duties: %d\nDuties This Month: %d\nNext Duty Date: %s", stats.TotalDuties, stats.DutiesThisMonth, stats.NextDutyDate)
	reply := tgbotapi.NewMessage(msg.Chat.ID, text)
	reply.ParseMode = tgbotapi.ModeMarkdown
	b.api.Send(reply)
}

func (b *Bot) handleUsers(msg *tgbotapi.Message) {
	users, err := b.repo.ListAllUsers(context.Background())
	if err != nil {
		reply := tgbotapi.NewMessage(msg.Chat.ID, "Failed to retrieve users.")
		b.api.Send(reply)
		return
	}

	text := "👥 *Users:*\n"
	for _, u := range users {
		activeStr := "❌ Inactive"
		if u.IsActive {
			activeStr = "✅ Active"
		}
		text += fmt.Sprintf("• %s - %s\n", u.FirstName, activeStr)
	}

	reply := tgbotapi.NewMessage(msg.Chat.ID, text)
	reply.ParseMode = tgbotapi.ModeMarkdown
	b.api.Send(reply)
}

func (b *Bot) handleSchedule(msg *tgbotapi.Message) {
	now := time.Now()
	duties, err := b.dutyService.GetSchedule(context.Background(), now.Year(), now.Month())
	if err != nil {
		reply := tgbotapi.NewMessage(msg.Chat.ID, "Failed to retrieve schedule.")
		b.api.Send(reply)
		return
	}

	text := fmt.Sprintf("📅 *Schedule for %s %d:*\n\n", now.Month().String(), now.Year())
	for _, d := range duties {
		if d.User != nil {
			text += fmt.Sprintf("%s: %s\n", d.DutyDate.Format("2006-01-02"), d.User.FirstName)
		}
	}
	if len(duties) == 0 {
		text += "No duties scheduled."
	}

	reply := tgbotapi.NewMessage(msg.Chat.ID, text)
	reply.ParseMode = tgbotapi.ModeMarkdown
	b.api.Send(reply)
}

// Add simple static response for remaining admin commands mapping to FSM integration
func (b *Bot) handleLegacyStub(msg *tgbotapi.Message, command string) {
	reply := tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf("Command /%s is currently available via the Web interface during FSM migration.", command))
	b.api.Send(reply)
}
