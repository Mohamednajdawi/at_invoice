package main

import (
	"log"
	"net/http"
	"os"

	"github.com/stripe/stripe-go/v76"
)

func main() {
	// Initialize Stripe
	stripeKey := os.Getenv("STRIPE_SECRET_KEY")
	if stripeKey != "" {
		stripe.Key = stripeKey
		log.Println("Stripe initialized")
	} else {
		log.Println("Warning: STRIPE_SECRET_KEY not set - Stripe features disabled")
	}

	mux := http.NewServeMux()

	// Static files (landing page and assets)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.ServeFile(w, r, "static/index.html")
		} else if r.URL.Path == "/generator" {
			http.ServeFile(w, r, "static/generator.html")
		} else if r.URL.Path == "/docs" {
			http.ServeFile(w, r, "static/docs.html")
		} else if r.URL.Path == "/script.js" {
			http.ServeFile(w, r, "static/script.js")
		} else if r.URL.Path == "/logo.svg" {
			http.ServeFile(w, r, "static/logo.svg")
		} else if r.URL.Path == "/hero-illustration.svg" {
			http.ServeFile(w, r, "static/hero-illustration.svg")
		} else if r.URL.Path == "/json-to-xml.svg" {
			http.ServeFile(w, r, "static/json-to-xml.svg")
		} else if r.URL.Path == "/validation.svg" {
			http.ServeFile(w, r, "static/validation.svg")
		} else if r.URL.Path == "/speed_performance.svg" {
			http.ServeFile(w, r, "static/speed_performance.svg")
		} else if r.URL.Path == "/no-storage_security.svg" {
			http.ServeFile(w, r, "static/no-storage_security.svg")
		} else if r.URL.Path == "/api-key.svg" {
			http.ServeFile(w, r, "static/api-key.svg")
		} else if r.URL.Path == "/web-generator.svg" {
			http.ServeFile(w, r, "static/web-generator.svg")
		} else {
			http.NotFound(w, r)
		}
	})

	// Public endpoints
	mux.HandleFunc("/buy", handleBuy)
	mux.HandleFunc("/success", handleSuccess)
	mux.HandleFunc("/cancel", handleCancel)
	mux.HandleFunc("/webhook", handleWebhook)
	mux.HandleFunc("/api-keys/free", handleFreeTierSignup)

	// Protected endpoints (require Stripe API key + rate limiting)
	mux.Handle("/generate", RateLimitMiddleware(StripeAuthMiddleware(http.HandlerFunc(generateHandler))))

	addr := ":8080"
	if v := os.Getenv("PORT"); v != "" {
		addr = ":" + v
	}

	log.Printf("Starting Austrian Invoice API service on %s\n", addr)
	log.Printf("Endpoints:")
	log.Printf("  POST /generate - Generate invoice (requires X-API-KEY)")
	log.Printf("  GET  /buy - Subscribe to service")
	log.Printf("  POST /webhook - Stripe webhook handler")

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func generateHandler(w http.ResponseWriter, r *http.Request) {
	var in InvoiceJSON
	if err := decodeJSON(r.Body, &in); err != nil {
		writeError(w, http.StatusBadRequest, ErrCodeInvalidJSON, "Invalid JSON payload", err.Error())
		return
	}

	if err := validateInvoice(in); err != nil {
		writeError(w, http.StatusBadRequest, ErrCodeValidationError, "Validation failed", err.Error())
		return
	}

	// Check if free tier and increment usage
	apiKey := r.Header.Get("X-API-KEY")
	if len(apiKey) > 7 && apiKey[:7] == "at_test_" {
		// Free tier - find customer and increment usage
		ctx := r.Context()
		cust, err := findCustomerByAPIKey(ctx, apiKey)
		if err == nil && cust != nil {
			if err := incrementFreeTierUsage(ctx, cust.ID); err != nil {
				log.Printf("Failed to increment free tier usage: %v", err)
				// Don't fail the request, just log
			}
		}
	}

	xmlBytes, err := TransformToEbInterface(in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, ErrCodeInternalError, "Failed to generate invoice", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	if _, err := w.Write(xmlBytes); err != nil {
		log.Printf("write response error: %v", err)
	}
}
