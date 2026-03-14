package fsm

import (
	"context"
	"testing"
)

func TestFSMFlows(t *testing.T) {
	t.Run("Chore Flow", func(t *testing.T) {
		machine := InitializeChoreFlow()
		ctx := context.Background()

		// Start -> Desc
		machine.ProcessEvent(ctx, EventStartChore, nil)
		if machine.CurrentState() != StateChoreDesc {
			t.Errorf("expected state %s, got %s", StateChoreDesc, machine.CurrentState())
		}

		// Desc -> Cancel -> Init
		machine.ProcessEvent(ctx, EventCancel, nil)
		if machine.CurrentState() != StateInit {
			t.Errorf("expected state %s, got %s", StateInit, machine.CurrentState())
		}
	})

	t.Run("Daily Ratings Flow", func(t *testing.T) {
		machine := InitializeDailyRatingsFlow()
		ctx := context.Background()

		// Start -> Prompt
		machine.ProcessEvent(ctx, EventStartRating, nil)
		if machine.CurrentState() != StateRatingPrompt {
			t.Errorf("expected state %s, got %s", StateRatingPrompt, machine.CurrentState())
		}

		// Prompt -> Score -> Confirm
		machine.ProcessEvent(ctx, EventInputScore, nil)
		if machine.CurrentState() != StateRatingConfirm {
			t.Errorf("expected state %s, got %s", StateRatingConfirm, machine.CurrentState())
		}
	})
}
