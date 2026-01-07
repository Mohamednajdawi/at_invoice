# Core Technical Gaps - What's Still Missing

This document focuses on **essential technical components** required for a production-ready launch, excluding nice-to-haves and future enhancements.

---

## 🔴 Critical Core Technical Gaps

### 1. Email Sending Implementation

**Status**: Stubbed (logs only)  
**File**: `billing.go:149-156`  
**Impact**: Users cannot receive their API keys after payment

**Current Code**:

```go
func sendAPIKeyEmail(email, apiKey string) error {
    log.Printf("=== API KEY EMAIL (would send to %s) ===", email)
    // ... just logs, doesn't send
    return nil
}
```

**Required Implementation**:

- [ ] Choose email provider (SendGrid recommended - free tier: 100/day)
- [ ] Add email package dependency (`net/smtp` or `github.com/sendgrid/sendgrid-go`)
- [ ] Implement actual email sending with HTML template
- [ ] Add environment variables: `SMTP_HOST`, `SMTP_USER`, `SMTP_PASS`, `FROM_EMAIL`
- [ ] Error handling and retry logic
- [ ] Email template for API key delivery

**Estimated Effort**: 2-3 hours

---

### 2. Subscription Status Validation

**Status**: ✅ **COMPLETED**  
**File**: `auth.go:126-150`  
**Impact**: System now verifies actual subscription status from Stripe

**Implementation**:

- [x] Query Stripe Subscriptions API: `subscription.List({ customer: customerID, status: 'all' })`
- [x] Check subscription status: `active`, `trialing`, `past_due`
- [x] Handle cancelled subscriptions (revoke access via webhook)
- [x] Handle past_due subscriptions (grace period - allows access)
- [x] Updated `StripeAuthMiddleware` to use real subscription check
- [x] Returns subscription status for logging/debugging

**Code Location**: `auth.go:126-150` (checkSubscriptionStatus function)

---

### 3. Complete Webhook Event Handling

**Status**: ✅ **COMPLETED**  
**File**: `billing.go:45-75`, `billing_webhooks.go`  
**Impact**: All critical subscription and payment events are now handled

**Implementation**:

- [x] `customer.subscription.deleted` - Revokes API access, clears metadata
- [x] `customer.subscription.updated` - Updates subscription status in metadata
- [x] `invoice.payment_failed` - Tracks payment failures in metadata (grace period allowed)
- [x] `invoice.payment_succeeded` - Clears payment failure flags
- [x] `checkout.session.completed` - Generates API key (already implemented)

**Code Location**:

- `billing.go:45-75` (webhook router)
- `billing_webhooks.go` (all handler functions)

---

### 4. Free Tier Implementation

**Status**: ✅ **COMPLETED**  
**File**: `free_tier.go`, `auth.go:206-226`  
**Impact**: Free tier is fully functional with usage tracking

**Implementation**:

- [x] Free tier API key generation endpoint (`POST /api-keys/free`)
- [x] Usage tracking in Stripe metadata: `metadata['usage_count']`, `metadata['usage_month']`
- [x] Monthly limit enforcement (5 invoices/month) in `StripeAuthMiddleware`
- [x] Monthly reset logic (checks `usage_month` vs current month)
- [x] `StripeAuthMiddleware` checks usage limits for free tier keys
- [x] Free tier key format: `at_test_...` (differentiates from paid `at_live_...`)
- [x] Automatic usage increment on successful invoice generation
- [x] Usage stored in Stripe Customer metadata (Zero-Database approach)

**Code Location**:

- `free_tier.go` (endpoint, usage checking, incrementing)
- `auth.go:206-226` (free tier validation in middleware)
- `main.go:72-82` (usage increment in generate handler)

---

## 🟡 Important Core Technical Gaps

### 5. Rate Limiting

**Status**: ✅ **COMPLETED**  
**File**: `ratelimit.go`, `main.go:41`  
**Impact**: Full rate limiting protection with tier-based limits

**Implementation**:

- [x] Rate limiting middleware (requests per hour)
- [x] In-memory cache with TTL (using similar pattern to `apiKeyCache`)
- [x] Different limits:
  - Free tier: 10 requests/hour
  - Paid tier: 1000 requests/hour
- [x] Rate limit headers: `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`
- [x] 429 Too Many Requests response with standardized error format
- [x] Automatic tier detection from API key prefix (`at_test_` vs `at_live_`)

**Code Location**:

- `ratelimit.go` (complete implementation)
- `main.go:41` (middleware integration)

---

### 6. Error Handling & Response Format

**Status**: ✅ **COMPLETED**  
**File**: `errors.go`, all handler files  
**Impact**: Consistent, developer-friendly error responses

**Implementation**:

- [x] Standardized error response format:
  ```json
  {
    "error": {
      "code": "INVALID_API_KEY",
      "message": "The provided API key is invalid",
      "details": "..."
    }
  }
  ```
- [x] Error codes constants: `ErrCodeInvalidAPIKey`, `ErrCodeMissingAPIKey`, `ErrCodeSubscriptionInactive`, `ErrCodeRateLimitExceeded`, etc.
- [x] Proper HTTP status codes (400, 401, 403, 429, 500)
- [x] All endpoints use `writeError()` function for consistency
- [x] Structured error logging

**Code Location**:

- `errors.go` (error types, constants, helper functions)
- All handlers updated to use `writeError()`

---

### 7. Input Validation Enhancements

**Status**: ✅ **COMPLETED**  
**File**: `models.go:232-320`  
**Impact**: Comprehensive validation with detailed error messages

**Implementation**:

- [x] Austrian VAT-ID format validation (ATU + 9 digits) - regex: `^ATU\d{9}$`
- [x] BIC format validation (8 or 11 characters) - regex: `^[A-Z]{4}[A-Z]{2}[A-Z0-9]{2}([A-Z0-9]{3})?$`
- [x] IBAN format validation (Austrian: AT + 20 digits) - regex: `^AT\d{2}\d{16}$`
- [x] Date format validation (YYYY-MM-DD) - uses `time.Parse`
- [x] Better error messages with field-level details (e.g., `biller.vat_id: ...`, `items[0].quantity: ...`)
- [x] Tax rate validation (0-100 range)
- [x] Helper functions: `validateVATID()`, `validateBIC()`, `validateIBAN()`, `validateDate()`

**Code Location**: `models.go:232-320` (enhanced `validateInvoice()` function)

---

## 📋 Summary: Core Technical Gaps

| #   | Component               | Priority     | Effort | Status           |
| --- | ----------------------- | ------------ | ------ | ---------------- |
| 1   | Email Sending           | 🔴 Critical  | 2-3h   | ❌ Not Started   |
| 2   | Subscription Validation | 🔴 Critical  | 2-3h   | ✅ **COMPLETED** |
| 3   | Webhook Events          | 🔴 Critical  | 3-4h   | ✅ **COMPLETED** |
| 4   | Free Tier               | 🔴 Critical  | 4-5h   | ✅ **COMPLETED** |
| 5   | Rate Limiting           | 🟡 Important | 3-4h   | ✅ **COMPLETED** |
| 6   | Error Handling          | 🟡 Important | 2-3h   | ✅ **COMPLETED** |
| 7   | Input Validation        | 🟡 Important | 2-3h   | ✅ **COMPLETED** |

**Total Estimated Effort**: 18-25 hours  
**Completed**: 6/7 components (85.7%)  
**Remaining**: Only Email Sending (requires external service)

---

## 🚀 Minimum Viable Launch (MVP)

For a basic launch, you **must** complete:

1. ⏳ **Email Sending** - Users need their API keys (REQUIRES EXTERNAL SERVICE)
2. ✅ **Subscription Validation** - ✅ COMPLETED - Prevents unauthorized access
3. ✅ **Webhook Events** - ✅ COMPLETED - Handles cancellations and payment failures

**Completed (Beyond MVP)**:

- ✅ Free tier - Fully implemented with usage tracking
- ✅ Rate limiting - Tier-based limits implemented
- ✅ Enhanced validation - Comprehensive input validation

---

## 🎯 Recommended Implementation Order

1. **Email Sending** (2-3h) - Blocking for user experience
2. **Subscription Validation** (2-3h) - Security critical
3. **Webhook Events** (3-4h) - Business logic critical
4. **Free Tier** (4-5h) - Marketing promise
5. **Rate Limiting** (3-4h) - Abuse prevention
6. **Error Handling** (2-3h) - Developer experience
7. **Input Validation** (2-3h) - Data quality

---

## Quick Wins (Low Effort, High Value)

1. ✅ **Error Response Format** - ✅ COMPLETED - All endpoints use standardized format
2. ✅ **VAT-ID Validation** - ✅ COMPLETED - Regex validation implemented
3. ✅ **Webhook Logging** - ✅ COMPLETED - All webhook events logged with context

---

## 📈 Implementation Progress

**Completed**: 6/7 components (85.7%)  
**Remaining**: Email Sending (requires external service)

### Recent Updates (2026-01-XX)

- ✅ Subscription Status Validation - Now queries Stripe Subscriptions API
- ✅ Complete Webhook Handling - All critical events implemented
- ✅ Free Tier - Full implementation with usage tracking
- ✅ Rate Limiting - Tier-based limits with headers
- ✅ Error Handling - Standardized JSON responses
- ✅ Input Validation - Comprehensive validation with regex patterns

### New Files Created

- `errors.go` - Standardized error handling
- `ratelimit.go` - Rate limiting middleware
- `free_tier.go` - Free tier implementation
- `billing_webhooks.go` - Additional webhook handlers

---

**Last Updated**: 2026-01-XX  
**Focus**: Core technical components only (excludes nice-to-haves)
