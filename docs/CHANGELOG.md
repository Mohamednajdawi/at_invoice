# Changelog - Austrian Invoice API

## 2026-01-XX - Major Implementation Update

### ✅ Completed Features

#### Authentication & Authorization
- **Subscription Status Validation**: Now queries Stripe Subscriptions API to verify active subscriptions
  - Checks for `active`, `trialing`, `past_due` statuses
  - Grace period support for past_due subscriptions
  - File: `auth.go:126-150`

- **Free Tier Support**: Complete free tier implementation
  - Endpoint: `POST /api-keys/free` for free tier registration
  - Usage tracking: 5 invoices/month limit
  - Monthly reset logic
  - API key format: `at_test_...` (vs `at_live_...` for paid)
  - Files: `free_tier.go`, `auth.go:206-226`

#### Webhook Handling
- **Complete Event Coverage**: All critical Stripe events now handled
  - `customer.subscription.deleted` - Automatic access revocation
  - `customer.subscription.updated` - Status synchronization
  - `invoice.payment_failed` - Payment failure tracking
  - `invoice.payment_succeeded` - Payment success handling
  - File: `billing_webhooks.go`

#### Rate Limiting
- **Tier-Based Rate Limiting**: In-memory rate limiting with tier detection
  - Free tier: 10 requests/hour
  - Paid tier: 1000 requests/hour
  - Rate limit headers in all responses
  - 429 Too Many Requests handling
  - File: `ratelimit.go`

#### Error Handling
- **Standardized Error Format**: Consistent JSON error responses
  - Error code constants
  - Structured error messages
  - Proper HTTP status codes
  - File: `errors.go`

#### Input Validation
- **Enhanced Validation**: Comprehensive format validation
  - Austrian VAT-ID: `ATU` + 9 digits
  - BIC: 8 or 11 characters
  - IBAN: Austrian format (AT + 20 digits)
  - Date: YYYY-MM-DD format
  - Field-level error messages
  - File: `models.go:232-320`

### 📝 New Files
- `errors.go` - Standardized error handling
- `ratelimit.go` - Rate limiting middleware
- `free_tier.go` - Free tier implementation
- `billing_webhooks.go` - Additional webhook handlers

### 🔧 Modified Files
- `auth.go` - Subscription validation, free tier support
- `billing.go` - Webhook routing, tier metadata
- `main.go` - Error handling, rate limiting integration, free tier endpoint
- `models.go` - Enhanced validation with regex patterns

### ⏳ Pending
- Email Sending (requires external service - SendGrid/AWS SES/SMTP)

---

## Progress Summary

**Core Technical Components**: 6/7 completed (85.7%)  
**Remaining**: Email integration only (requires external service)

**Ready for Launch**: Yes (with manual API key delivery workaround)

