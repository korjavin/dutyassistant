package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/korjavin/dutyassistant/internal/store"
)

// GetWho handles the GET /who endpoint.
// It returns the name of today's assigned duty person as a JSON object
// compatible with the EchoBridge DUTY_API contract.
//
// Responses:
//   - 200 OK with {"name": "<FirstName>"} when a duty is assigned.
//   - 200 OK with {"name": ""} when no duty is assigned today.
//   - 500 Internal Server Error on database failure.
func GetWho(s store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		duty, err := s.GetTodaysDuty(c.Request.Context())
		if err != nil {
			log.Printf("[WHO] Failed to get today's duty: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve duty information"})
			return
		}

		if duty == nil || duty.User == nil {
			// No duty assigned today — return an empty name.
			// EchoBridge will handle this as "no one is on duty".
			c.JSON(http.StatusOK, gin.H{"name": ""})
			return
		}

		c.JSON(http.StatusOK, gin.H{"name": duty.User.FirstName})
	}
}
