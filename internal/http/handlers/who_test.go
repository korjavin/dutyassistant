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

func TestGetWho_DutyAssigned(t *testing.T) {
	mockStore := new(mocks.MockStore)
	router := setupWhoRouter(mockStore)

	duty := &store.Duty{
		ID:       1,
		UserID:   42,
		DutyDate: time.Now(),
		User:     &store.User{FirstName: "Ivan"},
	}
	mockStore.On("GetTodaysDuty", mock.Anything).Return(duty, nil).Once()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/who", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "Ivan", resp["name"])
	mockStore.AssertExpectations(t)
}

func TestGetWho_NoDutyAssigned(t *testing.T) {
	mockStore := new(mocks.MockStore)
	router := setupWhoRouter(mockStore)

	mockStore.On("GetTodaysDuty", mock.Anything).Return(nil, nil).Once()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/who", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "", resp["name"])
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

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/who", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "", resp["name"])
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
