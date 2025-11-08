package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiter implements a simple token bucket rate limiter.
type RateLimiter struct {
	requests map[string]*bucket
	mu       sync.RWMutex
	rate     int // requests per duration
	duration time.Duration
}

type bucket struct {
	tokens    int
	lastReset time.Time
	mu        sync.Mutex
}

// NewRateLimiter creates a new rate limiter with the given rate.
// rate: maximum number of requests per duration
// duration: time window for rate limiting
func NewRateLimiter(rate int, duration time.Duration) *RateLimiter {
	rl := &RateLimiter{
		requests: make(map[string]*bucket),
		rate:     rate,
		duration: duration,
	}

	// Cleanup old entries every hour
	go rl.cleanup()

	return rl
}

// Middleware returns a Gin middleware that enforces rate limiting.
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Use client IP as the key
		key := c.ClientIP()

		if !rl.allow(key) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "Rate limit exceeded",
				"message": "Too many requests. Please try again later.",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

func (rl *RateLimiter) allow(key string) bool {
	rl.mu.Lock()
	b, exists := rl.requests[key]
	if !exists {
		b = &bucket{
			tokens:    rl.rate,
			lastReset: time.Now(),
		}
		rl.requests[key] = b
	}
	rl.mu.Unlock()

	b.mu.Lock()
	defer b.mu.Unlock()

	// Reset bucket if duration has passed
	if time.Since(b.lastReset) > rl.duration {
		b.tokens = rl.rate
		b.lastReset = time.Now()
	}

	if b.tokens > 0 {
		b.tokens--
		return true
	}

	return false
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		for key, b := range rl.requests {
			b.mu.Lock()
			if time.Since(b.lastReset) > 24*time.Hour {
				delete(rl.requests, key)
			}
			b.mu.Unlock()
		}
		rl.mu.Unlock()
	}
}
