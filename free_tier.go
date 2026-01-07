package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/customer"
)

// handleFreeTierSignup generates a free tier API key
func handleFreeTierSignup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, ErrCodeInternalError, "Method not allowed", "Only POST is allowed")
		return
	}
	
	// Parse email from request body
	var req struct {
		Email string `json:"email"`
	}
	
	if err := decodeJSON(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, ErrCodeInvalidJSON, "Invalid JSON", err.Error())
		return
	}
	
	if req.Email == "" {
		writeError(w, http.StatusBadRequest, ErrCodeValidationError, "Email is required", "")
		return
	}
	
	// Check if customer already exists
	stripeKey := stripe.Key
	if stripeKey == "" {
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "Stripe not configured", "")
		return
	}
	
	// Search for existing customer by email
	params := &stripe.CustomerListParams{}
	params.Filters.AddFilter("email", "", req.Email)
	
	iter := customer.List(params)
	var existingCustomer *stripe.Customer
	if iter.Next() {
		existingCustomer = iter.Customer()
	}
	
	var customerID string
	var apiKey string
	
	if existingCustomer != nil {
		// Customer exists - check if they already have a free tier key
		customerID = existingCustomer.ID
		if existingCustomer.Metadata != nil && existingCustomer.Metadata["api_key"] != "" {
			// Check if it's a free tier key
			existingKey := existingCustomer.Metadata["api_key"]
			if len(existingKey) > 7 && existingKey[:7] == "at_test_" {
				// Already has free tier key
				apiKey = existingKey
			} else {
				// Has paid key - don't override
				writeError(w, http.StatusConflict, ErrCodeInternalError, 
					"Customer already has a paid subscription", 
					"Please use your existing API key or contact support")
				return
			}
		} else {
			// Generate new free tier key
			var err error
			apiKey, err = generateAPIKey(true)
			if err != nil {
				writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "Failed to generate API key", err.Error())
				return
			}
			
			// Update customer metadata
			updateParams := &stripe.CustomerParams{}
			updateParams.AddMetadata("api_key", apiKey)
			updateParams.AddMetadata("tier", "free")
			updateParams.AddMetadata("usage_count", "0")
			updateParams.AddMetadata("usage_month", time.Now().Format("2006-01"))
			
			_, err = customer.Update(customerID, updateParams)
			if err != nil {
				writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "Failed to save API key", err.Error())
				return
			}
		}
	} else {
		// Create new customer
		custParams := &stripe.CustomerParams{
			Email: stripe.String(req.Email),
		}
		
		cust, err := customer.New(custParams)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "Failed to create customer", err.Error())
			return
		}
		
		customerID = cust.ID
		
		// Generate free tier API key
		apiKey, err = generateAPIKey(true)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "Failed to generate API key", err.Error())
			return
		}
		
		// Set metadata
		updateParams := &stripe.CustomerParams{}
		updateParams.AddMetadata("api_key", apiKey)
		updateParams.AddMetadata("tier", "free")
		updateParams.AddMetadata("usage_count", "0")
		updateParams.AddMetadata("usage_month", time.Now().Format("2006-01"))
		
		_, err = customer.Update(customerID, updateParams)
		if err != nil {
			writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "Failed to save API key", err.Error())
			return
		}
	}
	
	log.Printf("Free tier API key generated for customer %s: %s", customerID, apiKey[:20]+"...")
	
	// Send API key via email
	if req.Email != "" {
		if err := sendAPIKeyEmail(req.Email, apiKey); err != nil {
			log.Printf("Failed to send API key email: %v", err)
			// Don't fail the request if email fails
		}
	}
	
	// Return API key in response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"api_key": apiKey,
		"tier":    "free",
		"limit":    "5 invoices per month",
		"message":  "Your free tier API key has been generated. Check your email for details.",
	})
}

// checkFreeTierUsage checks if free tier customer has exceeded monthly limit
func checkFreeTierUsage(ctx context.Context, customerID string) (bool, int, error) {
	c, err := customer.Get(customerID, nil)
	if err != nil {
		return false, 0, fmt.Errorf("failed to get customer: %w", err)
	}
	
	if c.Metadata == nil {
		return true, 0, nil // No usage tracked yet
	}
	
	// Check if usage month matches current month
	currentMonth := time.Now().Format("2006-01")
	usageMonth := c.Metadata["usage_month"]
	
	if usageMonth != currentMonth {
		// New month - reset usage
		return true, 0, nil
	}
	
	// Get current usage count
	usageCountStr := c.Metadata["usage_count"]
	if usageCountStr == "" {
		return true, 0, nil
	}
	
	usageCount, err := strconv.Atoi(usageCountStr)
	if err != nil {
		return true, 0, nil // Default to allowed if parse fails
	}
	
	// Free tier limit: 5 invoices per month
	return usageCount < 5, usageCount, nil
}

// incrementFreeTierUsage increments the usage counter for free tier customers
func incrementFreeTierUsage(ctx context.Context, customerID string) error {
	c, err := customer.Get(customerID, nil)
	if err != nil {
		return fmt.Errorf("failed to get customer: %w", err)
	}
	
	currentMonth := time.Now().Format("2006-01")
	usageMonth := c.Metadata["usage_month"]
	
	var newCount int
	if usageMonth != currentMonth {
		// New month - reset to 1
		newCount = 1
	} else {
		// Increment existing count
		usageCountStr := c.Metadata["usage_count"]
		if usageCountStr == "" {
			newCount = 1
		} else {
			count, _ := strconv.Atoi(usageCountStr)
			newCount = count + 1
		}
	}
	
	// Update metadata
	updateParams := &stripe.CustomerParams{}
	updateParams.AddMetadata("usage_count", strconv.Itoa(newCount))
	updateParams.AddMetadata("usage_month", currentMonth)
	
	_, err = customer.Update(customerID, updateParams)
	return err
}

