package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/korjavin/dutyassistant/internal/http/middleware"
	"github.com/korjavin/dutyassistant/internal/store"
)

// GetUsers handles the GET /api/v1/users endpoint.
// Returns empty list for unauthenticated users.
func GetUsers(s store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if user is authenticated
		user, authenticated := c.Request.Context().Value(middleware.UserKey).(*store.User)
		// Allow admins or active users
		isAuthorized := authenticated && user != nil && (user.IsActive || user.IsAdmin)

		// Return empty list for unauthorized users
		if !isAuthorized {
			c.JSON(http.StatusOK, []*store.User{})
			return
		}

		users, err := s.ListAllUsers(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve users"})
			return
		}

		// To avoid returning `null` for an empty slice, we initialize it.
		if users == nil {
			users = []*store.User{}
		}

		c.JSON(http.StatusOK, users)
	}
}