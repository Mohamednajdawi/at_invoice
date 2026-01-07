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
	
	// Public endpoints
	mux.HandleFunc("/buy", handleBuy)
	mux.HandleFunc("/success", handleSuccess)
	mux.HandleFunc("/cancel", handleCancel)
	mux.HandleFunc("/webhook", handleWebhook)
	
	// Protected endpoints (require Stripe API key)
	mux.Handle("/generate", StripeAuthMiddleware(http.HandlerFunc(generateHandler)))

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
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := validateInvoice(in); err != nil {
		http.Error(w, "validation error: "+err.Error(), http.StatusBadRequest)
		return
	}

	xmlBytes, err := TransformToEbInterface(in)
	if err != nil {
		http.Error(w, "transform error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	if _, err := w.Write(xmlBytes); err != nil {
		log.Printf("write response error: %v", err)
	}
}


