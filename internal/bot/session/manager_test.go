package session

import (
	"testing"
	"time"
)

func TestSessionManager(t *testing.T) {
	manager := NewManager()

	sess := manager.GetOrCreateSession(123, "INIT")
	if sess.FSM.CurrentState() != "INIT" {
		t.Errorf("Expected state INIT, got %s", sess.FSM.CurrentState())
	}

	manager.EndSession(123)

	sess2 := manager.GetOrCreateSession(123, "INIT2")
	if sess2 == sess {
		t.Errorf("Expected different session after ending previous one")
	}

	// Test cleanup
	sess2.LastSeen = time.Now().Add(-2 * time.Hour)
	manager.CleanupStaleSessions(1 * time.Hour)

	sess3 := manager.GetOrCreateSession(123, "INIT3")
	if sess3 == sess2 {
		t.Errorf("Expected different session after cleanup")
	}
}
