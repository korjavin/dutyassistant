package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
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

		// Transform to frontend-friendly format
		type dutyResponse struct {
			ID             int64  `json:"id"`
			Date           string `json:"date"`
			UserID         int64  `json:"user_id"`
			UserName       string `json:"user_name"`
			AssignmentType string `json:"assignment_type"`
		}

		response := make([]dutyResponse, 0, len(duties))
		for _, duty := range duties {
			userName := ""
			if duty.User != nil {
				userName = duty.User.FirstName
			}
			response = append(response, dutyResponse{
				ID:             duty.ID,
				Date:           duty.DutyDate.Format(time.RFC3339),
				UserID:         duty.UserID,
				UserName:       userName,
				AssignmentType: string(duty.AssignmentType),
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
