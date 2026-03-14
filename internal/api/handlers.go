package api

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/korjavin/dutyassistant/internal/api/middleware"
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
	UserID int64  `json:"user_id" binding:"required"`
	Date   string `json:"date" binding:"required"`
}

type userResponse struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
	IsAdmin  bool   `json:"is_admin"`
}

type dutyResponse struct {
	ID             int64  `json:"id"`
	UserID         int64  `json:"user_id"`
	UserName       string `json:"user_name"`
	DutyDate       string `json:"duty_date"`
	AssignmentType string `json:"assignment_type"`
}

func GetActiveChores(repo domain.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, authenticated := c.Request.Context().Value(middleware.UserKey).(*domain.User)
		isAuthorized := authenticated && user != nil && (user.IsActive || user.IsAdmin)

		if !isAuthorized {
			c.JSON(http.StatusOK, gin.H{"chores": []choreResponse{}})
			return
		}

		chores, err := repo.GetActiveChores(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve active chores"})
			return
		}

		response := make([]choreResponse, 0, len(chores))
		for _, chore := range chores {
			userName := ""
			if chore.User != nil {
				userName = chore.User.FirstName
			}

			response = append(response, choreResponse{
				ID:          chore.ID,
				Description: chore.Description,
				AssignedAt:  chore.AssignedAt.Format(time.RFC3339),
				UserID:      chore.UserID,
				UserName:    userName,
			})
		}

		c.JSON(http.StatusOK, gin.H{"chores": response})
	}
}

func VolunteerForDuty(ds domain.DutyService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req dutyRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		dutyDate, err := time.Parse("2006-01-02", req.Date)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format"})
			return
		}

		user, ok := c.Request.Context().Value(middleware.UserKey).(*domain.User)
		if !ok || user == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication failed"})
			return
		}

		err = ds.AssignDuty(c.Request.Context(), dutyDate, user.ID, domain.AssignmentTypeVoluntary)
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
		dutyDate, err := time.Parse("2006-01-02", req.Date)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format"})
			return
		}

		err = ds.AssignDuty(c.Request.Context(), dutyDate, req.UserID, domain.AssignmentTypeAdmin)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to assign duty"})
			return
		}
		c.Status(http.StatusCreated)
	}
}

func AdminModifyDuty(ds domain.DutyService) gin.HandlerFunc {
	type modifyReq struct {
		UserID int64 `json:"user_id" binding:"required"`
	}

	return func(c *gin.Context) {
		dateStr := c.Param("date")
		dutyDate, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format"})
			return
		}

		var req modifyReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		err = ds.AssignDuty(c.Request.Context(), dutyDate, req.UserID, domain.AssignmentTypeAdmin)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to modify duty"})
			return
		}

		c.Status(http.StatusOK)
	}
}

func AdminDeleteDuty(repo domain.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		dateStr := c.Param("date")
		dutyDate, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format"})
			return
		}

		if err := repo.DeleteDuty(c.Request.Context(), dutyDate); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete duty"})
			return
		}

		c.Status(http.StatusNoContent)
	}
}

func GetSchedule(ds domain.DutyService) gin.HandlerFunc {
	return func(c *gin.Context) {
		yearStr := c.Param("year")
		monthStr := c.Param("month")

		year, err := strconv.Atoi(yearStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid year"})
			return
		}

		month, err := strconv.Atoi(monthStr)
		if err != nil || month < 1 || month > 12 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid month"})
			return
		}

		user, authenticated := c.Request.Context().Value(middleware.UserKey).(*domain.User)
		isAuthorized := authenticated && user != nil && (user.IsActive || user.IsAdmin)

		duties, err := ds.GetSchedule(c.Request.Context(), year, time.Month(month))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve schedule"})
			return
		}

		response := make([]dutyResponse, 0, len(duties))
		for _, d := range duties {
			userName := ""
			userID := int64(0)

			if d.User != nil {
				if isAuthorized {
					userName = d.User.FirstName
					userID = d.User.ID
				} else {
					userName = "***"
				}
			}

			response = append(response, dutyResponse{
				ID:             d.ID,
				UserID:         userID,
				UserName:       userName,
				DutyDate:       d.DutyDate.Format("2006-01-02"),
				AssignmentType: string(d.AssignmentType),
			})
		}

		c.JSON(http.StatusOK, response)
	}
}

func GetUsers(repo domain.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		users, err := repo.ListAllUsers(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		response := make([]userResponse, 0, len(users))
		for _, u := range users {
			response = append(response, userResponse{
				ID:       u.ID,
				Name:     u.FirstName,
				IsActive: u.IsActive,
				IsAdmin:  u.IsAdmin,
			})
		}

		c.JSON(http.StatusOK, response)
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
	deadlineUTC := deadline.UTC()
	nowUTC := now.UTC()
	deadlineDate := time.Date(deadlineUTC.Year(), deadlineUTC.Month(), deadlineUTC.Day(), 0, 0, 0, 0, time.UTC)
	nowDate := time.Date(nowUTC.Year(), nowUTC.Month(), nowUTC.Day(), 0, 0, 0, 0, time.UTC)
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

func GetWho(repo domain.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
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

		choreItems := make([]choreItem, 0)
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
