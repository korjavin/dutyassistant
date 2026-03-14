package fsm

import (
	"context"
	"testing"
)

func TestFSM_StateTransitions(t *testing.T) {
	machine := NewFSM("INIT")
	machine.AddTransition("INIT", "START", "RUNNING")

	actionCalled := false
	machine.AddAction("RUNNING", func(ctx context.Context, data interface{}) error {
		actionCalled = true
		return nil
	})

	machine.ProcessEvent(context.Background(), "START", nil)

	if machine.CurrentState() != "RUNNING" {
		t.Errorf("Expected state to be RUNNING, got %s", machine.CurrentState())
	}

	if !actionCalled {
		t.Errorf("Expected action to be called")
	}
}
