package handlers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSessionManager_Lifecycle(t *testing.T) {
	sm := NewSessionManager()
	chatID := int64(123)
	userID := int64(456)
	sessionType := SessionTypeChoreCreation

	// 1. Initially no session
	_, exists := sm.GetSession(chatID)
	assert.False(t, exists)

	// 2. Start session
	sm.StartSession(chatID, userID, sessionType)
	session, exists := sm.GetSession(chatID)
	assert.True(t, exists)
	assert.NotNil(t, session)
	assert.Equal(t, chatID, session.ChatID)
	assert.Equal(t, userID, session.UserID)
	assert.Equal(t, sessionType, session.Type)
	assert.WithinDuration(t, time.Now(), session.CreatedAt, 1*time.Second)

	// 3. Overwrite session
	newUserID := int64(789)
	sm.StartSession(chatID, newUserID, sessionType)
	session, exists = sm.GetSession(chatID)
	assert.True(t, exists)
	assert.Equal(t, newUserID, session.UserID)

	// 4. End session
	sm.EndSession(chatID)
	_, exists = sm.GetSession(chatID)
	assert.False(t, exists)
}

func TestSession_DataStorage(t *testing.T) {
	session := &Session{
		Data: make(map[string]interface{}),
	}

	// 1. Initially no data
	_, exists := session.GetData("test_key")
	assert.False(t, exists)

	// 2. Set and get data
	session.SetData("test_key", "test_value")
	val, exists := session.GetData("test_key")
	assert.True(t, exists)
	assert.Equal(t, "test_value", val)

	// 3. Overwrite data
	session.SetData("test_key", 123)
	val, exists = session.GetData("test_key")
	assert.True(t, exists)
	assert.Equal(t, 123, val)
}

func TestSessionManager_Cleanup(t *testing.T) {
	sm := NewSessionManager()
	chatID1 := int64(1)
	chatID2 := int64(2)

	sm.StartSession(chatID1, 101, SessionTypeChoreCreation)
	sm.StartSession(chatID2, 102, SessionTypeChoreCreation)

	// Manually manipulate CreatedAt for session 1 to be old
	sm.mu.Lock()
	sm.sessions[chatID1].CreatedAt = time.Now().Add(-10 * time.Minute)
	sm.mu.Unlock()

	// Run cleanup with 5 minute threshold
	sm.removeStale(5 * time.Minute)

	// Session 1 should be gone, session 2 should remain
	_, exists := sm.GetSession(chatID1)
	assert.False(t, exists)

	_, exists = sm.GetSession(chatID2)
	assert.True(t, exists)
}

func TestSessionManager_Cleanup_KeepsDailyRatingsSessionThroughSameBerlinDay(t *testing.T) {
	sm := NewSessionManager()
	chatID := int64(3)

	sm.StartSession(chatID, 123, SessionTypeDailyRatings)

	sm.mu.Lock()
	sm.sessions[chatID].CreatedAt = time.Date(2026, time.March, 13, 20, 50, 0, 0, time.UTC)
	sm.sessions[chatID].SetData(ratingSessionDateKey, normalizeRatingDate(time.Date(2026, time.March, 13, 20, 50, 0, 0, time.UTC)))
	sm.mu.Unlock()

	sm.removeStaleAt(time.Date(2026, time.March, 13, 22, 55, 0, 0, time.UTC), 5*time.Minute)

	_, exists := sm.GetSession(chatID)
	assert.True(t, exists)
}

func TestSessionManager_Cleanup_ExpiresDailyRatingsSessionAfterBerlinDayChanges(t *testing.T) {
	sm := NewSessionManager()
	chatID := int64(4)

	sm.StartSession(chatID, 123, SessionTypeDailyRatings)

	sm.mu.Lock()
	sm.sessions[chatID].CreatedAt = time.Date(2026, time.March, 13, 20, 50, 0, 0, time.UTC)
	sm.sessions[chatID].SetData(ratingSessionDateKey, normalizeRatingDate(time.Date(2026, time.March, 13, 20, 50, 0, 0, time.UTC)))
	sm.mu.Unlock()

	sm.removeStaleAt(time.Date(2026, time.March, 13, 23, 5, 0, 0, time.UTC), 5*time.Minute)

	_, exists := sm.GetSession(chatID)
	assert.False(t, exists)
}

func TestSessionManager_Cleanup_ExpiresDailyRatingsSessionAtMonthEndCutoff(t *testing.T) {
	sm := NewSessionManager()
	chatID := int64(5)

	sm.StartSession(chatID, 123, SessionTypeDailyRatings)

	sm.mu.Lock()
	sm.sessions[chatID].CreatedAt = time.Date(2026, time.March, 31, 20, 50, 0, 0, time.UTC)
	sm.sessions[chatID].SetData(ratingSessionDateKey, normalizeRatingDate(time.Date(2026, time.March, 31, 20, 50, 0, 0, time.UTC)))
	sm.mu.Unlock()

	sm.removeStaleAt(time.Date(2026, time.March, 31, 21, 0, 0, 0, time.UTC), 5*time.Minute)

	_, exists := sm.GetSession(chatID)
	assert.False(t, exists)
}
