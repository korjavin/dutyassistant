package telegram

import (
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func assertCommandList(t *testing.T, label string, cmds []tgbotapi.BotCommand) {
	t.Helper()
	if len(cmds) == 0 {
		t.Fatalf("%s: expected non-empty command list", label)
	}
	seen := make(map[string]bool, len(cmds))
	for i, c := range cmds {
		if c.Command == "" {
			t.Errorf("%s[%d]: empty Command", label, i)
		}
		if len(c.Command) > 32 {
			t.Errorf("%s[%d]: Command %q exceeds 32-char Telegram limit", label, i, c.Command)
		}
		if c.Description == "" {
			t.Errorf("%s[%d] (%s): empty Description", label, i, c.Command)
		}
		if len(c.Description) > 256 {
			t.Errorf("%s[%d] (%s): description exceeds 256-char Telegram limit", label, i, c.Command)
		}
		if seen[c.Command] {
			t.Errorf("%s: duplicate command %q", label, c.Command)
		}
		seen[c.Command] = true
	}
}

func TestUserCommandsCatalog(t *testing.T) {
	assertCommandList(t, "userCommands", userCommands)
}

func TestAdminCommandsCatalog(t *testing.T) {
	assertCommandList(t, "adminCommands", adminCommands)

	// Sanity: adminCommands must be a strict superset of userCommands.
	adminSet := make(map[string]bool, len(adminCommands))
	for _, c := range adminCommands {
		adminSet[c.Command] = true
	}
	for _, u := range userCommands {
		if !adminSet[u.Command] {
			t.Errorf("adminCommands missing user command %q (Telegram scope precedence replaces, does not merge)", u.Command)
		}
	}
}

func TestGroupCommandsCatalog(t *testing.T) {
	assertCommandList(t, "groupCommands", groupCommands)
}
