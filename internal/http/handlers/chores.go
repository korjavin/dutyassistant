package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/korjavin/dutyassistant/internal/http/middleware"
	"github.com/korjavin/dutyassistant/internal/store"
)

// GetActiveChores handles the GET /api/v1/chores/active endpoint.
// It returns chores assigned but not yet confirmed as completed.
func GetActiveChores(s store.Store) gin.HandlerFunc {
	type choreResponse struct {
		ID          int64  `json:"id"`
		Description string `json:"description"`
		AssignedAt  string `json:"assigned_at"`
		UserID      int64  `json:"user_id"`
		UserName    string `json:"user_name"`
	}

	return func(c *gin.Context) {
		// Check if user is authenticated.
		user, authenticated := c.Request.Context().Value(middleware.UserKey).(*store.User)
		// Allow admins or active users.
		isAuthorized := authenticated && user != nil && (user.IsActive || user.IsAdmin)

		// Return empty list for unauthorized users.
		if !isAuthorized {
			c.JSON(http.StatusOK, gin.H{"chores": []choreResponse{}})
			return
		}

		chores, err := s.GetActiveChores(c.Request.Context())
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
