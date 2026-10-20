package notification

import (
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/korjavin/dutyassistant/internal/store"
)

// FormatPeriodicChoreReminder produces a friendly HTML message listing a user's pending chores.
func FormatPeriodicChoreReminder(chores []*store.Chore, loc *time.Location) string {
	if len(chores) == 0 {
		return ""
	}

	if loc == nil {
		loc = time.UTC
	}

	var sb strings.Builder
	sb.WriteString("Just a gentle reminder, you've got some chores on your list 🧹\n\n")
	writeChoreList(&sb, chores, loc)
	sb.WriteString("\nYou can complete them using the /chore menu. Thank you! 🌟")

	return sb.String()
}

// FormatNudgeChoreReminder combines an LLM-generated motivational message
// (already sanitized Telegram HTML) with the factual list of pending chores.
func FormatNudgeChoreReminder(nudge string, chores []*store.Chore, loc *time.Location) string {
	if len(chores) == 0 {
		return ""
	}

	if loc == nil {
		loc = time.UTC
	}

	var sb strings.Builder
	sb.WriteString(nudge)
	sb.WriteString("\n\n")
	writeChoreList(&sb, chores, loc)
	sb.WriteString("\nYou can complete them using the /chore menu.")

	return sb.String()
}

func writeChoreList(sb *strings.Builder, chores []*store.Chore, loc *time.Location) {
	for i, chore := range chores {
		deadlineStr := "no deadline"
		if !chore.DeadlineAt.IsZero() {
			deadlineStr = chore.DeadlineAt.In(loc).Format("Jan 02, 15:04")
		}

		fmt.Fprintf(sb, "%d. <b>%s</b> (Due: %s)\n", i+1, html.EscapeString(chore.Description), deadlineStr)
	}
}

// DescribeChoreDelays produces a plain-text summary of how long each chore has
// been pending and whether it is overdue, used as context for the LLM.
func DescribeChoreDelays(chores []*store.Chore, now time.Time) string {
	var sb strings.Builder
	for i, chore := range chores {
		fmt.Fprintf(&sb, "%d. \"%s\"", i+1, chore.Description)
		if !chore.AssignedAt.IsZero() {
			fmt.Fprintf(&sb, " - assigned %s ago", humanizeDuration(now.Sub(chore.AssignedAt)))
		}
		switch {
		case chore.DeadlineAt.IsZero():
			sb.WriteString(", no deadline")
		case now.After(chore.DeadlineAt):
			fmt.Fprintf(&sb, ", OVERDUE by %s", humanizeDuration(now.Sub(chore.DeadlineAt)))
		default:
			fmt.Fprintf(&sb, ", due in %s", humanizeDuration(chore.DeadlineAt.Sub(now)))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// humanizeDuration renders a duration as a coarse "N minutes/hours/days" string.
func humanizeDuration(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	switch {
	case d < time.Hour:
		return pluralize(int(d/time.Minute), "minute")
	case d < 48*time.Hour:
		return pluralize(int(d/time.Hour), "hour")
	default:
		return pluralize(int(d/(24*time.Hour)), "day")
	}
}

func pluralize(n int, unit string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", unit)
	}
	return fmt.Sprintf("%d %ss", n, unit)
}
