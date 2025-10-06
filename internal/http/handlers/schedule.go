package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/korjavin/dutyassistant/internal/http/middleware"
	"github.com/korjavin/dutyassistant/internal/store"
)

// GetSchedule handles the GET /api/v1/schedule/:year/:month endpoint.
// It retrieves the duty schedule for a given month and year.
func GetSchedule(s store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		year, err := strconv.Atoi(c.Param("year"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid year format"})
			return
		}

		month, err := strconv.Atoi(c.Param("month"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid month format"})
			return
		}

		if month < 1 || month > 12 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Month must be between 1 and 12"})
			return
		}

		duties, err := s.GetDutiesByMonth(c.Request.Context(), year, time.Month(month))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve schedule"})
			return
		}

		// Check if user is authenticated
		user, authenticated := c.Request.Context().Value(middleware.UserKey).(*store.User)
		isAuthorized := authenticated && user != nil && user.IsActive

		// Transform to frontend-friendly format
		type dutyResponse struct {
			ID                 int64  `json:"id"`
			Date               string `json:"date"`
			UserID             int64  `json:"user_id"`
			UserName           string `json:"user_name"`
			AssignmentType     string `json:"assignment_type"`
			VolunteerQueueDays int    `json:"volunteer_queue_days"`
			AdminQueueDays     int    `json:"admin_queue_days"`
		}

		response := make([]dutyResponse, 0, len(duties))
		for _, duty := range duties {
			userName := ""
			volunteerQueue := 0
			adminQueue := 0

			// Only include user details if authorized
			if isAuthorized && duty.User != nil {
				userName = duty.User.FirstName
				volunteerQueue = duty.User.VolunteerQueueDays
				adminQueue = duty.User.AdminQueueDays
			} else if duty.User != nil {
				userName = "***" // Anonymous placeholder
			}

			response = append(response, dutyResponse{
				ID:                 duty.ID,
				Date:               duty.DutyDate.Format(time.RFC3339),
				UserID:             duty.UserID,
				UserName:           userName,
				AssignmentType:     string(duty.AssignmentType),
				VolunteerQueueDays: volunteerQueue,
				AdminQueueDays:     adminQueue,
			})
		}

		c.JSON(http.StatusOK, gin.H{"duties": response})
	}
}

// GetPrognosis handles the GET /api/v1/prognosis/:year/:month endpoint.
// It returns an empty prognosis for now (feature not yet implemented).
func GetPrognosis(s store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, err := strconv.Atoi(c.Param("year"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid year format"})
			return
		}

		month, err := strconv.Atoi(c.Param("month"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid month format"})
			return
		}

		if month < 1 || month > 12 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Month must be between 1 and 12"})
			return
		}

		// Return empty prognosis for now
		c.JSON(http.StatusOK, gin.H{"prognosis": []interface{}{}})
	}
}
