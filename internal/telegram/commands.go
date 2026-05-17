package telegram

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// userCommands are the commands surfaced in autocomplete for all private chats
// with the bot (regular users in DM).
var userCommands = []tgbotapi.BotCommand{
	{Command: "start", Description: "Register and show the welcome message"},
	{Command: "help", Description: "Show available commands"},
	{Command: "status", Description: "Show your current duty statistics"},
	{Command: "schedule", Description: "View the duty schedule for the current month"},
	{Command: "volunteer", Description: "Sign up for a duty"},
	{Command: "explain", Description: "Explain how the last assignment was made"},
	{Command: "chore", Description: "View your assigned chores"},
	{Command: "takechore", Description: "Volunteer to take an available chore"},
	{Command: "sviniya", Description: "View all sviniya balances"},
	{Command: "spend", Description: "Spend a sviniya"},
}

// adminOnlyCommands are the admin-only entries appended to the user list for
// the admin's private chat. Telegram's scope precedence replaces (does not
// merge) the all_private_chats list when a chat-scoped list is present, so
// the admin scope must contain the union of user + admin commands.
var adminOnlyCommands = []tgbotapi.BotCommand{
	{Command: "assign", Description: "Add days to a user's admin queue"},
	{Command: "unassign", Description: "Remove days from a user's admin queue"},
	{Command: "modify", Description: "Change the assigned user for a date"},
	{Command: "cancel", Description: "Cancel a duty or chore"},
	{Command: "edit", Description: "Edit an active chore"},
	{Command: "offduty", Description: "Set off-duty period for a user"},
	{Command: "vacation", Description: "Toggle vacation mode for a user"},
	{Command: "users", Description: "List all users and their status"},
	{Command: "toggle_active", Description: "Toggle a user's participation"},
	{Command: "ratings", Description: "Show this month's participant ratings"},
	{Command: "chore_stats", Description: "Show top overdue chores and top completions"},
	{Command: "list", Description: "List active periodic chores or tasks"},
	{Command: "complete", Description: "Mark any active chore as completed"},
	{Command: "overdue", Description: "Send the overdue chores report"},
	{Command: "set_sviniya_balance", Description: "Set sviniya balance for a user"},
}

// adminCommands is the full list registered against the admin's private chat:
// user commands plus admin-only commands.
var adminCommands = append(append([]tgbotapi.BotCommand{}, userCommands...), adminOnlyCommands...)

// groupCommands is the read-only / informational subset registered against
// the main DISH_GROUP chat. Action commands like /volunteer or /spend are
// intentionally omitted to keep group-chat autocomplete clean.
var groupCommands = []tgbotapi.BotCommand{
	{Command: "help", Description: "Show available commands"},
	{Command: "status", Description: "Show your current duty statistics"},
	{Command: "schedule", Description: "View the duty schedule for the current month"},
	{Command: "ratings", Description: "Show this month's participant ratings"},
	{Command: "sviniya", Description: "View all sviniya balances"},
	{Command: "chore_stats", Description: "Show top overdue chores and top completions"},
	{Command: "overdue", Description: "Send the overdue chores report"},
}
