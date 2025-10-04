package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/korjavin/dutyassistant/internal/scheduler"
	"github.com/korjavin/dutyassistant/internal/store"
)

// GetPrognosis handles GET /api/v1/prognosis/:year/:month
// Returns round-robin predictions for days without assignments
func GetPrognosis(s store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		year, err := strconv.Atoi(c.Param("year"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid year"})
			return
		}

		month, err := strconv.Atoi(c.Param("month"))
		if err != nil || month < 1 || month > 12 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid month"})
			return
		}

		sched := scheduler.NewScheduler(s)
		ctx := context.Background()

		// Get existing duties for the month
		existingDuties, err := s.GetDutiesByMonth(ctx, year, time.Month(month))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get duties"})
			return
		}

		existingDatesMap := make(map[string]bool)
		for _, duty := range existingDuties {
			dateStr := duty.DutyDate.Format("2006-01-02")
			existingDatesMap[dateStr] = true
		}

		// Generate prognosis for each day of the month
		type prognosisItem struct {
			Date     string `json:"date"`
			UserName string `json:"user_name"`
			UserID   int64  `json:"user_id"`
		}

		var prognoses []prognosisItem
		daysInMonth := time.Date(year, time.Month(month+1), 0, 0, 0, 0, 0, time.UTC).Day()

		for day := 1; day <= daysInMonth; day++ {
			date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
			dateStr := date.Format("2006-01-02")

			// Skip if already assigned
			if existingDatesMap[dateStr] {
				continue
			}

			// Get round-robin prediction
			duty, err := sched.AssignDutyRoundRobin(ctx, date)
			if err != nil || duty == nil || duty.User == nil {
				continue
			}

			prognoses = append(prognoses, prognosisItem{
				Date:     dateStr,
				UserName: duty.User.FirstName,
				UserID:   duty.UserID,
			})
		}

		c.JSON(http.StatusOK, gin.H{"prognosis": prognoses})
	}
}
