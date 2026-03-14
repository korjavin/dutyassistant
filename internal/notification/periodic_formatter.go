package notification

import (
	"fmt"
	"strings"

	"github.com/korjavin/dutyassistant/internal/store"
)

// FormatPeriodicChoreReminder produces a friendly HTML message listing a user's pending chores.
func FormatPeriodicChoreReminder(chores []*store.Chore) string {
	if len(chores) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("Just a gentle reminder, you've got some chores on your list 🧹\n\n")

	for i, chore := range chores {
		deadlineStr := "no deadline"
		if !chore.DeadlineAt.IsZero() {
			deadlineStr = chore.DeadlineAt.Format("Jan 02, 15:04")
		}

		sb.WriteString(fmt.Sprintf("%d. <b>%s</b> (Due: %s)\n", i+1, chore.Description, deadlineStr))
	}

	sb.WriteString("\nYou can complete them using the /chore menu. Thank you! 🌟")

	return sb.String()
}
