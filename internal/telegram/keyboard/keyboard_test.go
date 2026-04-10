package keyboard

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChoreMenu(t *testing.T) {
	menu := ChoreMenu()

	// Collect all button texts and callback data
	var buttons []struct {
		Text string
		Data string
	}
	for _, row := range menu.InlineKeyboard {
		for _, btn := range row {
			buttons = append(buttons, struct {
				Text string
				Data string
			}{Text: btn.Text, Data: *btn.CallbackData})
		}
	}

	assert.Len(t, buttons, 5, "ChoreMenu should have 5 buttons")

	// Verify button order and content
	assert.Equal(t, "📋 List Chores", buttons[0].Text)
	assert.Equal(t, "chore_action:list", buttons[0].Data)

	assert.Equal(t, "➕ Create Chore", buttons[1].Text)
	assert.Equal(t, "chore_action:create", buttons[1].Data)

	assert.Equal(t, "🗑 Delete Chore", buttons[2].Text)
	assert.Equal(t, "chore_action:delete", buttons[2].Data)

	assert.Equal(t, "✅ Complete Chore", buttons[3].Text)
	assert.Equal(t, "chore_action:complete", buttons[3].Data)

	assert.Equal(t, "❌ Cancel", buttons[4].Text)
	assert.Equal(t, "cancel_flow", buttons[4].Data)
}
