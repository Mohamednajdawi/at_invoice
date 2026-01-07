package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/customer"
)

// apiKeyCache stores validated API keys with expiration
type apiKeyCache struct {
	mu    sync.RWMutex
	keys  map[string]cacheEntry
	ttl   time.Duration
	cleanupInterval time.Duration
}

type cacheEntry struct {
	valid      bool
	expiresAt  time.Time
	customerID string
}

// newAPIKeyCache creates a new cache with TTL and cleanup
func newAPIKeyCache(ttl time.Duration) *apiKeyCache {
	c := &apiKeyCache{
		keys:            make(map[string]cacheEntry),
		ttl:             ttl,
		cleanupInterval: 1 * time.Minute,
	}
	
	// Start background cleanup goroutine
	go c.cleanup()
	
	return c
}

// cleanup removes expired entries periodically
func (c *apiKeyCache) cleanup() {
	ticker := time.NewTicker(c.cleanupInterval)
	defer ticker.Stop()
	
	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, entry := range c.keys {
			if now.After(entry.expiresAt) {
				delete(c.keys, key)
			}
		}
		c.mu.Unlock()
	}
}

// get retrieves a cache entry if valid and not expired
func (c *apiKeyCache) get(key string) (bool, string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	entry, exists := c.keys[key]
	if !exists {
		return false, ""
	}
	
	if time.Now().After(entry.expiresAt) {
		return false, ""
	}
	
	return entry.valid, entry.customerID
}

// set stores a cache entry with expiration
func (c *apiKeyCache) set(key string, valid bool, customerID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.keys[key] = cacheEntry{
		valid:      valid,
		expiresAt:  time.Now().Add(c.ttl),
		customerID: customerID,
	}
}

// global cache instance (5 minute TTL)
var apiKeyCacheInstance = newAPIKeyCache(5 * time.Minute)

// generateAPIKey creates a secure random API key in format at_live_...
func generateAPIKey() (string, error) {
	bytes := make([]byte, 32) // 64 hex characters
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return "at_live_" + hex.EncodeToString(bytes), nil
}

// findCustomerByAPIKey queries Stripe for a customer with matching API key in metadata
func findCustomerByAPIKey(ctx context.Context, apiKey string) (*stripe.Customer, error) {
	params := &stripe.CustomerListParams{}
	params.Filters.AddFilter("metadata[api_key]", "", apiKey)
	params.Context = ctx
	
	iter := customer.List(params)
	if iter.Err() != nil {
		return nil, fmt.Errorf("stripe customer list error: %w", iter.Err())
	}
	
	if iter.Next() {
		return iter.Customer(), nil
	}
	
	return nil, nil // No customer found
}

// checkSubscriptionStatus verifies if a customer has an active subscription
func checkSubscriptionStatus(ctx context.Context, customerID string) (bool, error) {
	// For simplicity, we'll check if customer exists and has metadata
	// In a production system, you'd check subscription status via Stripe Subscriptions API
	c, err := customer.Get(customerID, &stripe.CustomerParams{
		Context: ctx,
	})
	if err != nil {
		return false, fmt.Errorf("failed to get customer: %w", err)
	}
	
	// Check if customer has api_key in metadata (indicates they completed checkout)
	if c.Metadata != nil && c.Metadata["api_key"] != "" {
		return true, nil
	}
	
	return false, nil
}

// StripeAuthMiddleware validates API keys against Stripe customer metadata
func StripeAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-API-KEY")
		if apiKey == "" {
			http.Error(w, "unauthorized: missing X-API-KEY header", http.StatusUnauthorized)
			return
		}
		
		// Check cache first
		valid, customerID := apiKeyCacheInstance.get(apiKey)
		if valid {
			// Cache hit - allow request
			log.Printf("API key validated from cache: %s (customer: %s)", apiKey[:20]+"...", customerID)
			next.ServeHTTP(w, r)
			return
		}
		
		// Cache miss - query Stripe
		ctx := r.Context()
		cust, err := findCustomerByAPIKey(ctx, apiKey)
		if err != nil {
			log.Printf("Stripe lookup error: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		
		if cust == nil {
			// No customer found - cache negative result
			apiKeyCacheInstance.set(apiKey, false, "")
			http.Error(w, "unauthorized: invalid API key", http.StatusUnauthorized)
			return
		}
		
		// Customer found - check subscription status
		hasActiveSubscription, err := checkSubscriptionStatus(ctx, cust.ID)
		if err != nil {
			log.Printf("Subscription check error: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		
		if !hasActiveSubscription {
			apiKeyCacheInstance.set(apiKey, false, cust.ID)
			http.Error(w, "unauthorized: subscription not active", http.StatusUnauthorized)
			return
		}
		
		// Valid key - cache positive result
		apiKeyCacheInstance.set(apiKey, true, cust.ID)
		log.Printf("API key validated via Stripe: %s (customer: %s)", apiKey[:20]+"...", cust.ID)
		next.ServeHTTP(w, r)
	})
}

