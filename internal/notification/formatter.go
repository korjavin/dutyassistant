package notification

import (
	"fmt"
	"strings"

	"github.com/korjavin/dutyassistant/internal/store"
)

const (
	// dutyDateFormat defines the format for dates in notifications (e.g., "Monday, 02 January 2006").
	dutyDateFormat = "Monday, 02 January 2006"
)

// FormatDutyAssignedMessage formats the notification message for a pre-existing duty.
// It reminds the group who is on duty for the upcoming day.
func FormatDutyAssignedMessage(duty *store.Duty) string {
	if duty == nil || duty.User == nil {
		return "Error: Could not format duty message, essential data is missing."
	}
	dateStr := duty.DutyDate.Format(dutyDateFormat)
	// Using MarkdownV2 for formatting. Note the escaped period at the end.
	return fmt.Sprintf(
		"🔔 *Duty Reminder* 🔔\n\nTomorrow, *%s*, the duty is assigned to *%s*\\.",
		escapeMarkdown(dateStr),
		escapeMarkdown(duty.User.FirstName),
	)
}

// FormatDutyAutoAssignedMessage formats the notification message for a duty that
// was just automatically assigned by the round-robin scheduler.
func FormatDutyAutoAssignedMessage(duty *store.Duty) string {
	if duty == nil || duty.User == nil {
		return "Error: Could not format auto-assignment message, essential data is missing."
	}
	dateStr := duty.DutyDate.Format(dutyDateFormat)
	// Using MarkdownV2 for formatting. Note the escaped characters in the static text.
	return fmt.Sprintf(
		"📢 *Automatic Duty Assignment* 📢\n\nNo duty was scheduled for tomorrow\\. The round\\-robin scheduler has assigned the duty for *%s* to *%s*\\.",
		escapeMarkdown(dateStr),
		escapeMarkdown(duty.User.FirstName),
	)
}

// escapeMarkdown escapes characters for Telegram's MarkdownV2 parser.
// See: https://core.telegram.org/bots/api#markdownv2-style
func escapeMarkdown(s string) string {
	charsToEscape := []string{"_", "*", "[", "]", "(", ")", "~", "`", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!"}
	for _, char := range charsToEscape {
		s = strings.ReplaceAll(s, char, "\\"+char)
	}
	return s
}