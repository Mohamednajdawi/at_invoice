# Missing Features & Implementation Gaps

This document outlines the features and components that are currently missing or incomplete in the Austrian Invoice API implementation.

## 🔴 Critical Missing Features

### 1. Free Tier (Testing Plan) Implementation

**Status**: ✅ **COMPLETED**  
**Priority**: High

- **Implementation**:
  - ✅ Free tier API key generation endpoint (`POST /api-keys/free`)
  - ✅ Usage tracking per API key (invoice count per month) in Stripe metadata
  - ✅ Monthly limit enforcement (5 invoices/month) in authentication middleware
  - ✅ Automatic monthly reset logic (checks `usage_month` vs current month)
  - ✅ Storage in Stripe Customer metadata (Zero-Database approach)
  - ✅ Automatic usage increment on successful invoice generation
  - ✅ API key format: `at_test_...` (differentiates from paid `at_live_...`)
- **Files**: `free_tier.go`, `auth.go:206-226`, `main.go:72-82`

### 2. Email Sending Functionality

**Status**: Stubbed (logs only)  
**Priority**: High  
**Note**: Requires external service (SendGrid/AWS SES/SMTP)

- **Current State**: `sendAPIKeyEmail()` only logs the email content.
- **Required**:
  - Integration with email service (SendGrid, AWS SES, SMTP, or similar)
  - Email templates for API key delivery
  - Email templates for subscription confirmations
  - Error handling and retry logic
  - Environment variables: `SMTP_HOST`, `SMTP_USER`, `SMTP_PASS`, `FROM_EMAIL`

### 3. Subscription Status Validation

**Status**: ✅ **COMPLETED**  
**Priority**: Medium

- **Implementation**:
  - ✅ Queries Stripe Subscriptions API to check active subscription status
  - ✅ Handles `active`, `trialing`, and `past_due` subscriptions (grace period)
  - ✅ Webhook handlers for `customer.subscription.deleted` and `invoice.payment_failed`
  - ✅ Automatic access revocation on subscription cancellation
  - **File**: `auth.go:126-150` (checkSubscriptionStatus function)

## 🟡 Important Missing Features

### 4. API Key Management

**Status**: Partial Implementation  
**Priority**: Medium

- **Completed**:
  - ✅ Free tier key generation (`POST /api-keys/free`)
  - ✅ Automatic key revocation on subscription cancellation (via webhook)
- **Still Required**:
  - [ ] Endpoint to regenerate API key (`POST /api-keys/regenerate`)
  - [ ] Endpoint to revoke API key (`DELETE /api-keys`)
  - [ ] Endpoint to list API keys for a customer (`GET /api-keys`)
  - [ ] Admin endpoint to view all keys (requires `ADMIN_API_KEY`)
  - [ ] Key rotation support

### 5. Usage Statistics & Analytics

**Status**: Partial Implementation (Free Tier Only)  
**Priority**: Medium

- **Completed**:
  - ✅ Free tier usage tracking (count, month) in Stripe metadata
  - ✅ Automatic usage increment on invoice generation
  - ✅ Monthly reset logic
- **Still Required**:
  - [ ] Endpoint to retrieve usage stats (`GET /usage`)
  - [ ] Dashboard data endpoint for customers
  - [ ] Monthly usage reports
  - [ ] Paid tier usage tracking (optional - currently unlimited)

### 6. Rate Limiting

**Status**: ✅ **COMPLETED**  
**Priority**: Medium

- **Implementation**:
  - ✅ Rate limiting middleware (requests per hour)
  - ✅ Different limits: Free tier (10/hour), Paid tier (1000/hour)
  - ✅ Rate limit headers: `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`
  - ✅ In-memory cache with TTL (similar to API key cache)
  - ✅ 429 Too Many Requests response
  - **File**: `ratelimit.go`

### 7. Webhook Event Handling

**Status**: ✅ **COMPLETED**  
**Priority**: Medium

- **Implementation**:
  - ✅ `checkout.session.completed` - Generate API key
  - ✅ `customer.subscription.deleted` - Revoke API access
  - ✅ `customer.subscription.updated` - Update subscription status
  - ✅ `invoice.payment_succeeded` - Clear payment failure flags
  - ✅ `invoice.payment_failed` - Track payment failures (grace period)
- **Files**: `billing.go:45-75`, `billing_webhooks.go`

### 8. Self-Hosted License Management

**Status**: Not Implemented  
**Priority**: Low

- **Required**:
  - License key generation and validation
  - Endpoint for license activation
  - License expiration checking
  - Contact form handler for self-hosted inquiries

## 🟢 Nice-to-Have Features

### 9. API Documentation Endpoint

**Status**: Not Implemented  
**Priority**: Low

- **Required**:
  - OpenAPI/Swagger specification
  - Interactive API docs at `/docs` or `/swagger`
  - Example requests and responses
  - Authentication documentation

### 10. Health Check & Status Endpoint

**Status**: Not Implemented  
**Priority**: Low

- **Required**:
  - `GET /health` - Basic health check
  - `GET /status` - Service status with Stripe connectivity check
  - Metrics endpoint (optional)

### 11. Request Logging & Monitoring

**Status**: Basic (stdout logging)  
**Priority**: Low

- **Required**:
  - Structured logging (JSON format)
  - Request/response logging middleware
  - Error tracking (Sentry, Rollbar, etc.)
  - Performance monitoring

### 12. Input Validation Enhancements

**Status**: ✅ **COMPLETED**  
**Priority**: Low

- **Implementation**:
  - ✅ Comprehensive Austrian VAT-ID validation (ATU + 9 digits)
  - ✅ BIC format validation (8 or 11 characters)
  - ✅ IBAN format validation (Austrian: AT + 20 digits)
  - ✅ Date format validation (YYYY-MM-DD)
  - ✅ Better error messages with field-level details (e.g., `biller.vat_id: ...`)
  - ✅ Tax rate range validation (0-100)
- **File**: `models.go:232-320`

### 13. CORS Support

**Status**: Not Implemented  
**Priority**: Low

- **Required**:
  - CORS middleware for browser-based clients
  - Configurable allowed origins
  - Preflight request handling

### 14. API Versioning

**Status**: Not Implemented  
**Priority**: Low

- **Required**:
  - Version prefix in routes (`/v1/generate`)
  - Version negotiation via headers
  - Backward compatibility strategy

## 📋 Infrastructure & Operations

### 15. Environment Configuration

**Status**: Partial  
**Priority**: Medium

- **Missing Documentation**:
  - Complete list of required environment variables
  - Example `.env` file
  - Configuration validation on startup
  - Default values documentation

### 16. Testing Suite

**Status**: Not Implemented  
**Priority**: Medium

- **Required**:
  - Unit tests for transformer logic
  - Integration tests for API endpoints
  - Webhook handler tests (with Stripe test events)
  - Golden file tests for XML output
  - Test fixtures and mocks

### 17. Deployment Documentation

**Status**: Not Implemented  
**Priority**: Medium

- **Required**:
  - Dockerfile
  - Docker Compose setup
  - Deployment guide (Heroku, AWS, GCP, etc.)
  - CI/CD pipeline examples
  - Production checklist

### 18. Security Enhancements

**Status**: Basic  
**Priority**: Medium

- **Required**:
  - HTTPS enforcement
  - Request size limits
  - Timeout configurations
  - API key encryption at rest (if stored)
  - Security headers middleware

## 🔧 Technical Debt

### 19. Error Handling

**Status**: Basic  
**Priority**: Medium

- **Required**:
  - Consistent error response format
  - Error codes and messages
  - Proper HTTP status codes
  - Error logging with context

### 20. Code Organization

**Status**: Good  
**Priority**: Low

- **Considerations**:
  - Separate handlers into `handlers/` package
  - Move business logic to `service/` package
  - Configuration management package
  - Constants file for magic numbers

### 21. Caching Strategy

**Status**: Basic (API key cache only)  
**Priority**: Low

- **Considerations**:
  - Cache Stripe customer lookups
  - Cache subscription status
  - TTL configuration
  - Cache invalidation on webhook events

## 📝 Documentation Gaps

### 22. API Documentation

**Status**: Partial  
**Priority**: Medium

- **Missing**:
  - Complete API reference
  - Request/response examples
  - Error code reference
  - Rate limits documentation
  - Authentication guide

### 23. Integration Guides

**Status**: Not Implemented  
**Priority**: Low

- **Missing**:
  - Integration examples (PHP, Python, Node.js, etc.)
  - Webhook integration guide
  - Best practices guide
  - Troubleshooting guide

### 24. Legal & Compliance

**Status**: Not Implemented  
**Priority**: Low

- **Missing**:
  - Terms of Service
  - Privacy Policy
  - GDPR compliance documentation
  - Data retention policy

## 🎯 Recommended Implementation Order

1. **Phase 1 (Critical)**:

   - Email sending functionality
   - Free tier implementation with usage tracking
   - Subscription status validation

2. **Phase 2 (Important)**:

   - Complete webhook event handling
   - API key management endpoints
   - Rate limiting
   - Usage statistics

3. **Phase 3 (Polish)**:

   - Testing suite
   - Deployment documentation
   - API documentation endpoint
   - Security enhancements

4. **Phase 4 (Future)**:
   - Self-hosted license management
   - Advanced analytics
   - Integration guides
   - Legal documentation

---

**Last Updated**: 2026-01-XX  
**Maintainer**: Development Team
