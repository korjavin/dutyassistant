package notification

import (
	"strings"
	"testing"
	"time"

	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/stretchr/testify/assert"
)

func TestFormatPeriodicChoreReminder_Empty(t *testing.T) {
	msg := FormatPeriodicChoreReminder(nil)
	assert.Empty(t, msg)

	msg = FormatPeriodicChoreReminder([]*store.Chore{})
	assert.Empty(t, msg)
}

func TestFormatPeriodicChoreReminder_SingleChore(t *testing.T) {
	deadline := time.Date(2026, 3, 14, 15, 30, 0, 0, time.UTC)
	chores := []*store.Chore{
		{
			Description: "Take out the trash",
			DeadlineAt:  deadline,
		},
	}

	msg := FormatPeriodicChoreReminder(chores)
	assert.NotEmpty(t, msg)
	assert.True(t, strings.Contains(msg, "Just a gentle reminder, you've got some chores on your list 🧹"))
	assert.True(t, strings.Contains(msg, "1. <b>Take out the trash</b> (Due: Mar 14, 15:30)"))
	assert.True(t, strings.Contains(msg, "Thank you! 🌟"))
}

func TestFormatPeriodicChoreReminder_MultipleChores(t *testing.T) {
	deadline1 := time.Date(2026, 3, 14, 15, 30, 0, 0, time.UTC)
	deadline2 := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	chores := []*store.Chore{
		{
			Description: "Take out the trash",
			DeadlineAt:  deadline1,
		},
		{
			Description: "Wash the dishes",
			DeadlineAt:  deadline2,
		},
	}

	msg := FormatPeriodicChoreReminder(chores)
	assert.NotEmpty(t, msg)
	assert.True(t, strings.Contains(msg, "Just a gentle reminder, you've got some chores on your list 🧹"))
	assert.True(t, strings.Contains(msg, "1. <b>Take out the trash</b> (Due: Mar 14, 15:30)"))
	assert.True(t, strings.Contains(msg, "2. <b>Wash the dishes</b> (Due: Mar 15, 12:00)"))
	assert.True(t, strings.Contains(msg, "Thank you! 🌟"))
}

func TestFormatPeriodicChoreReminder_Escaping(t *testing.T) {
	deadline := time.Date(2026, 3, 14, 15, 30, 0, 0, time.UTC)
	chores := []*store.Chore{
		{
			Description: "Clean <script>alert(1)</script> & <b>tags</b>",
			DeadlineAt:  deadline,
		},
	}

	msg := FormatPeriodicChoreReminder(chores)
	assert.NotEmpty(t, msg)
	assert.True(t, strings.Contains(msg, "1. <b>Clean &lt;script&gt;alert(1)&lt;/script&gt; &amp; &lt;b&gt;tags&lt;/b&gt;</b>"))
}
