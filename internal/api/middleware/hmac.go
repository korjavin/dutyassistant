package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

const hmacTimestampTolerance = 5 * time.Minute

func HMACAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if secret == "" {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "DUTY_SECRET is not configured"})
			return
		}

		tsHeader := c.GetHeader("X-Timestamp")
		sigHeader := c.GetHeader("X-Signature")

		if tsHeader == "" || sigHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "X-Timestamp and X-Signature headers are required"})
			return
		}

		tsMs, err := strconv.ParseInt(tsHeader, 10, 64)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid X-Timestamp value"})
			return
		}

		ts := time.UnixMilli(tsMs)
		if diff := time.Since(ts); diff > hmacTimestampTolerance || diff < -hmacTimestampTolerance {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Request timestamp is outside the acceptable window"})
			return
		}

		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(tsHeader))
		expectedSig := hex.EncodeToString(mac.Sum(nil))

		expectedBytes, _ := hex.DecodeString(expectedSig)
		actualBytes, err := hex.DecodeString(sigHeader)
		if err != nil || !hmac.Equal(actualBytes, expectedBytes) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid signature"})
			return
		}

		c.Next()
	}
}
