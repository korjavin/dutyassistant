package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/korjavin/dutyassistant/internal/store/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setupWhoRouter(mockStore *mocks.MockStore) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/who", GetWho(mockStore))
	return r
}

type whoResponse struct {
	Name   string      `json:"name"`
	Chores []choreItem `json:"chores"`
}

func TestGetWho_DutyAssigned(t *testing.T) {
	mockStore := new(mocks.MockStore)
	router := setupWhoRouter(mockStore)

	now := time.Now()
	// Mock timeNow so formatRelativeDate is deterministic.
	timeNow = func() time.Time {
		return now
	}

	duty := &store.Duty{
		ID:       1,
		UserID:   42,
		DutyDate: now,
		User:     &store.User{FirstName: "Ivan"},
	}
	mockStore.On("GetTodaysDuty", mock.Anything).Return(duty, nil).Once()

	chores := []*store.Chore{
		{Description: "Buy milk", DeadlineAt: now, User: &store.User{FirstName: "Maria"}},
		{Description: "Clean desk", DeadlineAt: now.AddDate(0, 0, -1), User: &store.User{FirstName: "John"}},
	}
	mockStore.On("GetActiveChores", mock.Anything).Return(chores, nil).Once()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/who", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp whoResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "Ivan", resp.Name)
	assert.Len(t, resp.Chores, 2)
	assert.Equal(t, "Buy milk", resp.Chores[0].Description)
	assert.Equal(t, "today", resp.Chores[0].DeadlineAt)
	assert.Equal(t, "Maria", resp.Chores[0].Assignee)
	assert.Equal(t, "Clean desk", resp.Chores[1].Description)
	assert.Equal(t, "yesterday", resp.Chores[1].DeadlineAt)
	assert.Equal(t, "John", resp.Chores[1].Assignee)

	mockStore.AssertExpectations(t)
}

func TestGetWho_NoDutyAssigned(t *testing.T) {
	mockStore := new(mocks.MockStore)
	router := setupWhoRouter(mockStore)

	mockStore.On("GetTodaysDuty", mock.Anything).Return(nil, nil).Once()
	mockStore.On("GetActiveChores", mock.Anything).Return([]*store.Chore{}, nil).Once()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/who", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp whoResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "", resp.Name)
	assert.NotNil(t, resp.Chores)
	assert.Len(t, resp.Chores, 0)
	mockStore.AssertExpectations(t)
}

func TestGetWho_DutyWithoutUser(t *testing.T) {
	// Duty exists but User is not eagerly loaded (nil) — should return empty name.
	mockStore := new(mocks.MockStore)
	router := setupWhoRouter(mockStore)

	duty := &store.Duty{
		ID:       1,
		UserID:   42,
		DutyDate: time.Now(),
		User:     nil,
	}
	mockStore.On("GetTodaysDuty", mock.Anything).Return(duty, nil).Once()
	mockStore.On("GetActiveChores", mock.Anything).Return([]*store.Chore{}, nil).Once()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/who", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp whoResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "", resp.Name)
	assert.NotNil(t, resp.Chores)
	assert.Len(t, resp.Chores, 0)
	mockStore.AssertExpectations(t)
}

func TestGetWho_DatabaseError(t *testing.T) {
	mockStore := new(mocks.MockStore)
	router := setupWhoRouter(mockStore)

	mockStore.On("GetTodaysDuty", mock.Anything).Return(nil, errors.New("db connection lost")).Once()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/who", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockStore.AssertExpectations(t)
}

func TestGetWho_ChoresError(t *testing.T) {
	mockStore := new(mocks.MockStore)
	router := setupWhoRouter(mockStore)

	duty := &store.Duty{
		ID:       1,
		UserID:   42,
		DutyDate: time.Now(),
		User:     &store.User{FirstName: "Ivan"},
	}
	mockStore.On("GetTodaysDuty", mock.Anything).Return(duty, nil).Once()
	mockStore.On("GetActiveChores", mock.Anything).Return(nil, errors.New("db chores error")).Once()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/who", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockStore.AssertExpectations(t)
}

func TestFormatRelativeDate(t *testing.T) {
	loc, _ := time.LoadLocation("UTC")
	now := time.Date(2023, 10, 15, 12, 0, 0, 0, loc)

	timeNow = func() time.Time {
		return now
	}

	tests := []struct {
		name     string
		deadline time.Time
		expected string
	}{
		{"today", time.Date(2023, 10, 15, 14, 0, 0, 0, loc), "today"},
		{"yesterday", time.Date(2023, 10, 14, 10, 0, 0, 0, loc), "yesterday"},
		{"2 days ago", time.Date(2023, 10, 13, 18, 0, 0, 0, loc), "2 days ago"},
		{"3 days ago", time.Date(2023, 10, 12, 8, 0, 0, 0, loc), "3 days ago"},
		{"tomorrow", time.Date(2023, 10, 16, 9, 0, 0, 0, loc), "tomorrow"},
		{"in 2 days", time.Date(2023, 10, 17, 9, 0, 0, 0, loc), "in 2 days"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, formatRelativeDate(tt.deadline))
		})
	}
}
