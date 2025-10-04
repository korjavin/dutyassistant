package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/korjavin/dutyassistant/internal/store"
)

// GetUsers handles the GET /api/v1/users endpoint.
// It retrieves a list of all users in the system.
// In the future, this might be split into active/inactive users,
// but for now, it returns everyone.
func GetUsers(s store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
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