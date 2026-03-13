package handlers

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/korjavin/dutyassistant/internal/store"
)

type choreItem struct {
	Description string `json:"description"`
	DeadlineAt  string `json:"deadline_at"`
	Assignee    string `json:"assignee"`
}

// TimeNow is exposed for testing
var timeNow = time.Now

func formatRelativeDate(deadline time.Time) string {
	now := timeNow()

	// Convert both to UTC to avoid DST boundary issues when comparing days
	deadlineUTC := deadline.UTC()
	nowUTC := now.UTC()

	// Truncate to start of day in UTC
	deadlineDate := time.Date(deadlineUTC.Year(), deadlineUTC.Month(), deadlineUTC.Day(), 0, 0, 0, 0, time.UTC)
	nowDate := time.Date(nowUTC.Year(), nowUTC.Month(), nowUTC.Day(), 0, 0, 0, 0, time.UTC)

	// Since they are UTC, each day is exactly 24 hours
	days := int(nowDate.Sub(deadlineDate).Hours() / 24)

	if days == 0 {
		return "today"
	} else if days == 1 {
		return "yesterday"
	} else if days > 1 {
		return fmt.Sprintf("%d days ago", days)
	} else if days == -1 {
		return "tomorrow"
	} else {
		return fmt.Sprintf("in %d days", -days)
	}
}

// GetWho handles the GET /who endpoint.
// It returns the name of today's assigned duty person and active chores as a JSON object
// compatible with the EchoBridge DUTY_API contract.
//
// Responses:
//   - 200 OK with {"name": "<FirstName>", "chores": [...]} when a duty is assigned.
//   - 200 OK with {"name": "", "chores": [...]} when no duty is assigned today.
//   - 500 Internal Server Error on database failure.
func GetWho(s store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		duty, err := s.GetTodaysDuty(c.Request.Context())
		if err != nil {
			log.Printf("[WHO] Failed to get today's duty: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve duty information"})
			return
		}

		chores, err := s.GetActiveChores(c.Request.Context())
		if err != nil {
			log.Printf("[WHO] Failed to get active chores: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve active chores"})
			return
		}

		choreItems := make([]choreItem, 0) // ensure it marshals to [] instead of null
		for _, chore := range chores {
			assignee := ""
			if chore.User != nil {
				assignee = chore.User.FirstName
			}
			choreItems = append(choreItems, choreItem{
				Description: chore.Description,
				DeadlineAt:  formatRelativeDate(chore.DeadlineAt),
				Assignee:    assignee,
			})
		}

		name := ""
		if duty != nil && duty.User != nil {
			name = duty.User.FirstName
		}

		c.JSON(http.StatusOK, gin.H{
			"name":   name,
			"chores": choreItems,
		})
	}
}
