package api

import (
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

func GetWho(repo domain.Repository, secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}
