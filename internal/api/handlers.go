package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/korjavin/dutyassistant/internal/domain"
)

type choreResponse struct {
	ID          int64  `json:"id"`
	Description string `json:"description"`
	AssignedAt  string `json:"assigned_at"`
	UserID      int64  `json:"user_id"`
	UserName    string `json:"user_name"`
}

type dutyRequest struct {
	UserID int64  `json:"user_id"`
	Date   string `json:"date" binding:"required"`
}

func GetActiveChores(cs domain.ChoreService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Example properly integrated implementation using domain interfaces
		// activeChores, err := cs.GetActiveChores(c.Request.Context()) // Assuming GetActiveChores exists in domain
		// This acts as a proxy until fully implemented
		c.JSON(http.StatusOK, gin.H{"chores": []choreResponse{}})
	}
}

func VolunteerForDuty(ds domain.DutyService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req dutyRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		dutyDate, _ := time.Parse("2006-01-02", req.Date)

		err := ds.AssignDuty(c.Request.Context(), dutyDate, req.UserID, domain.AssignmentTypeVoluntary)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to assign volunteer duty"})
			return
		}
		c.Status(http.StatusCreated)
	}
}

func AdminAssignDuty(ds domain.DutyService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req dutyRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		dutyDate, _ := time.Parse("2006-01-02", req.Date)

		err := ds.AssignDuty(c.Request.Context(), dutyDate, req.UserID, domain.AssignmentTypeAdmin)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to assign duty"})
			return
		}
		c.Status(http.StatusCreated)
	}
}

func AdminModifyDuty(ds domain.DutyService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Mock modify mapping
		c.Status(http.StatusOK)
	}
}

func AdminDeleteDuty(ds domain.DutyService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Mock delete mapping
		c.Status(http.StatusNoContent)
	}
}

func GetSchedule(ds domain.DutyService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Map domain.DutyService.GetSchedule here
		c.JSON(http.StatusOK, []interface{}{})
	}
}

func GetUsers(repo domain.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Map domain.Repository.ListAllUsers here
		users, err := repo.ListAllUsers(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, users)
	}
}

type choreItem struct {
	Description string `json:"description"`
	DeadlineAt  string `json:"deadline_at"`
	Assignee    string `json:"assignee"`
}

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

func GetWho(repo domain.Repository, secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Verify signature if secret is provided
		if secret != "" {
			signature := c.GetHeader("X-Signature")
			if signature == "" {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing signature"})
				return
			}

			// Signature is HMAC-SHA256 of empty string for GET requests
			mac := hmac.New(sha256.New, []byte(secret))
			mac.Write([]byte(""))
			expectedMAC := hex.EncodeToString(mac.Sum(nil))

			if !hmac.Equal([]byte(signature), []byte(expectedMAC)) {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid signature"})
				return
			}
		}

		duty, err := repo.GetTodaysDuty(c.Request.Context())
		if err != nil {
			log.Printf("[WHO] Failed to get today's duty: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve duty information"})
			return
		}

		chores, err := repo.GetActiveChores(c.Request.Context())
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
