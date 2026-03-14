---
# Fix Security Best Practices Findings

## Overview
Implement security fixes identified in the security best practices report to address 1 Critical, 4 High, and 2 Medium severity issues. This includes adding HTTP server timeouts, security headers, rate limiting, and fixing XSS vulnerabilities in the frontend.

## Context
- Files involved:
  - `cmd/roster-bot/main.go` - HTTP server configuration
  - `internal/http/server.go` - Middleware and routing setup
  - `internal/http/middleware/` - New security middleware files
  - `web/js/ui/calendar.js` - XSS vulnerable code
  - `web/js/ui/components.js` - XSS vulnerable code
- Related patterns: Middleware follows existing pattern in `internal/http/middleware/` with gin.HandlerFunc
- Dependencies: May need `golang.org/x/time/rate` for rate limiting (already available in go.mod)

## Development Approach
- **Testing approach**: Regular (code first, then tests)
- Complete each task fully before moving to the next
- Follow existing Go test patterns using testify/assert and httptest
- **CRITICAL: every task MUST include new/updated tests**
- **CRITICAL: all tests must pass before starting next task**

## Implementation Steps

### Task 1: Fix HTTP Server Timeouts (Critical)

**Files:**
- Modify: `cmd/roster-bot/main.go`

- [ ] Add timeout configuration to http.Server struct (ReadTimeout, WriteTimeout, IdleTimeout, ReadHeaderTimeout, MaxHeaderBytes)
- [ ] Add unit test for HTTP server timeout configuration
- [ ] Run Go tests: `go test ./cmd/roster-bot/` - must pass before task 2

### Task 2: Add Security Headers Middleware (High)

**Files:**
- Create: `internal/http/middleware/security.go`
- Modify: `internal/http/server.go`

- [ ] Create securityHeadersMiddleware function in new security.go file
- [ ] Add Content-Security-Policy header with strict directives
- [ ] Add X-Content-Type-Options: nosniff
- [ ] Add X-Frame-Options: DENY
- [ ] Add Referrer-Policy: strict-origin-when-cross-origin
- [ ] Add Permissions-Policy for geolocation, camera, microphone
- [ ] Apply securityHeadersMiddleware to all routes in server.go
- [ ] Add unit tests for security headers middleware using httptest
- [ ] Run Go tests: `go test ./internal/http/middleware/` - must pass before task 3

### Task 3: Add Rate Limiting Middleware (High)

**Files:**
- Create: `internal/http/middleware/ratelimit.go`
- Modify: `internal/http/server.go`

- [ ] Create rateLimitMiddleware function using golang.org/x/time/rate
- [ ] Implement per-IP rate limiting with configurable limits
- [ ] Create limiter factory for different endpoint types (auth: 10/min, API: 100/min, public: 1000/min)
- [ ] Apply rate limiting to /api/v1 routes in server.go
- [ ] Add unit tests for rate limiting middleware
- [ ] Run Go tests: `go test ./internal/http/middleware/` - must pass before task 4

### Task 4: Fix XSS Vulnerabilities in calendar.js (High)

**Files:**
- Modify: `web/js/ui/calendar.js`

- [ ] Replace innerHTML usage on line 283 with safe DOM APIs (createElement, textContent, appendChild)
- [ ] Replace insertAdjacentHTML usage on lines 219 and 250 with safe DOM methods
- [ ] Test calendar rendering manually in browser to verify functionality preserved
- [ ] Create setup for frontend testing (Jest or Vitest)
- [ ] Add unit tests for calendar UI functions to verify XSS protection
- [ ] Run frontend tests: `npm test` - must pass before task 5

### Task 5: Fix XSS Vulnerabilities in components.js (High)

**Files:**
- Modify: `web/js/ui/components.js`

- [ ] Add escapeHtml utility function for HTML escaping
- [ ] Update createUserBadge to use escaped values
- [ ] Update createDutyCard to use escaped values
- [ ] Update createModal to use escaped title
- [ ] Add unit tests for escapeHtml function
- [ ] Add unit tests for component functions with XSS injection attempts
- [ ] Run frontend tests: `npm test` - must pass before task 6

### Task 6: Configure CORS Policy (Medium)

**Files:**
- Modify: `internal/http/server.go`

- [ ] Determine if cross-origin access is needed
- [ ] If not needed: document that same-origin policy is used
- [ ] If needed: add CORS middleware using github.com/gin-contrib/cors
- [ ] Configure explicit allow-list for origins
- [ ] Add unit tests for CORS behavior
- [ ] Run Go tests: `go test ./internal/http/` - must pass before task 7

### Task 7: Verify All Security Fixes Work Together

- [ ] manual test: Start server and verify all security headers are present in responses
- [ ] manual test: Test rate limiting with rapid requests
- [ ] manual test: Verify frontend UI still functions correctly after XSS fixes
- [ ] manual test: Open browser DevTools and verify CSP is enforced
- [ ] run full test suite: `go test ./...` - must pass before task 8
- [ ] run frontend tests: `npm test` - must pass before task 8
- [ ] verify test coverage meets 80%+: `go test -cover ./...`

### Task 8: Update Documentation

- [ ] update README.md with security hardening notes
- [ ] update CLAUDE.md with security middleware patterns
- [ ] move this plan to `docs/plans/completed/`
- [ ] delete security_best_practices_report.md (no longer needed after fixes)
