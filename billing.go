package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/checkout/session"
	"github.com/stripe/stripe-go/v76/customer"
	"github.com/stripe/stripe-go/v76/webhook"
)

const (
	businessPlanPriceID = "price_business_monthly" // Replace with your actual Stripe Price ID
	businessPlanAmount  = 2900                     // €29.00 in cents
)

// handleWebhook processes Stripe webhook events
func handleWebhook(w http.ResponseWriter, r *http.Request) {
	const maxBodySize = 65536
	body := make([]byte, maxBodySize)
	bodyLen, err := r.Body.Read(body)
	if err != nil && err.Error() != "EOF" {
		log.Printf("Error reading webhook body: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	webhookSecret := os.Getenv("STRIPE_WEBHOOK_SECRET")
	if webhookSecret == "" {
		log.Printf("Warning: STRIPE_WEBHOOK_SECRET not set, webhook verification skipped")
	}

	// Verify webhook signature
	event, err := webhook.ConstructEvent(body[:bodyLen], r.Header.Get("Stripe-Signature"), webhookSecret)
	if err != nil {
		log.Printf("Webhook signature verification failed: %v", err)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Handle the event
	switch event.Type {
	case "checkout.session.completed":
		if err := handleCheckoutCompleted(event); err != nil {
			log.Printf("Error handling checkout.session.completed: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
	default:
		log.Printf("Unhandled event type: %s", event.Type)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// handleCheckoutCompleted processes successful checkout sessions
func handleCheckoutCompleted(event stripe.Event) error {
	// Extract session ID from event
	var sessionData struct {
		Object struct {
			ID string `json:"id"`
		} `json:"object"`
	}
	if err := json.Unmarshal(event.Data.Raw, &sessionData); err != nil {
		return fmt.Errorf("failed to extract session ID: %w", err)
	}
	
	sessionID := sessionData.Object.ID
	if sessionID == "" {
		return fmt.Errorf("session ID not found in event")
	}
	
	// Retrieve full session from Stripe API
	sess, err := session.Get(sessionID, nil)
	if err != nil {
		return fmt.Errorf("failed to retrieve checkout session: %w", err)
	}

	// Get customer ID from session
	// In Stripe CheckoutSession, Customer is a string ID
	var customerID string
	var customerEmail string
	
	if sess.Customer != nil {
		// Customer might be expanded as an object or be a string
		if cust, ok := sess.Customer.(*stripe.Customer); ok {
			customerID = cust.ID
			customerEmail = cust.Email
		} else if custID, ok := sess.Customer.(string); ok {
			customerID = custID
		}
	}
	
	// For guest checkouts, create a customer from email
	if customerID == "" {
		if sess.CustomerDetails != nil && sess.CustomerDetails.Email != "" {
			params := &stripe.CustomerParams{
				Email: stripe.String(sess.CustomerDetails.Email),
			}
			cust, err := customer.New(params)
			if err != nil {
				return fmt.Errorf("failed to create customer from guest checkout: %w", err)
			}
			customerID = cust.ID
			customerEmail = cust.Email
			log.Printf("Created customer from guest checkout: %s", customerID)
		} else {
			return fmt.Errorf("no customer ID in checkout session")
		}
	} else if customerEmail == "" && sess.CustomerDetails != nil {
		// Get email from customer details if not in customer object
		customerEmail = sess.CustomerDetails.Email
	}

	// Generate API key
	apiKey, err := generateAPIKey()
	if err != nil {
		return fmt.Errorf("failed to generate API key: %w", err)
	}

	// Update customer metadata with API key
	updateParams := &stripe.CustomerParams{}
	updateParams.AddMetadata("api_key", apiKey)
	
	_, err = customer.Update(customerID, updateParams)
	if err != nil {
		return fmt.Errorf("failed to update customer metadata: %w", err)
	}

	log.Printf("API key generated and stored for customer %s: %s", customerID, apiKey[:20]+"...")

	// Send confirmation email (log for now)
	if customerEmail != "" {
		if err := sendAPIKeyEmail(customerEmail, apiKey); err != nil {
			log.Printf("Failed to send API key email: %v", err)
			// Don't fail the webhook if email fails
		}
	}

	return nil
}

// sendAPIKeyEmail logs the email action (replace with actual email sending)
func sendAPIKeyEmail(email, apiKey string) error {
	log.Printf("=== API KEY EMAIL (would send to %s) ===", email)
	log.Printf("Subject: Your Austrian Invoice API Key")
	log.Printf("Body: Your API key is: %s", apiKey)
	log.Printf("Keep this key secure and use it in the X-API-KEY header for all requests.")
	log.Printf("=========================================")
	return nil
}

// handleBuy redirects to Stripe Checkout
func handleBuy(w http.ResponseWriter, r *http.Request) {
	stripeKey := os.Getenv("STRIPE_SECRET_KEY")
	if stripeKey == "" {
		http.Error(w, "stripe not configured", http.StatusInternalServerError)
		return
	}
	stripe.Key = stripeKey

	// Get price ID from environment or use default
	priceID := os.Getenv("STRIPE_PRICE_ID")
	if priceID == "" {
		// For demo purposes, we'll create a session with amount
		// In production, use a Price ID from Stripe Dashboard
		http.Error(w, "STRIPE_PRICE_ID not configured. Please set a Stripe Price ID in environment variables.", http.StatusInternalServerError)
		return
	}

	// Create checkout session
	params := &stripe.CheckoutSessionParams{
		Mode: stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(priceID),
				Quantity: stripe.Int64(1),
			},
		},
		SuccessURL: stripe.String(getSuccessURL(r)),
		CancelURL:  stripe.String(getCancelURL(r)),
		Metadata: map[string]string{
			"service": "austrian_invoice_api",
		},
	}

	sess, err := session.New(params)
	if err != nil {
		log.Printf("Failed to create checkout session: %v", err)
		http.Error(w, "failed to create checkout session", http.StatusInternalServerError)
		return
	}

	// Redirect to Stripe Checkout
	http.Redirect(w, r, sess.URL, http.StatusSeeOther)
}

// getSuccessURL constructs the success URL for checkout
func getSuccessURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	host := r.Host
	if host == "" {
		host = "localhost:8080"
	}
	return fmt.Sprintf("%s://%s/success", scheme, host)
}

// getCancelURL constructs the cancel URL for checkout
func getCancelURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	host := r.Host
	if host == "" {
		host = "localhost:8080"
	}
	return fmt.Sprintf("%s://%s/cancel", scheme, host)
}

// handleSuccess handles successful checkout redirect
func handleSuccess(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `
		<!DOCTYPE html>
		<html>
		<head>
			<title>Payment Successful</title>
			<style>
				body { font-family: Arial, sans-serif; text-align: center; padding: 50px; }
				.success { color: green; font-size: 24px; }
			</style>
		</head>
		<body>
			<div class="success">✓ Payment Successful!</div>
			<p>Your API key has been sent to your email address.</p>
			<p>Check your inbox for your API key and start using the Austrian Invoice API.</p>
		</body>
		</html>
	`)
}

// handleCancel handles cancelled checkout redirect
func handleCancel(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `
		<!DOCTYPE html>
		<html>
		<head>
			<title>Payment Cancelled</title>
			<style>
				body { font-family: Arial, sans-serif; text-align: center; padding: 50px; }
				.cancel { color: orange; font-size: 24px; }
			</style>
		</head>
		<body>
			<div class="cancel">Payment Cancelled</div>
			<p>You can try again anytime.</p>
		</body>
		</html>
	`)
}

