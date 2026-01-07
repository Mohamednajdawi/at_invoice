package main

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// rateLimitCache stores rate limit counters per API key
type rateLimitCache struct {
	mu    sync.RWMutex
	keys  map[string]rateLimitEntry
	ttl   time.Duration
	cleanupInterval time.Duration
}

type rateLimitEntry struct {
	count     int
	resetAt   time.Time
	limit     int
	remaining int
}

// newRateLimitCache creates a new rate limit cache
func newRateLimitCache(ttl time.Duration) *rateLimitCache {
	c := &rateLimitCache{
		keys:            make(map[string]rateLimitEntry),
		ttl:             ttl,
		cleanupInterval: 1 * time.Minute,
	}
	
	// Start background cleanup goroutine
	go c.cleanup()
	
	return c
}

// cleanup removes expired entries periodically
func (c *rateLimitCache) cleanup() {
	ticker := time.NewTicker(c.cleanupInterval)
	defer ticker.Stop()
	
	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, entry := range c.keys {
			if now.After(entry.resetAt) {
				delete(c.keys, key)
			}
		}
		c.mu.Unlock()
	}
}

// checkAndIncrement checks if limit is exceeded and increments counter
func (c *rateLimitCache) checkAndIncrement(key string, limit int, window time.Duration) (allowed bool, remaining int, resetAt time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	now := time.Now()
	entry, exists := c.keys[key]
	
	// Reset if window expired
	if !exists || now.After(entry.resetAt) {
		entry = rateLimitEntry{
			count:     1,
			resetAt:   now.Add(window),
			limit:     limit,
			remaining: limit - 1,
		}
		c.keys[key] = entry
		return true, entry.remaining, entry.resetAt
	}
	
	// Check if limit exceeded
	if entry.count >= limit {
		return false, 0, entry.resetAt
	}
	
	// Increment counter
	entry.count++
	entry.remaining = limit - entry.count
	c.keys[key] = entry
	
	return true, entry.remaining, entry.resetAt
}

// global rate limit cache (1 hour window)
var rateLimitCacheInstance = newRateLimitCache(1 * time.Hour)

// getRateLimit returns the rate limit for a given tier
func getRateLimit(tier string) int {
	switch tier {
	case "free":
		return 10 // 10 requests per hour for free tier
	case "paid":
		return 1000 // 1000 requests per hour for paid tier
	default:
		return 10 // Default to free tier limit
	}
}

// RateLimitMiddleware enforces rate limiting based on API key tier
func RateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-API-KEY")
		if apiKey == "" {
			next.ServeHTTP(w, r)
			return
		}
		
		// Determine tier from API key prefix
		tier := "paid"
		if len(apiKey) > 7 && apiKey[:7] == "at_test_" {
			tier = "free"
		}
		
		limit := getRateLimit(tier)
		window := 1 * time.Hour
		
		allowed, remaining, resetAt := rateLimitCacheInstance.checkAndIncrement(apiKey, limit, window)
		
		// Set rate limit headers
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetAt.Unix(), 10))
		
		if !allowed {
			writeError(w, http.StatusTooManyRequests, ErrCodeRateLimitExceeded, 
				"Rate limit exceeded", 
				fmt.Sprintf("You have exceeded the rate limit of %d requests per hour. Please try again later.", limit))
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

