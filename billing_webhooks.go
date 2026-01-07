package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/customer"
)

// handleSubscriptionDeleted revokes API access when subscription is cancelled
func handleSubscriptionDeleted(event stripe.Event) error {
	var subscription stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &subscription); err != nil {
		return fmt.Errorf("failed to unmarshal subscription: %w", err)
	}
	
	customerID := ""
	if subscription.Customer != nil {
		customerID = subscription.Customer.ID
	}
	
	if customerID == "" {
		return fmt.Errorf("no customer ID in subscription")
	}
	
	// Clear API key from metadata (revoke access)
	updateParams := &stripe.CustomerParams{}
	updateParams.AddMetadata("api_key", "")
	updateParams.AddMetadata("subscription_status", "cancelled")
	
	_, err := customer.Update(customerID, updateParams)
	if err != nil {
		return fmt.Errorf("failed to revoke API key: %w", err)
	}
	
	log.Printf("API access revoked for customer %s (subscription cancelled)", customerID)
	return nil
}

// handleSubscriptionUpdated handles subscription status changes
func handleSubscriptionUpdated(event stripe.Event) error {
	var subscription stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &subscription); err != nil {
		return fmt.Errorf("failed to unmarshal subscription: %w", err)
	}
	
	customerID := ""
	if subscription.Customer != nil {
		customerID = subscription.Customer.ID
	}
	
	if customerID == "" {
		return fmt.Errorf("no customer ID in subscription")
	}
	
	// Update subscription status in metadata
	updateParams := &stripe.CustomerParams{}
	updateParams.AddMetadata("subscription_status", string(subscription.Status))
	
	_, err := customer.Update(customerID, updateParams)
	if err != nil {
		return fmt.Errorf("failed to update subscription status: %w", err)
	}
	
	log.Printf("Subscription status updated for customer %s: %s", customerID, subscription.Status)
	return nil
}

// handlePaymentFailed handles failed payment attempts
func handlePaymentFailed(event stripe.Event) error {
	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		return fmt.Errorf("failed to unmarshal invoice: %w", err)
	}
	
	var customerID string
	if invoice.Customer != nil {
		customerID = invoice.Customer.ID
	}
	
	if customerID == "" {
		return fmt.Errorf("no customer ID in invoice")
	}
	
	// Update metadata to track payment failure
	updateParams := &stripe.CustomerParams{}
	updateParams.AddMetadata("last_payment_failed", "true")
	updateParams.AddMetadata("last_payment_failed_at", fmt.Sprintf("%d", time.Now().Unix()))
	
	_, err := customer.Update(customerID, updateParams)
	if err != nil {
		return fmt.Errorf("failed to update payment failure status: %w", err)
	}
	
	log.Printf("Payment failed for customer %s (invoice: %s)", customerID, invoice.ID)
	// Note: We don't immediately revoke access - allow grace period
	// Access will be revoked when subscription status changes to cancelled
	
	return nil
}

// handlePaymentSucceeded reactivates access if previously revoked
func handlePaymentSucceeded(event stripe.Event) error {
	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		return fmt.Errorf("failed to unmarshal invoice: %w", err)
	}
	
	var customerID string
	if invoice.Customer != nil {
		customerID = invoice.Customer.ID
	}
	
	if customerID == "" {
		return fmt.Errorf("no customer ID in invoice")
	}
	
	// Clear payment failure flags
	updateParams := &stripe.CustomerParams{}
	updateParams.AddMetadata("last_payment_failed", "false")
	
	_, err := customer.Update(customerID, updateParams)
	if err != nil {
		return fmt.Errorf("failed to update payment status: %w", err)
	}
	
	log.Printf("Payment succeeded for customer %s (invoice: %s)", customerID, invoice.ID)
	return nil
}

