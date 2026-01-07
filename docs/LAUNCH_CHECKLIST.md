# Launch Checklist - Austrian Invoice API

## Pre-Launch Requirements

### ✅ Completed

- [x] **ebInterface 6.1 Validation**
  - All schema validation errors resolved
  - Passes official Austrian validator at `e-rechnung.gv.at`
  - All mandatory fields implemented (OrderReference, InvoiceRecipientsBillerID, etc.)

- [x] **Core API Implementation**
  - Go microservice with JSON → XML conversion
  - Stripe-based authentication ("Zero-Database" method)
  - Webhook handlers for checkout completion
  - In-memory caching for API key validation

- [x] **Subscription Status Validation**
  - Queries Stripe Subscriptions API
  - Handles active, trialing, past_due statuses
  - Automatic access revocation on cancellation

- [x] **Complete Webhook Event Handling**
  - `checkout.session.completed` - API key generation
  - `customer.subscription.deleted` - Access revocation
  - `customer.subscription.updated` - Status updates
  - `invoice.payment_failed` - Payment failure tracking
  - `invoice.payment_succeeded` - Payment success handling

- [x] **Free Tier Implementation**
  - `POST /api-keys/free` endpoint
  - Usage tracking (5 invoices/month)
  - Monthly reset logic
  - Automatic usage increment

- [x] **Rate Limiting**
  - Tier-based limits (free: 10/hour, paid: 1000/hour)
  - Rate limit headers in responses
  - 429 Too Many Requests handling

- [x] **Error Handling**
  - Standardized JSON error format
  - Error code constants
  - Consistent HTTP status codes

- [x] **Input Validation**
  - VAT-ID, BIC, IBAN format validation
  - Date format validation
  - Field-level error messages

- [x] **Landing Page**
  - Tailwind CSS responsive design
  - Copy-to-clipboard API examples
  - Pricing tiers display
  - Integration with Go backend

### ⏳ Pending

#### 1. Stripe Configuration

- [ ] **Create Stripe Product**
  - Product name: "Austrian Invoice API - Business Plan"
  - Price: €29.00/month (recurring)
  - Currency: EUR
  - Description: "Unlimited ebInterface 6.1 invoice generation"

- [ ] **Create Stripe Price ID**
  - Copy the Price ID (format: `price_...`)
  - Set environment variable: `STRIPE_PRICE_ID`

- [ ] **Configure Webhook Endpoint**
  - URL: `https://api.at-invoice.at/webhook`
  - Events to listen for:
    - `checkout.session.completed`
    - `customer.subscription.created`
    - `customer.subscription.updated`
    - `customer.subscription.deleted`
    - `invoice.payment_succeeded`
    - `invoice.payment_failed`

- [ ] **Get Webhook Signing Secret**
  - Copy from Stripe Dashboard → Webhooks
  - Set environment variable: `STRIPE_WEBHOOK_SECRET`

- [ ] **Create Restricted API Key** (Optional but recommended)
  - Stripe Dashboard → Developers → API keys
  - Create restricted key with permissions:
    - `customers:read` (to search customers by metadata)
    - `customers:write` (to update customer metadata with API key)
    - `checkout_sessions:read` (to retrieve session details)
  - Use this key instead of full secret key for production

#### 2. Domain & SSL Setup

- [ ] **Purchase Domain**
  - Recommended: `at-invoice.at` or `api.at-invoice.at`
  - Registrar: Any Austrian or EU-based registrar

- [ ] **Configure DNS**
  - Point domain to Fly.io
  - Add A/CNAME records as per Fly.io documentation

- [ ] **SSL Certificate**
  - Fly.io provides automatic SSL via Let's Encrypt
  - Configure in `fly.toml` or via Fly.io dashboard
  - Ensure HTTPS-only redirect

#### 3. Email Service Integration

- [ ] **Choose Email Provider**
  - Options: SendGrid, AWS SES, Mailgun, SMTP
  - Recommended: SendGrid (free tier: 100 emails/day)

- [ ] **Configure Email Service**
  - Set environment variables:
    - `SMTP_HOST` (or provider-specific vars)
    - `SMTP_USER`
    - `SMTP_PASS`
    - `FROM_EMAIL` (e.g., `noreply@at-invoice.at`)

- [ ] **Create Email Templates**
  - API key delivery email
  - Subscription confirmation
  - Payment failure notification

- [ ] **Update `billing.go`**
  - Replace `sendAPIKeyEmail()` stub with actual email sending
  - Add error handling and retry logic

#### 4. Environment Variables

Create `.env` file or set in Fly.io secrets:

```bash
# Stripe
STRIPE_SECRET_KEY=sk_live_...
STRIPE_WEBHOOK_SECRET=whsec_...
STRIPE_PRICE_ID=price_...

# Email (if using SMTP)
SMTP_HOST=smtp.sendgrid.net
SMTP_USER=apikey
SMTP_PASS=SG....
FROM_EMAIL=noreply@at-invoice.at

# Server
PORT=8080
```

Set in Fly.io:
```bash
fly secrets set STRIPE_SECRET_KEY=sk_live_...
fly secrets set STRIPE_WEBHOOK_SECRET=whsec_...
fly secrets set STRIPE_PRICE_ID=price_...
```

#### 5. Deployment

- [ ] **Create Fly.io App**
  ```bash
  fly launch --region vie
  ```

- [ ] **Configure `fly.toml`**
  - Set region to `vie` (Vienna)
  - Configure health checks
  - Set memory/CPU limits

- [ ] **Deploy Application**
  ```bash
  fly deploy
  ```

- [ ] **Verify Deployment**
  - Test landing page: `https://at-invoice.at`
  - Test API endpoint: `POST https://api.at-invoice.at/generate`
  - Test checkout flow: `GET https://at-invoice.at/buy`

#### 6. Testing

- [ ] **Unit Tests**
  - Test JSON → XML transformation
  - Test validation logic
  - Test error handling

- [ ] **Integration Tests**
  - Test Stripe webhook handling
  - Test API key generation and storage
  - Test authentication middleware

- [ ] **End-to-End Tests**
  - Complete checkout flow
  - API key delivery
  - Invoice generation with real API key

- [ ] **Load Testing**
  - Test with 100+ concurrent requests
  - Verify response times <100ms
  - Check memory usage

#### 7. Documentation

- [ ] **API Documentation**
  - OpenAPI/Swagger specification
  - Request/response examples
  - Error code reference

- [ ] **Integration Guides**
  - PHP example
  - Python example
  - Node.js example
  - cURL examples

- [ ] **Developer Documentation**
  - Setup instructions
  - Environment variables
  - Webhook configuration

#### 8. Monitoring & Observability

- [ ] **Error Tracking**
  - Set up Sentry or similar
  - Configure error alerts

- [ ] **Uptime Monitoring**
  - Configure UptimeRobot or Pingdom
  - Set up alerts for downtime

- [ ] **Logging**
  - Structured JSON logging
  - Log aggregation (if needed)

- [ ] **Analytics**
  - Track API usage
  - Monitor Stripe webhook events
  - Track conversion rates

#### 9. Legal & Compliance

- [ ] **Terms of Service**
  - Create ToS document
  - Link from footer

- [ ] **Privacy Policy**
  - GDPR-compliant privacy policy
  - Data retention policy (zero retention)
  - Link from footer

- [ ] **Impressum** (Austrian requirement)
  - Company information
  - Contact details
  - Link from footer

#### 10. Marketing & Launch

- [ ] **Landing Page Finalization**
  - Add legal links (ToS, Privacy, Impressum)
  - Test all CTAs
  - Mobile responsiveness check

- [ ] **Content Marketing**
  - Blog post about Austrian e-invoicing
  - Developer-focused tutorials
  - Case studies (after first customers)

- [ ] **Community Outreach**
  - Austrian Shopify Experts
  - IT consultant networks
  - Austrian tech communities

- [ ] **Launch Announcement**
  - Product Hunt (if applicable)
  - Austrian tech newsletters
  - Social media (LinkedIn, Twitter)

---

## Post-Launch Tasks

- [ ] Monitor error rates and fix issues
- [ ] Collect user feedback
- [ ] Iterate on documentation
- [ ] Add usage analytics dashboard
- [ ] Implement rate limiting per tier
- [ ] Add webhook events for invoice generation

---

## Quick Start Commands

### Local Development
```bash
# Set environment variables
export STRIPE_SECRET_KEY=sk_test_...
export STRIPE_WEBHOOK_SECRET=whsec_...
export STRIPE_PRICE_ID=price_...

# Run server
go run .
```

### Production Deployment
```bash
# Set secrets
fly secrets set STRIPE_SECRET_KEY=sk_live_...
fly secrets set STRIPE_WEBHOOK_SECRET=whsec_...
fly secrets set STRIPE_PRICE_ID=price_...

# Deploy
fly deploy
```

### Testing Webhook Locally
```bash
# Use Stripe CLI
stripe listen --forward-to localhost:8080/webhook
```

---

**Last Updated**: 2026-01-XX  
**Status**: Pre-Launch (Core Complete, Infrastructure Pending)

