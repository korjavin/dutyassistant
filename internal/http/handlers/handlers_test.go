package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/korjavin/dutyassistant/internal/http/middleware"
	"github.com/korjavin/dutyassistant/internal/store"
	"github.com/korjavin/dutyassistant/internal/store/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// setupTestServer initializes a new Gin test server with a mock store.
// It does NOT include authentication middleware, allowing handlers to be
// tested in isolation. Tests are responsible for injecting user context
// as needed.
func setupTestServer(mockStore *mocks.MockStore) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	api := router.Group("/api/v1")
	{
		// Public endpoints
		api.GET("/schedule/:year/:month", GetSchedule(mockStore))
		api.GET("/users", GetUsers(mockStore))

		// Endpoints that require authentication context.
		// The real auth middleware is omitted for unit testing.
		api.POST("/duties/volunteer", VolunteerForDuty(mockStore))
		api.POST("/duties", AdminAssignDuty(mockStore))
		api.PUT("/duties/:date", AdminModifyDuty(mockStore))
		api.DELETE("/duties/:date", AdminDeleteDuty(mockStore))
	}

	return router
}

// TestGetSchedule tests the GetSchedule handler.
func TestGetSchedule(t *testing.T) {
	mockStore := new(mocks.MockStore)
	router := setupTestServer(mockStore)

	t.Run("success", func(t *testing.T) {
		year, month := 2023, 10
		dutyDate, _ := time.Parse("2006-01-02", "2023-10-25")
		expectedDuties := []*store.Duty{
			{ID: 1, UserID: 101, DutyDate: dutyDate},
		}

		mockStore.On("GetDutiesByMonth", mock.Anything, year, time.Month(month)).Return(expectedDuties, nil).Once()

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/schedule/2023/10", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var duties []*store.Duty
		json.Unmarshal(w.Body.Bytes(), &duties)
		assert.Equal(t, expectedDuties, duties)
		mockStore.AssertExpectations(t)
	})

	t.Run("invalid year", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/schedule/invalid/10", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("db error", func(t *testing.T) {
		mockStore.On("GetDutiesByMonth", mock.Anything, 2023, time.Month(11)).Return(nil, errors.New("db error")).Once()

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/schedule/2023/11", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		mockStore.AssertExpectations(t)
	})
}

// TestGetUsers tests the GetUsers handler.
func TestGetUsers(t *testing.T) {
	mockStore := new(mocks.MockStore)
	router := setupTestServer(mockStore)

	t.Run("success", func(t *testing.T) {
		expectedUsers := []*store.User{
			{ID: 1, FirstName: "Alice"},
			{ID: 2, FirstName: "Bob"},
		}
		mockStore.On("ListAllUsers", mock.Anything).Return(expectedUsers, nil).Once()

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/users", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var users []*store.User
		json.Unmarshal(w.Body.Bytes(), &users)
		assert.Equal(t, expectedUsers, users)
		mockStore.AssertExpectations(t)
	})
}

// TestVolunteerForDuty tests the VolunteerForDuty handler.
func TestVolunteerForDuty(t *testing.T) {
	mockStore := new(mocks.MockStore)
	router := setupTestServer(mockStore)

	t.Run("success", func(t *testing.T) {
		user := &store.User{ID: 1, TelegramUserID: 123, IsActive: true}
		dateStr := time.Now().Format("2006-01-02")
		dutyDate, _ := time.Parse("2006-01-02", dateStr)

		mockStore.On("DeleteDuty", mock.Anything, dutyDate).Return(nil).Once()
		mockStore.On("CreateDuty", mock.Anything, mock.AnythingOfType("*store.Duty")).Return(nil).Once()

		body, _ := json.Marshal(gin.H{"date": dateStr})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/duties/volunteer", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		// Create a context with the user and attach it to the request.
		ctx := context.WithValue(req.Context(), middleware.UserKey, user)
		req = req.WithContext(ctx)

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		mockStore.AssertExpectations(t)
	})
}

// TestAdminAssignDuty tests the AdminAssignDuty handler.
func TestAdminAssignDuty(t *testing.T) {
	mockStore := new(mocks.MockStore)
	router := setupTestServer(mockStore)

	t.Run("success", func(t *testing.T) {
		adminUser := &store.User{ID: 1, TelegramUserID: 123, IsActive: true, IsAdmin: true}
		dateStr := "2023-11-11"
		dutyDate, _ := time.Parse("2006-01-02", dateStr)

		mockStore.On("DeleteDuty", mock.Anything, dutyDate).Return(nil).Once()
		mockStore.On("CreateDuty", mock.Anything, mock.AnythingOfType("*store.Duty")).Return(nil).Once()

		body, _ := json.Marshal(gin.H{"user_id": 101, "date": dateStr})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/api/v1/duties", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		// Create a context with the user and attach it to the request.
		ctx := context.WithValue(req.Context(), middleware.UserKey, adminUser)
		req = req.WithContext(ctx)

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		mockStore.AssertExpectations(t)
	})
}

// TestAdminModifyDuty tests the AdminModifyDuty handler.
func TestAdminModifyDuty(t *testing.T) {
	mockStore := new(mocks.MockStore)
	router := setupTestServer(mockStore)

	t.Run("success", func(t *testing.T) {
		adminUser := &store.User{ID: 1, TelegramUserID: 123, IsActive: true, IsAdmin: true}
		dateStr := "2023-11-12"
		dutyDate, _ := time.Parse("2006-01-02", dateStr)
		existingDuty := &store.Duty{ID: 1, UserID: 101, DutyDate: dutyDate}

		mockStore.On("GetDutyByDate", mock.Anything, dutyDate).Return(existingDuty, nil).Once()
		mockStore.On("UpdateDuty", mock.Anything, mock.AnythingOfType("*store.Duty")).Return(nil).Once()

		body, _ := json.Marshal(gin.H{"user_id": 102})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("PUT", "/api/v1/duties/"+dateStr, bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		// Create a context with the user and attach it to the request.
		ctx := context.WithValue(req.Context(), middleware.UserKey, adminUser)
		req = req.WithContext(ctx)

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockStore.AssertExpectations(t)
	})
}

// TestAdminDeleteDuty tests the AdminDeleteDuty handler.
func TestAdminDeleteDuty(t *testing.T) {
	mockStore := new(mocks.MockStore)
	router := setupTestServer(mockStore)

	t.Run("success", func(t *testing.T) {
		adminUser := &store.User{ID: 1, TelegramUserID: 123, IsActive: true, IsAdmin: true}
		dateStr := "2023-11-13"
		dutyDate, _ := time.Parse("2006-01-02", dateStr)

		mockStore.On("DeleteDuty", mock.Anything, dutyDate).Return(nil).Once()

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("DELETE", "/api/v1/duties/"+dateStr, nil)

		// Create a context with the user and attach it to the request.
		ctx := context.WithValue(req.Context(), middleware.UserKey, adminUser)
		req = req.WithContext(ctx)

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
		mockStore.AssertExpectations(t)
	})
}