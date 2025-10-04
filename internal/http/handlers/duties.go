package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/korjavin/dutyassistant/internal/http/middleware"
	"github.com/korjavin/dutyassistant/internal/store"
)

// VolunteerForDuty handles the POST /api/v1/duties/volunteer endpoint.
// It allows an authenticated user to volunteer for duty on a specific date.
func VolunteerForDuty(s store.Store) gin.HandlerFunc {
	type request struct {
		Date string `json:"date" binding:"required"` // YYYY-MM-DD
	}

	return func(c *gin.Context) {
		var req request
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Validate date format
		dutyDate, err := time.Parse("2006-01-02", req.Date)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format, expected YYYY-MM-DD"})
			return
		}

		// Retrieve the authenticated user from the context.
		user, ok := c.Request.Context().Value(middleware.UserKey).(*store.User)
		if !ok || user == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication failed"})
			return
		}

		// Create the new duty record.
		// The business logic for checking conflicts (e.g., against admin assignments)
		// should ideally be handled within the store or a dedicated service layer.
		// For this implementation, we assume a simple create/update.
		newDuty := &store.Duty{
			UserID:         user.ID,
			DutyDate:       dutyDate,
			AssignmentType: store.AssignmentTypeVoluntary,
			CreatedAt:      time.Now().UTC(),
		}

		// Here, we might check if a duty already exists and update it, or just create.
		// A simple approach is to try deleting any existing duty for that date first.
		_ = s.DeleteDuty(c.Request.Context(), dutyDate)
		if err := s.CreateDuty(c.Request.Context(), newDuty); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to assign volunteer duty"})
			return
		}

		c.Status(http.StatusCreated)
	}
}

// AdminAssignDuty handles the POST /api/v1/duties endpoint.
// It allows an administrator to assign any user to duty on a specific date.
func AdminAssignDuty(s store.Store) gin.HandlerFunc {
	type request struct {
		UserID int64  `json:"user_id" binding:"required"`
		Date   string `json:"date" binding:"required"` // YYYY-MM-DD
	}

	return func(c *gin.Context) {
		var req request
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		dutyDate, err := time.Parse("2006-01-02", req.Date)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format, expected YYYY-MM-DD"})
			return
		}

		newDuty := &store.Duty{
			UserID:         req.UserID,
			DutyDate:       dutyDate,
			AssignmentType: store.AssignmentTypeAdmin,
			CreatedAt:      time.Now().UTC(),
		}

		// Admin assignment overwrites any existing assignment.
		_ = s.DeleteDuty(c.Request.Context(), dutyDate)
		if err := s.CreateDuty(c.Request.Context(), newDuty); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to assign duty"})
			return
		}

		c.Status(http.StatusCreated)
	}
}

// AdminModifyDuty handles the PUT /api/v1/duties/:date endpoint.
// It allows an administrator to change the user assigned to a duty on a specific date.
func AdminModifyDuty(s store.Store) gin.HandlerFunc {
	type request struct {
		UserID int64 `json:"user_id" binding:"required"`
	}

	return func(c *gin.Context) {
		date := c.Param("date")
		if _, err := time.Parse("2006-01-02", date); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format in URL, expected YYYY-MM-DD"})
			return
		}

		var req request
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Parse the date
		dutyDate, err := time.Parse("2006-01-02", date)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format"})
			return
		}

		// Fetch the existing duty to update it.
		existingDuty, err := s.GetDutyByDate(c.Request.Context(), dutyDate)
		if err != nil || existingDuty == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "No duty found for the specified date"})
			return
		}

		// Update the user ID.
		existingDuty.UserID = req.UserID
		// The assignment type is kept or could be updated to 'admin' if desired.
		// existingDuty.AssignmentType = "admin"

		if err := s.UpdateDuty(c.Request.Context(), existingDuty); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to modify duty"})
			return
		}

		c.Status(http.StatusOK)
	}
}

// AdminDeleteDuty handles the DELETE /api/v1/duties/:date endpoint.
// It allows an administrator to delete a duty assignment for a specific date.
func AdminDeleteDuty(s store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		date := c.Param("date")
		dutyDate, err := time.Parse("2006-01-02", date)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format in URL, expected YYYY-MM-DD"})
			return
		}

		if err := s.DeleteDuty(c.Request.Context(), dutyDate); err != nil {
			// This could fail if the duty doesn't exist, which might not be an error.
			// Depending on requirements, you might return 204 regardless.
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete duty"})
			return
		}

		c.Status(http.StatusNoContent)
	}
}