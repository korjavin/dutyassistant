package session

import (
	"sync"
	"time"

	"github.com/korjavin/dutyassistant/internal/bot/fsm"
)

type Session struct {
	FSM      *fsm.FSM
	Data     interface{}
	LastSeen time.Time
}

type Manager struct {
	mu       sync.RWMutex
	sessions map[int64]*Session
}

func NewManager() *Manager {
	return &Manager{
		sessions: make(map[int64]*Session),
	}
}

func (m *Manager) GetOrCreateSession(userID int64, initialState fsm.State) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	if sess, exists := m.sessions[userID]; exists {
		sess.LastSeen = time.Now()
		return sess
	}

	newFSM := fsm.NewFSM(initialState)
	sess := &Session{
		FSM:      newFSM,
		LastSeen: time.Now(),
	}
	m.sessions[userID] = sess
	return sess
}

func (m *Manager) EndSession(userID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, userID)
}

func (m *Manager) CleanupStaleSessions(timeout time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for userID, sess := range m.sessions {
		if now.Sub(sess.LastSeen) > timeout {
			delete(m.sessions, userID)
		}
	}
}
