package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
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
	case "customer.subscription.deleted":
		if err := handleSubscriptionDeleted(event); err != nil {
			log.Printf("Error handling customer.subscription.deleted: %v", err)
			// Don't fail webhook, just log
		}
	case "customer.subscription.updated":
		if err := handleSubscriptionUpdated(event); err != nil {
			log.Printf("Error handling customer.subscription.updated: %v", err)
			// Don't fail webhook, just log
		}
	case "invoice.payment_failed":
		if err := handlePaymentFailed(event); err != nil {
			log.Printf("Error handling invoice.payment_failed: %v", err)
			// Don't fail webhook, just log
		}
	case "invoice.payment_succeeded":
		if err := handlePaymentSucceeded(event); err != nil {
			log.Printf("Error handling invoice.payment_succeeded: %v", err)
			// Don't fail webhook, just log
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
	// In Stripe CheckoutSession, Customer can be a string ID or Customer object
	var customerID string
	var customerEmail string
	
	// Get customer ID - in CheckoutSession, Customer is a pointer to Customer object
	if sess.Customer != nil {
		customerID = sess.Customer.ID
		customerEmail = sess.Customer.Email
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

	// Generate API key (paid tier)
	apiKey, err := generateAPIKey(false)
	if err != nil {
		return fmt.Errorf("failed to generate API key: %w", err)
	}

	// Update customer metadata with API key and tier
	updateParams := &stripe.CustomerParams{}
	updateParams.AddMetadata("api_key", apiKey)
	updateParams.AddMetadata("tier", "paid")
	
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

// sendAPIKeyEmail sends the API key to the user via SendGrid
func sendAPIKeyEmail(email, apiKey string) error {
	sendGridAPIKey := os.Getenv("SENDGRID_API_KEY")
	fromEmail := os.Getenv("FROM_EMAIL")
	
	// Fallback if FROM_EMAIL not set
	if fromEmail == "" {
		fromEmail = "noreply@at-invoice.at"
	}
	
	// If SendGrid not configured, log and return (don't fail)
	if sendGridAPIKey == "" {
		log.Printf("SENDGRID_API_KEY not set - email not sent to %s", email)
		log.Printf("API Key for %s: %s", email, apiKey)
		return nil
	}
	
	// Create email message
	from := mail.NewEmail("AT-Invoice", fromEmail)
	to := mail.NewEmail("", email)
	subject := "Your Austrian Invoice API Key"
	
	// HTML email body
	htmlContent := fmt.Sprintf(`
		<!DOCTYPE html>
		<html>
		<head>
			<meta charset="UTF-8">
			<style>
				body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
				.container { max-width: 600px; margin: 0 auto; padding: 20px; }
				.header { background-color: #dc2626; color: white; padding: 20px; text-align: center; }
				.content { padding: 20px; background-color: #f9fafb; }
				.api-key { background-color: #1e293b; color: #60a5fa; padding: 15px; border-radius: 5px; font-family: monospace; font-size: 14px; word-break: break-all; margin: 20px 0; }
				.footer { padding: 20px; text-align: center; color: #6b7280; font-size: 12px; }
				.button { display: inline-block; padding: 12px 24px; background-color: #dc2626; color: white; text-decoration: none; border-radius: 5px; margin: 20px 0; }
			</style>
		</head>
		<body>
			<div class="container">
				<div class="header">
					<h1>AT-Invoice API Key</h1>
				</div>
				<div class="content">
					<p>Thank you for subscribing to AT-Invoice!</p>
					<p>Your API key has been generated. Use it in the <code>X-API-KEY</code> header for all API requests.</p>
					
					<div class="api-key">%s</div>
					
					<p><strong>Important Security Notes:</strong></p>
					<ul>
						<li>Keep this API key secure and never share it publicly</li>
						<li>Use it in the <code>X-API-KEY</code> header for all requests</li>
						<li>If you suspect it's compromised, contact support immediately</li>
					</ul>
					
					<p><strong>Example Usage:</strong></p>
					<pre style="background-color: #1e293b; color: #60a5fa; padding: 15px; border-radius: 5px; overflow-x: auto;">
curl -X POST https://api.at-invoice.at/generate \\
  -H "X-API-KEY: %s" \\
  -H "Content-Type: application/json" \\
  -d '{...}'
					</pre>
					
					<p><a href="https://at-invoice.at" class="button">View Documentation</a></p>
				</div>
				<div class="footer">
					<p>AT-Invoice | Austrian ebInterface 6.1 Compliance API</p>
					<p>If you didn't request this key, please contact support.</p>
				</div>
			</div>
		</body>
		</html>
	`, apiKey, apiKey)
	
	// Plain text version
	plainTextContent := fmt.Sprintf(`
Thank you for subscribing to AT-Invoice!

Your API key has been generated: %s

Important Security Notes:
- Keep this API key secure and never share it publicly
- Use it in the X-API-KEY header for all requests
- If you suspect it's compromised, contact support immediately

Example Usage:
curl -X POST https://api.at-invoice.at/generate \\
  -H "X-API-KEY: %s" \\
  -H "Content-Type: application/json" \\
  -d '{...}'

View documentation: https://at-invoice.at

If you didn't request this key, please contact support.
	`, apiKey, apiKey)
	
	message := mail.NewSingleEmail(from, subject, to, plainTextContent, htmlContent)
	
	// Send email via SendGrid
	client := sendgrid.NewSendClient(sendGridAPIKey)
	response, err := client.Send(message)
	if err != nil {
		return fmt.Errorf("failed to send email via SendGrid: %w", err)
	}
	
	// Log response for debugging
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		log.Printf("API key email sent successfully to %s (Status: %d)", email, response.StatusCode)
	} else {
		log.Printf("SendGrid returned non-2xx status: %d, Body: %s", response.StatusCode, response.Body)
		return fmt.Errorf("SendGrid returned status %d", response.StatusCode)
	}
	
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

