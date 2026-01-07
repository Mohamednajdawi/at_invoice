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
	"github.com/stripe/stripe-go/v76/subscription"
)

// apiKeyCache stores validated API keys with expiration
type apiKeyCache struct {
	mu              sync.RWMutex
	keys            map[string]cacheEntry
	ttl             time.Duration
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

// generateAPIKey creates a secure random API key in format at_live_... or at_test_...
func generateAPIKey(isFreeTier bool) (string, error) {
	bytes := make([]byte, 32) // 64 hex characters
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	prefix := "at_live_"
	if isFreeTier {
		prefix = "at_test_"
	}
	return prefix + hex.EncodeToString(bytes), nil
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
func checkSubscriptionStatus(ctx context.Context, customerID string) (bool, string, error) {
	// Query Stripe Subscriptions API for active subscriptions
	params := &stripe.SubscriptionListParams{
		Customer: stripe.String(customerID),
		Status:   stripe.String("all"), // Get all statuses to check
	}

	iter := subscription.List(params)
	if iter.Err() != nil {
		return false, "", fmt.Errorf("stripe subscription list error: %w", iter.Err())
	}

	// Check for active, trialing, or past_due subscriptions
	for iter.Next() {
		sub := iter.Subscription()
		// Allow active, trialing, and past_due (grace period)
		if sub.Status == stripe.SubscriptionStatusActive ||
			sub.Status == stripe.SubscriptionStatusTrialing ||
			sub.Status == stripe.SubscriptionStatusPastDue {
			return true, string(sub.Status), nil
		}
	}

	// No active subscription found
	return false, "", nil
}

// getCustomerTier determines if customer is on free or paid tier
func getCustomerTier(ctx context.Context, customerID string) (string, error) {
	c, err := customer.Get(customerID, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get customer: %w", err)
	}

	if c.Metadata != nil && c.Metadata["tier"] != "" {
		return c.Metadata["tier"], nil
	}

	// Default to "paid" if tier not set (backward compatibility)
	return "paid", nil
}

// StripeAuthMiddleware validates API keys against Stripe customer metadata
func StripeAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-API-KEY")
		if apiKey == "" {
			writeError(w, http.StatusUnauthorized, ErrCodeMissingAPIKey, "Missing X-API-KEY header", "Please include your API key in the X-API-KEY header")
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
			writeError(w, http.StatusUnauthorized, ErrCodeInvalidAPIKey, "The provided API key is invalid", "")
			return
		}

		// Check if free tier key (at_test_...) - different validation
		isFreeTier := len(apiKey) > 7 && apiKey[:7] == "at_test_"

		if isFreeTier {
			// Free tier validation - check usage limits
			tier, err := getCustomerTier(ctx, cust.ID)
			if err != nil {
				log.Printf("Tier check error: %v", err)
				writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "Internal server error", "")
				return
			}

			if tier != "free" {
				apiKeyCacheInstance.set(apiKey, false, cust.ID)
				writeError(w, http.StatusUnauthorized, ErrCodeInvalidAPIKey, "Invalid API key", "")
				return
			}

			// Check monthly usage limit for free tier
			allowed, usageCount, err := checkFreeTierUsage(ctx, cust.ID)
			if err != nil {
				log.Printf("Usage check error: %v", err)
				writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "Internal server error", "")
				return
			}

			if !allowed {
				apiKeyCacheInstance.set(apiKey, false, cust.ID)
				writeError(w, http.StatusForbidden, ErrCodeInternalError,
					"Monthly limit exceeded",
					fmt.Sprintf("Free tier limit: 5 invoices per month. Current usage: %d/5", usageCount))
				return
			}

			// Free tier keys don't need subscription check
			apiKeyCacheInstance.set(apiKey, true, cust.ID)
			log.Printf("Free tier API key validated: %s (customer: %s, usage: %d/5)", apiKey[:20]+"...", cust.ID, usageCount)
			next.ServeHTTP(w, r)
			return
		}

		// Paid tier - check subscription status
		hasActiveSubscription, status, err := checkSubscriptionStatus(ctx, cust.ID)
		if status == "" {
			status = "none"
		}
		if err != nil {
			log.Printf("Subscription check error: %v", err)
			writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "Internal server error", "")
			return
		}

		if !hasActiveSubscription {
			apiKeyCacheInstance.set(apiKey, false, cust.ID)
			writeError(w, http.StatusUnauthorized, ErrCodeSubscriptionInactive, "Subscription is not active", fmt.Sprintf("Current status: %s", status))
			return
		}

		// Valid key - cache positive result
		apiKeyCacheInstance.set(apiKey, true, cust.ID)
		log.Printf("API key validated via Stripe: %s (customer: %s, status: %s)", apiKey[:20]+"...", cust.ID, status)
		next.ServeHTTP(w, r)
	})
}
