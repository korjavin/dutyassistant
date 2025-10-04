package notification

import (
	"testing"
	"time"

	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/stretchr/testify/assert"
)

func TestFormatDutyAssignedMessage(t *testing.T) {
	testUser := &store.User{FirstName: "John"}
	testDate, _ := time.Parse("2006-01-02", "2023-10-27")
	duty := &store.Duty{
		User:     testUser,
		DutyDate: testDate,
	}

	expected := "🔔 *Duty Reminder* 🔔\n\nTomorrow, *Friday, 27 October 2023*, the duty is assigned to *John*\\."
	actual := FormatDutyAssignedMessage(duty)

	assert.Equal(t, expected, actual)
}

func TestFormatDutyAutoAssignedMessage(t *testing.T) {
	testUser := &store.User{FirstName: "Jane"}
	testDate, _ := time.Parse("2006-01-02", "2023-10-28")
	duty := &store.Duty{
		User:     testUser,
		DutyDate: testDate,
	}

	expected := "📢 *Automatic Duty Assignment* 📢\n\nNo duty was scheduled for tomorrow\\. The round\\-robin scheduler has assigned the duty for *Saturday, 28 October 2023* to *Jane*\\."
	actual := FormatDutyAutoAssignedMessage(duty)

	assert.Equal(t, expected, actual)
}

func TestFormatDutyMessage_NilDuty(t *testing.T) {
	expected := "Error: Could not format duty message, essential data is missing."
	actual := FormatDutyAssignedMessage(nil)
	assert.Equal(t, expected, actual)

	actualAuto := FormatDutyAutoAssignedMessage(nil)
	assert.Equal(t, "Error: Could not format auto-assignment message, essential data is missing.", actualAuto)
}

func TestFormatDutyMessage_NilUser(t *testing.T) {
	duty := &store.Duty{
		DutyDate: time.Now(),
		User:     nil, // Nil user
	}
	expected := "Error: Could not format duty message, essential data is missing."
	actual := FormatDutyAssignedMessage(duty)
	assert.Equal(t, expected, actual)

	actualAuto := FormatDutyAutoAssignedMessage(duty)
	assert.Equal(t, "Error: Could not format auto-assignment message, essential data is missing.", actualAuto)
}

func TestEscapeMarkdown(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"No special characters", "Hello world", "Hello world"},
		{"With underscore", "hello_world", "hello\\_world"},
		{"With asterisk", "hello*world", "hello\\*world"},
		{"With multiple characters", "This is a test. (v1.0) #important", "This is a test\\. \\(v1\\.0\\) \\#important"},
		{"All special characters", "_*[]()~`>#+-=|{}.!", "\\_\\*\\[\\]\\(\\)\\~\\`\\>\\#\\+\\-\\=\\|\\{\\}\\.\\!"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, escapeMarkdown(tt.input))
		})
	}
}