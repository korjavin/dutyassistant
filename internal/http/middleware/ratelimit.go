package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// rateLimiterClient stores the rate limiter and the last access time
type rateLimiterClient struct {
	limiter    *rate.Limiter
	lastAccess time.Time
}

// IPRateLimiter stores rate limiters for each IP address
type IPRateLimiter struct {
	ips map[string]*rateLimiterClient
	mu  *sync.RWMutex
	r   rate.Limit
	b   int
}

// NewIPRateLimiter creates a new rate limiter and starts a background cleanup goroutine
func NewIPRateLimiter(r rate.Limit, b int) *IPRateLimiter {
	i := &IPRateLimiter{
		ips: make(map[string]*rateLimiterClient),
		mu:  &sync.RWMutex{},
		r:   r,
		b:   b,
	}

	// Start cleanup goroutine
	go i.cleanupStaleLimiters()

	return i
}

// cleanupStaleLimiters periodically removes entries that haven't been accessed in a while
func (i *IPRateLimiter) cleanupStaleLimiters() {
	for {
		time.Sleep(time.Minute)
		i.mu.Lock()
		for ip, client := range i.ips {
			if time.Since(client.lastAccess) > 3*time.Minute {
				delete(i.ips, ip)
			}
		}
		i.mu.Unlock()
	}
}

// GetLimiter returns the rate limiter for the provided IP address
func (i *IPRateLimiter) GetLimiter(ip string) *rate.Limiter {
	i.mu.RLock()
	client, exists := i.ips[ip]
	i.mu.RUnlock()

	if !exists {
		i.mu.Lock()
		client, exists = i.ips[ip]
		if !exists {
			client = &rateLimiterClient{
				limiter:    rate.NewLimiter(i.r, i.b),
				lastAccess: time.Now(),
			}
			i.ips[ip] = client
		} else {
			client.lastAccess = time.Now()
		}
		i.mu.Unlock()
	} else {
		i.mu.Lock()
		client.lastAccess = time.Now()
		i.mu.Unlock()
	}

	return client.limiter
}

// RateLimitMiddleware creates a new rate limiter middleware.
func RateLimitMiddleware(limiter *IPRateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		l := limiter.GetLimiter(ip)
		if !l.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded",
			})
			return
		}
		c.Next()
	}
}

// Factory functions for different endpoint types
func NewAuthLimiter() *IPRateLimiter {
	return NewIPRateLimiter(rate.Every(time.Minute/10), 10) // 10 requests per minute
}

func NewAPILimiter() *IPRateLimiter {
	return NewIPRateLimiter(rate.Every(time.Minute/100), 100) // 100 requests per minute
}

func NewPublicLimiter() *IPRateLimiter {
	return NewIPRateLimiter(rate.Every(time.Minute/1000), 1000) // 1000 requests per minute
}
