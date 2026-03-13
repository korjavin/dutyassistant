package handlers

import (
	"sync"
	"time"
)

// SessionType represents the type of interactive session
type SessionType string

const (
	SessionTypeChoreCreation SessionType = "chore_creation"
	SessionTypeDailyRatings  SessionType = "daily_ratings"
)

// Session represents an active user session
type Session struct {
	Type      SessionType
	ChatID    int64
	UserID    int64
	CreatedAt time.Time
	Data      map[string]interface{}
}

// SessionManager manages interactive command sessions
type SessionManager struct {
	sessions map[int64]*Session // key is chatID
	mu       sync.RWMutex
}

// NewSessionManager creates a new session manager
func NewSessionManager() *SessionManager {
	sm := &SessionManager{
		sessions: make(map[int64]*Session),
	}

	// Start cleanup goroutine to remove stale sessions (older than 5 minutes)
	go sm.cleanupStale()

	return sm
}

// StartSession creates a new session for the given chat
func (sm *SessionManager) StartSession(chatID int64, userID int64, sessionType SessionType) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.sessions[chatID] = &Session{
		Type:      sessionType,
		ChatID:    chatID,
		UserID:    userID,
		CreatedAt: time.Now(),
		Data:      make(map[string]interface{}),
	}
}

// GetSession retrieves the active session for a chat
func (sm *SessionManager) GetSession(chatID int64) (*Session, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, ok := sm.sessions[chatID]
	return session, ok
}

// EndSession removes the session for the given chat
func (sm *SessionManager) EndSession(chatID int64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	delete(sm.sessions, chatID)
}

// TouchSession refreshes the session timestamp to keep an active flow alive.
func (sm *SessionManager) TouchSession(chatID int64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if session, ok := sm.sessions[chatID]; ok {
		session.CreatedAt = time.Now()
	}
}

// SetData sets a value in the session data
func (s *Session) SetData(key string, value interface{}) {
	s.Data[key] = value
}

// GetData retrieves a value from session data
func (s *Session) GetData(key string) (interface{}, bool) {
	val, ok := s.Data[key]
	return val, ok
}

// cleanupStale removes sessions older than 5 minutes
func (sm *SessionManager) cleanupStale() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		sm.removeStale(5 * time.Minute)
	}
}

// removeStale removes sessions older than the given duration
func (sm *SessionManager) removeStale(olderThan time.Duration) {
	sm.removeStaleAt(time.Now(), olderThan)
}

func (sm *SessionManager) removeStaleAt(now time.Time, olderThan time.Duration) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	for chatID, session := range sm.sessions {
		if sessionExpired(session, now, olderThan) {
			delete(sm.sessions, chatID)
		}
	}
}

func sessionExpired(session *Session, now time.Time, olderThan time.Duration) bool {
	if session == nil {
		return true
	}

	if session.Type == SessionTypeDailyRatings {
		ratingDate, ok := ratingDateFromSession(session)
		if ok {
			return !ratingSubmissionWindowOpen(ratingDate, now)
		}
	}

	return now.Sub(session.CreatedAt) > olderThan
}
