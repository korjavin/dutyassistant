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

		// Generate prognosis for only the next unassigned day
		type prognosisItem struct {
			Date     string `json:"date"`
			UserName string `json:"user_name"`
			UserID   int64  `json:"user_id"`
		}

		var prognoses []prognosisItem
		daysInMonth := time.Date(year, time.Month(month+1), 0, 0, 0, 0, 0, time.UTC).Day()
		now := time.Now()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

		// Find only the next unassigned day (don't store it, just predict)
		for day := 1; day <= daysInMonth; day++ {
			date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
			dateStr := date.Format("2006-01-02")

			// Skip past dates
			if date.Before(today) {
				continue
			}

			// Skip if already assigned
			if existingDatesMap[dateStr] {
				continue
			}

			// Get round-robin prediction for just this one day (without storing it)
			nextUser, err := s.GetNextRoundRobinUser(ctx)
			if err != nil || nextUser == nil {
				continue
			}

			prognoses = append(prognoses, prognosisItem{
				Date:     dateStr,
				UserName: nextUser.FirstName,
				UserID:   nextUser.ID,
			})

			// Only show one day of prognosis
			break
		}

		c.JSON(http.StatusOK, gin.H{"prognosis": prognoses})
	}
}
