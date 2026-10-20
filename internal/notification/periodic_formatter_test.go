package notification

import (
	"strings"
	"testing"
	"time"

	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/stretchr/testify/assert"
)

func TestFormatPeriodicChoreReminder_Empty(t *testing.T) {
	msg := FormatPeriodicChoreReminder(nil, time.UTC)
	assert.Empty(t, msg)

	msg = FormatPeriodicChoreReminder([]*store.Chore{}, time.UTC)
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

	msg := FormatPeriodicChoreReminder(chores, time.UTC)
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

	msg := FormatPeriodicChoreReminder(chores, time.UTC)
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

	msg := FormatPeriodicChoreReminder(chores, time.UTC)
	assert.NotEmpty(t, msg)
	assert.True(t, strings.Contains(msg, "1. <b>Clean &lt;script&gt;alert(1)&lt;/script&gt; &amp; &lt;b&gt;tags&lt;/b&gt;</b>"))
}

func TestFormatPeriodicChoreReminder_TimezoneShift(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Berlin") // UTC+1 or UTC+2

	// Create a chore with a UTC deadline
	deadline := time.Date(2026, 3, 14, 22, 59, 0, 0, time.UTC)
	chores := []*store.Chore{
		{
			Description: "Timezone Test",
			DeadlineAt:  deadline,
		},
	}

	// In Berlin time (CET, UTC+1), 22:59 UTC on March 14, 2026 is 23:59 local time.
	msg := FormatPeriodicChoreReminder(chores, loc)
	assert.NotEmpty(t, msg)
	assert.True(t, strings.Contains(msg, "1. <b>Timezone Test</b> (Due: Mar 14, 23:59)"))
}

func TestFormatNudgeChoreReminder(t *testing.T) {
	assert.Empty(t, FormatNudgeChoreReminder("hi", nil, time.UTC))

	chores := []*store.Chore{
		{Description: "Feed <the> cat", DeadlineAt: time.Date(2026, 3, 14, 15, 30, 0, 0, time.UTC)},
		{Description: "Vacuum"},
	}
	msg := FormatNudgeChoreReminder("🐱 The cat is filing a complaint!", chores, time.UTC)

	assert.True(t, strings.HasPrefix(msg, "🐱 The cat is filing a complaint!\n\n"))
	assert.Contains(t, msg, "1. <b>Feed &lt;the&gt; cat</b> (Due: Mar 14, 15:30)")
	assert.Contains(t, msg, "2. <b>Vacuum</b> (Due: no deadline)")
	assert.Contains(t, msg, "/chore menu")
	assert.NotContains(t, msg, "Just a gentle reminder")
}

func TestDescribeChoreDelays(t *testing.T) {
	now := time.Date(2026, 3, 14, 17, 0, 0, 0, time.UTC)
	chores := []*store.Chore{
		{Description: "Trash", AssignedAt: now.Add(-73 * time.Hour), DeadlineAt: now.Add(-50 * time.Hour)},
		{Description: "Dishes", AssignedAt: now.Add(-5 * time.Hour), DeadlineAt: now.Add(3 * time.Hour)},
		{Description: "Plants", AssignedAt: now.Add(-1 * time.Hour)},
		{Description: "Laundry", AssignedAt: now.Add(-30 * time.Minute), DeadlineAt: now.Add(-1 * time.Minute)},
	}

	got := DescribeChoreDelays(chores, now)

	assert.Contains(t, got, "1. \"Trash\" - assigned 3 days ago, OVERDUE by 2 days\n")
	assert.Contains(t, got, "2. \"Dishes\" - assigned 5 hours ago, due in 3 hours\n")
	assert.Contains(t, got, "3. \"Plants\" - assigned 1 hour ago, no deadline\n")
	assert.Contains(t, got, "4. \"Laundry\" - assigned 30 minutes ago, OVERDUE by 1 minute\n")
}

func TestHumanizeDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "0 minutes"},
		{time.Minute, "1 minute"},
		{59 * time.Minute, "59 minutes"},
		{time.Hour, "1 hour"},
		{47 * time.Hour, "47 hours"},
		{48 * time.Hour, "2 days"},
		{-3 * time.Hour, "3 hours"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, humanizeDuration(tt.d), tt.d.String())
	}
}
