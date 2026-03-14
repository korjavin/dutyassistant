package middleware

import (
	"context"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/korjavin/dutyassistant/internal/domain"
	initdata "github.com/telegram-mini-apps/init-data-golang"
)

type contextKey string

const (
	UserKey contextKey = "user"
)

func Authenticate(repo domain.Repository, botToken string) gin.HandlerFunc {
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

		if err := initdata.Validate(initData, botToken, 0); err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid authentication data"})
			return
		}

		data, err := initdata.Parse(initData)
		if err != nil || data.User.ID == 0 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Failed to parse or validate authentication data"})
			return
		}

		user, err := repo.GetUserByTelegramID(c.Request.Context(), data.User.ID)
		if err != nil || user == nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "User not found or database error"})
			return
		}

		if !user.IsActive {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "User is inactive"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), UserKey, user)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := c.Request.Context().Value(UserKey).(*domain.User)
		if !ok || user == nil {
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

func OptionalAuth(repo domain.Repository, botToken string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			log.Println("[WEB_AUTH] No Authorization header present")
			c.Next()
			return
		}

		log.Printf("[WEB_AUTH] Authorization header received (length: %d)", len(authHeader))

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "tma" {
			log.Printf("[WEB_AUTH] Invalid auth format: parts=%d, scheme=%s", len(parts), parts[0])
			c.Next()
			return
		}

		initData := parts[1]
		log.Printf("[WEB_AUTH] Validating initData (length: %d)", len(initData))

		if err := initdata.Validate(initData, botToken, 0); err != nil {
			log.Printf("[WEB_AUTH] Validation failed: %v", err)
			c.Next()
			return
		}

		data, err := initdata.Parse(initData)
		if err != nil || data.User.ID == 0 {
			log.Printf("[WEB_AUTH] Parse failed or invalid user ID: err=%v, userID=%d", err, data.User.ID)
			c.Next()
			return
		}

		log.Printf("[WEB_AUTH] Parsed successfully, user ID: %d", data.User.ID)

		user, err := repo.GetUserByTelegramID(c.Request.Context(), data.User.ID)
		if err != nil || user == nil {
			log.Printf("[WEB_AUTH] User lookup failed: err=%v, found=%v", err, user != nil)
			c.Next()
			return
		}

		log.Printf("[WEB_AUTH] User authenticated: ID=%d, Name=%s, IsActive=%v", user.ID, user.FirstName, user.IsActive)

		ctx := context.WithValue(c.Request.Context(), UserKey, user)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}
