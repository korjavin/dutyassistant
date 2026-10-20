package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseChoreArgs(t *testing.T) {
	tests := []struct {
		name         string
		args         string
		wantDesc     string
		wantInterval string
		wantAssignee string
		wantFrom     string
		wantUntil    string
	}{
		{"plain description", "Clean kitchen", "Clean kitchen", "", "", "", ""},
		{"assignee", "Fix the door knob /Crusader", "Fix the door knob", "", "Crusader", "", ""},
		{"interval", "Water plants /3d", "Water plants", "3", "", "", ""},
		{"interval uppercase", "Water plants /10D", "Water plants", "10", "", "", ""},
		{"assignee then interval", "Water plants /Alice /3d", "Water plants", "3", "Alice", "", ""},
		{"interval then assignee", "Water plants /3d /Alice", "Water plants", "3", "Alice", "", ""},
		{"cyrillic assignee", "Помыть посуду /Маша", "Помыть посуду", "", "Маша", "", ""},
		{"underscore assignee", "Clean /Anna_Maria", "Clean", "", "Anna_Maria", "", ""},
		{"surrounding whitespace", "  Clean kitchen /Bob  ", "Clean kitchen", "", "Bob", "", ""},
		{"only assignee", "/Alice", "", "", "Alice", "", ""},
		{"only interval", "/3d", "", "3", "", "", ""},
		{"only assignee after space", "x /Alice", "x", "", "Alice", "", ""},
		{"slash inside word is kept", "Clean kitchen/bathroom", "Clean kitchen/bathroom", "", "", "", ""},
		{"zero interval is not an interval", "Clean /0d", "Clean", "", "0d", "", ""},
		{"second assignee stays in description", "Clean /Alice /Bob", "Clean /Alice", "", "Bob", "", ""},
		{"second interval stays in description", "Clean /2d /3d", "Clean /2d", "3", "", "", ""},
		{"from", "Air the cellar /Vladimir /from=2026-10-21_12:00", "Air the cellar", "", "Vladimir", "2026-10-21_12:00", ""},
		{"from until interval any order", "Air /until=2026-10-30 /1d /Vladimir /from=2026-10-21_12:00", "Air", "1", "Vladimir", "2026-10-21_12:00", "2026-10-30"},
		{"schedule keys are case-insensitive", "Air /FROM=2026-10-21 /Until=2026-10-30", "Air", "", "", "2026-10-21", "2026-10-30"},
		{"second from stays in description", "Air /from=a /from=b", "Air /from=a", "", "", "b", ""},
		{"empty from is an assignee", "Air /from=", "Air", "", "from=", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseChoreArgs(tt.args)
			assert.Equal(t, tt.wantDesc, got.Description)
			assert.Equal(t, tt.wantInterval, got.Interval)
			assert.Equal(t, tt.wantAssignee, got.Assignee)
			assert.Equal(t, tt.wantFrom, got.From)
			assert.Equal(t, tt.wantUntil, got.Until)
		})
	}
}
