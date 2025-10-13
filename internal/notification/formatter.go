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
	reason := formatAssignmentReason(duty.AssignmentType)
	// Using MarkdownV2 for formatting.
	return fmt.Sprintf(
		"Today's hero is *%s*\\! Because of %s\\. Let us be proud of you\\!",
		escapeMarkdown(duty.User.FirstName),
		reason,
	)
}

// FormatDutyAutoAssignedMessage formats the notification message for a duty that
// was just automatically assigned by the round-robin scheduler.
func FormatDutyAutoAssignedMessage(duty *store.Duty) string {
	if duty == nil || duty.User == nil {
		return "Error: Could not format auto-assignment message, essential data is missing."
	}
	reason := formatAssignmentReason(duty.AssignmentType)
	// Using MarkdownV2 for formatting.
	return fmt.Sprintf(
		"Today's hero is *%s*\\! Because of %s\\. Let us be proud of you\\!",
		escapeMarkdown(duty.User.FirstName),
		reason,
	)
}

// FormatDMToAssignee formats the direct message to the assigned user.
func FormatDMToAssignee(duty *store.Duty) string {
	if duty == nil || duty.User == nil {
		return "Error: Could not format DM message, essential data is missing."
	}
	reason := formatAssignmentReason(duty.AssignmentType)
	// Using MarkdownV2 for formatting.
	return fmt.Sprintf(
		"Congratulations, you were chosen for today because of %s\\. I know it's great to help the family with chores\\!",
		reason,
	)
}

// formatAssignmentReason returns a human-readable reason for the assignment.
func formatAssignmentReason(assignmentType store.AssignmentType) string {
	switch assignmentType {
	case store.AssignmentTypeVoluntary:
		return "your volunteer request"
	case store.AssignmentTypeAdmin:
		return "admin assignment"
	case store.AssignmentTypeRoundRobin:
		return "fair rotation"
	default:
		return "unknown reason"
	}
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