package handlers

import (
	"net/http"
	"strconv"

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

		duties, err := s.GetDutiesByMonth(c.Request.Context(), year, month)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve schedule"})
			return
		}

		// To avoid returning `null` for an empty slice, we initialize it.
		if duties == nil {
			duties = []*store.Duty{}
		}

		c.JSON(http.StatusOK, duties)
	}
}