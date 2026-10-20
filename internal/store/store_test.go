package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRecurringChore_RunsLeft(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skip("no tzdata")
	}
	at := func(day, hour int) time.Time { return time.Date(2026, time.October, day, hour, 0, 0, 0, berlin) }
	endOf := func(day int) *time.Time { t := time.Date(2026, time.October, day, 23, 59, 59, 0, berlin); return &t }

	tests := []struct {
		name  string
		chore RecurringChore
		want  int
	}{
		{"one-off", RecurringChore{Interval: 0, NextRunAt: at(21, 12)}, 1},
		{"no end", RecurringChore{Interval: 1, NextRunAt: at(21, 12)}, -1},
		{"daily for 10 days", RecurringChore{Interval: 1, NextRunAt: at(21, 12), EndsAt: endOf(30)}, 10},
		// 2026-10-25 is the end of DST in Berlin; the 12:00 runs must not drift
		{"across DST change", RecurringChore{Interval: 1, NextRunAt: at(24, 12), EndsAt: endOf(26)}, 3},
		{"every 3 days", RecurringChore{Interval: 3, NextRunAt: at(21, 12), EndsAt: endOf(30)}, 4},
		{"last run on the end day", RecurringChore{Interval: 1, NextRunAt: at(30, 12), EndsAt: endOf(30)}, 1},
		{"end before next run", RecurringChore{Interval: 1, NextRunAt: at(31, 12), EndsAt: endOf(30)}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.chore.RunsLeft(berlin))
		})
	}
}
