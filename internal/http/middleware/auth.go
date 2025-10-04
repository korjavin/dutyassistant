package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	initdata "github.com/telegram-mini-apps/init-data-golang"
	"github.com/korjavin/dutyassistant/internal/store"
)

// A private key for context that only this package can access. This helps
// prevent collisions with other context keys.
type contextKey string

const (
	// UserKey is the key used to store the user object in the request context.
	UserKey contextKey = "user"
)

// Authenticate is a Gin middleware that handles user authentication based on
// Telegram Web App initData. It validates the data, fetches the corresponding
// user from the application's database, and attaches the user object to the
// request context.
//
// This middleware should be applied to all endpoints that require user
// authentication. If authentication fails for any reason, it aborts the
// request with a 401 Unauthorized or 403 Forbidden status.
func Authenticate(s store.Store, botToken string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "tma" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header format must be 'tma <initData>'"})
			return
		}

		initData := parts[1]

		// Validate the initData string against the bot's token.
		// A zero expiration time disables the expiration check, which is suitable for many server-side validation scenarios.
		if err := initdata.Validate(initData, botToken, 0); err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid authentication data"})
			return
		}

		data, err := initdata.Parse(initData)
		// A valid user from Telegram always has a non-zero ID.
		// If parsing fails or the user ID is zero, the data is invalid.
		if err != nil || data.User.ID == 0 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Failed to parse or validate authentication data"})
			return
		}

		// Fetch the user from our application's database using their Telegram ID.
		user, err := s.GetUserByTelegramID(c.Request.Context(), data.User.ID)
		if err != nil {
			// This can happen if the user is not registered in our system or if there's a database error.
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "User not found or database error"})
			return
		}

		// Ensure the user is marked as active in the system.
		if !user.IsActive {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "User is inactive"})
			return
		}

		// Store the user object in the request context for use by subsequent handlers.
		ctx := context.WithValue(c.Request.Context(), UserKey, user)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// AdminRequired is a middleware that checks if the authenticated user has admin
// privileges. It must be used *after* the Authenticate middleware in the chain.
// If the user is not an admin, it aborts the request with a 403 Forbidden status.
func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := c.Request.Context().Value(UserKey).(*store.User)
		if !ok || user == nil {
			// This should theoretically not be reached if Authenticate runs first, but it's a critical safeguard.
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authentication failed or user not found in context"})
			return
		}

		if !user.IsAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Admin privileges required"})
			return
		}

		c.Next()
	}
}