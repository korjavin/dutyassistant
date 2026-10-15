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
	}{
		{"plain description", "Clean kitchen", "Clean kitchen", "", ""},
		{"assignee", "Fix the door knob /Crusader", "Fix the door knob", "", "Crusader"},
		{"interval", "Water plants /3d", "Water plants", "3", ""},
		{"interval uppercase", "Water plants /10D", "Water plants", "10", ""},
		{"assignee then interval", "Water plants /Alice /3d", "Water plants", "3", "Alice"},
		{"interval then assignee", "Water plants /3d /Alice", "Water plants", "3", "Alice"},
		{"cyrillic assignee", "Помыть посуду /Маша", "Помыть посуду", "", "Маша"},
		{"underscore assignee", "Clean /Anna_Maria", "Clean", "", "Anna_Maria"},
		{"surrounding whitespace", "  Clean kitchen /Bob  ", "Clean kitchen", "", "Bob"},
		{"only assignee", "/Alice", "", "", "Alice"},
		{"only interval", "/3d", "", "3", ""},
		{"only assignee after space", "x /Alice", "x", "", "Alice"},
		{"slash inside word is kept", "Clean kitchen/bathroom", "Clean kitchen/bathroom", "", ""},
		{"zero interval is not an interval", "Clean /0d", "Clean", "", "0d"},
		{"second assignee stays in description", "Clean /Alice /Bob", "Clean /Alice", "", "Bob"},
		{"second interval stays in description", "Clean /2d /3d", "Clean /2d", "3", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desc, interval, assignee := parseChoreArgs(tt.args)
			assert.Equal(t, tt.wantDesc, desc)
			assert.Equal(t, tt.wantInterval, interval)
			assert.Equal(t, tt.wantAssignee, assignee)
		})
	}
}
