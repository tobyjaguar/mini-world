// Rate limiter for API endpoints that consume LLM resources.
// Simple in-memory token bucket per IP address.
package api

import (
	"net/http"
	"strconv"
	"sync"
	"time"
)

// RateLimiter tracks request counts per IP with a sliding window.
type RateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	maxRate  int           // max requests per window
	window   time.Duration // time window
}

type bucket struct {
	tokens    int
	lastReset time.Time
}

// NewRateLimiter creates a rate limiter allowing maxRate requests per window.
func NewRateLimiter(maxRate int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		buckets: make(map[string]*bucket),
		maxRate: maxRate,
		window:  window,
	}
	// Periodic cleanup of stale entries.
	go func() {
		for {
			time.Sleep(time.Hour)
			rl.cleanup()
		}
	}()
	return rl
}

// Allow checks if the given IP is within rate limits.
// Returns true if allowed, false if rate-limited.
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, ok := rl.buckets[ip]
	now := time.Now()

	if !ok || now.Sub(b.lastReset) >= rl.window {
		rl.buckets[ip] = &bucket{tokens: rl.maxRate - 1, lastReset: now}
		return true
	}

	if b.tokens > 0 {
		b.tokens--
		return true
	}
	return false
}

// RetryAfter returns how many seconds until the window resets for this IP.
func (rl *RateLimiter) RetryAfter(ip string) int {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, ok := rl.buckets[ip]
	if !ok {
		return 0
	}
	remaining := rl.window - time.Since(b.lastReset)
	if remaining < 0 {
		return 0
	}
	return int(remaining.Seconds()) + 1
}

func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for ip, b := range rl.buckets {
		if now.Sub(b.lastReset) > 2*rl.window {
			delete(rl.buckets, ip)
		}
	}
}

// RateLimitMiddleware wraps a handler with rate limiting. Returns 429 if exceeded.
func RateLimitMiddleware(rl *RateLimiter, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		// Strip port from IP.
		if idx := len(ip) - 1; idx >= 0 {
			for i := idx; i >= 0; i-- {
				if ip[i] == ':' {
					ip = ip[:i]
					break
				}
			}
		}
		// Check X-Forwarded-For for proxied requests.
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			ip = xff
			// Take first IP if comma-separated.
			for i, c := range xff {
				if c == ',' {
					ip = xff[:i]
					break
				}
			}
		}

		if !rl.Allow(ip) {
			w.Header().Set("Retry-After", strconv.Itoa(rl.RetryAfter(ip)))
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}
