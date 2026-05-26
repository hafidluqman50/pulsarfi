package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type bucket struct {
	count     int
	resetAt   time.Time
}

type rateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	limit    int
	window   time.Duration
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	rl := &rateLimiter{
		buckets: make(map[string]*bucket),
		limit:   limit,
		window:  window,
	}
	go rl.cleanup()
	return rl
}

func (rl *rateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, b := range rl.buckets {
			if now.After(b.resetAt) {
				delete(rl.buckets, ip)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, ok := rl.buckets[ip]
	now := time.Now()

	if !ok || now.After(b.resetAt) {
		rl.buckets[ip] = &bucket{count: 1, resetAt: now.Add(rl.window)}
		return true
	}
	if b.count >= rl.limit {
		return false
	}
	b.count++
	return true
}

func RateLimit(limit int, window time.Duration) gin.HandlerFunc {
	rl := newRateLimiter(limit, window)
	return func(c *gin.Context) {
		if !rl.allow(c.ClientIP()) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}
