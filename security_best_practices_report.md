# Security Best Practices Report for Duty Assistant Bot

**Date:** 2026-03-14
**Project:** Duty Assistant Bot (Telegram Bot for On-Call Duty Management)
**Tech Stack:** Go 1.23+ (Gin framework), JavaScript (Vanilla), SQLite

---

## Executive Summary

This security review identified **1 Critical**, **4 High**, and **2 Medium** severity issues that should be addressed to improve the security posture of the Duty Assistant Bot. The most significant concerns are:

1. **Missing HTTP server timeouts** - Creates Denial of Service vulnerability
2. **DOM XSS vulnerabilities** - Potential for cross-site scripting attacks
3. **Missing security headers** - Lacks basic web security protections
4. **Missing Content Security Policy** - No defense-in-depth against XSS
5. **Missing rate limiting** - Vulnerable to brute force attacks

The application shows good security practices in several areas including proper secret management, Telegram Web App authentication, and parameterized database queries.

---

## Critical Severity Issues

### CRIT-001: HTTP Server Missing Timeouts (DoS Vulnerability)
**Rule ID:** GO-HTTP-001
**Severity:** Critical
**Location:** `cmd/roster-bot/main.go:348`

**Evidence:**
```go
srv := &http.Server{
    Addr:     ":8080",
    Handler:   router,
}
```

**Impact:** The HTTP server is created without any timeout configuration, making it vulnerable to slow HTTP attacks and resource exhaustion attacks. Attackers can hold connections open indefinitely, consume server resources, and potentially cause denial of service.

**Fix:**
```go
srv := &http.Server{
    Addr:              ":8080",
    Handler:            router,
    ReadHeaderTimeout:    10 * time.Second,
    ReadTimeout:         30 * time.Second,
    WriteTimeout:        30 * time.Second,
    IdleTimeout:         60 * time.Second,
    MaxHeaderBytes:      1 << 20, // 1MB
}
```

**Mitigation:** Deploy a reverse proxy (nginx, Traefik) with timeout configurations until the application can be fixed.

---

## High Severity Issues

### HIGH-001: DOM XSS Vulnerabilities in Frontend
**Rule ID:** JS-XSS-001
**Severity:** High
**Location:** Multiple files in `web/js/ui/`

**Evidence:**
- `web/js/ui/calendar.js:219` - `document.body.insertAdjacentHTML('beforeend', createModal(...))`
- `web/js/ui/calendar.js:250` - `target.insertAdjacentHTML('afterend', ...)`
- `web/js/ui/calendar.js:283` - `HTMLButtonElement.innerHTML = ...`
- `web/js/ui/components.js:13-19` - Template literals with user data

**Impact:** Cross-Site Scripting (XSS) vulnerabilities allow attackers to inject malicious scripts that execute in users' browsers. While current data sources are server-controlled, future features could introduce user-generated content, making these vectors exploitable.

**Fix:**
1. Replace `innerHTML` and `insertAdjacentHTML` with safe DOM APIs:
   - Use `textContent` for plain text
   - Use `createElement` and `appendChild` for structured content

2. Example fix for calendar.js:283:
```javascript
const span = document.createElement('span');
span.className = `${bgColor} ${textColor} px-1 rounded text-[10px]`;
span.textContent = shortName;
HTMLButtonElement.appendChild(span);
```

**Mitigation:** Implement Content Security Policy headers to limit XSS impact.

---

### HIGH-002: Missing Content Security Policy
**Rule ID:** JS-CSP-001
**Severity:** High
**Location:** `internal/http/server.go` (missing)

**Evidence:** No CSP headers are set in the HTTP server middleware or configuration.

**Impact:** Without Content Security Policy, the browser cannot restrict which resources can be loaded or where scripts can execute from. This significantly reduces defense-in-depth against XSS attacks and makes it harder to mitigate vulnerabilities when they occur.

**Fix:**
Add middleware to set CSP header:
```go
func securityHeadersMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self';")
        c.Header("X-Content-Type-Options", "nosniff")
        c.Header("X-Frame-Options", "DENY")
        c.Next()
    }
}
```

Apply this middleware to all routes.

**Mitigation:** Configure CSP at the reverse proxy/CDN level.

---

### HIGH-003: Missing Security Headers
**Rule ID:** GO-HTTP-004
**Severity:** High
**Location:** `internal/http/server.go`

**Evidence:** Only `Cache-Control` headers are set, no other security headers.

**Impact:** Missing standard security headers removes important layers of defense against various attack vectors including clickjacking, MIME type sniffing, and cross-origin attacks.

**Fix:**
Implement comprehensive security header middleware (see HIGH-002 fix for example):
- `Content-Security-Policy` - Controls resource loading
- `X-Content-Type-Options: nosniff` - Prevents MIME sniffing
- `X-Frame-Options: DENY` - Prevents clickjacking
- `Referrer-Policy: strict-origin-when-cross-origin` - Controls referrer information
- `Permissions-Policy` - Controls browser feature access

---

### HIGH-004: Missing Rate Limiting
**Rule ID:** GO-HTTP baseline
**Severity:** High
**Location:** `internal/http/server.go`

**Evidence:** No rate limiting middleware is applied to API endpoints.

**Impact:** API endpoints have no protection against brute force attacks, credential stuffing, or abuse. Attackers can make unlimited requests, potentially overwhelming the service or attempting to enumerate users or attack authentication mechanisms.

**Fix:**
Implement rate limiting middleware for all API endpoints:
```go
import "golang.org/x/time/rate"

func rateLimitMiddleware(limiter *rate.Limiter) gin.HandlerFunc {
    return func(c *gin.Context) {
        if !limiter.Allow() {
            c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
                "error": "Rate limit exceeded",
            })
            return
        }
        c.Next()
    }
}
```

Apply appropriate limits per endpoint type:
- Authentication endpoints: 10 requests per minute
- API endpoints: 100 requests per minute
- Public endpoints: 1000 requests per minute

---

## Medium Severity Issues

### MED-001: CORS Not Explicitly Configured
**Rule ID:** GO-HTTP-007
**Severity:** Medium
**Location:** `internal/http/server.go`

**Evidence:** No CORS middleware or configuration found in the HTTP server setup.

**Impact:** Cross-Origin Resource Sharing behavior is not explicitly controlled. This could lead to either:
1. Unauthorized cross-origin access to API resources
2. Legitimate cross-origin requests being blocked unexpectedly

**Fix:**
If cross-origin access is needed, implement explicit CORS middleware:
```go
import "github.com/gin-contrib/cors"

func corsMiddleware() gin.HandlerFunc {
    return cors.New(cors.Config{
        AllowOrigins:     []string{"https://yourdomain.com"},
        AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
        AllowHeaders:     []string{"Content-Type", "Authorization"},
        ExposeHeaders:    []string{"Content-Length"},
        AllowCredentials: false,
        MaxAge:           12 * time.Hour,
    })
}
```

If cross-origin access is not needed, explicitly disable CORS or ensure the application works with same-origin policy.

---

### MED-002: Missing Input Validation in Frontend
**Rule ID:** JS-XSS-001 (related)
**Severity:** Medium
**Location:** `web/js/ui/components.js`

**Evidence:**
```javascript
export function createUserBadge(user) {
  return `
    <div class="inline-flex items-center bg-gray-200 rounded-full px-3 py-1 text-sm font-semibold text-gray-700 mr-2 mb-2">
      ${user.avatarUrl ? `<img src="${user.avatarUrl}" class="w-6 h-6 rounded-full mr-2" alt="${user.name}">` : ''}
      <span>${user.name}</span>
    </div>
  `;
}
```

**Impact:** User-provided data (names, avatars) is directly inserted into HTML without escaping or validation. While current data sources are controlled, this pattern creates a template for future vulnerabilities if user-generated content is added.

**Fix:**
Implement proper HTML escaping for all user-generated content:
```javascript
function escapeHtml(unsafe) {
    return unsafe
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;")
        .replace(/'/g, "&#039;");
}

export function createUserBadge(user) {
  const safeName = escapeHtml(user.name);
  const safeAvatar = user.avatarUrl ? escapeHtml(user.avatarUrl) : '';
  return `
    <div class="inline-flex items-center bg-gray-200 rounded-full px-3 py-1 text-sm font-semibold text-gray-700 mr-2 mb-2">
      ${safeAvatar ? `<img src="${safeAvatar}" class="w-6 h-6 rounded-full mr-2" alt="${safeName}">` : ''}
      <span>${safeName}</span>
    </div>
  `;
}
```

---

## Positive Security Findings

The application demonstrates several good security practices:

1. **Secrets Management:** Secrets are loaded from environment variables, not hard-coded in source code
2. **Authentication:** Proper Telegram Web App authentication using validated init data
3. **Database Security:** Uses parameterized SQLite queries, preventing SQL injection
4. **HMAC Authentication:** Secure HMAC-SHA256 authentication for the `/who` endpoint with timestamp validation
5. **Authorization:** Proper middleware-based authorization checks for admin functions
6. **No CSRF Risk:** Application doesn't use cookies for authentication, making CSRF not applicable
7. **User Activation:** Checks for user active status before allowing access
8. **Logging:** Structured logging with appropriate levels for security-relevant events
9. **Error Handling:** Proper error handling that doesn't expose sensitive information
10. **Development Mode:** Gin is set to release mode in production

---

## Recommendations Priority

### Immediate (Critical/High Severity)
1. **Fix HTTP server timeouts** - Add timeout configuration to prevent DoS attacks
2. **Fix DOM XSS vulnerabilities** - Replace unsafe HTML insertion methods
3. **Implement Content Security Policy** - Add CSP headers to all responses
4. **Add security headers** - Implement comprehensive security header middleware

### Short-term (Medium Severity)
1. **Implement rate limiting** - Add rate limiting to all API endpoints
2. **Explicit CORS configuration** - Define CORS policy if cross-origin access is needed

### Long-term
1. **Security testing** - Implement automated security testing in CI/CD pipeline
2. **Dependency scanning** - Add `govulncheck` to CI/CD workflow
3. **Input validation** - Implement comprehensive input validation for all user inputs
4. **Security monitoring** - Add logging and monitoring for security-relevant events

---

## Conclusion

The Duty Assistant Bot has a solid foundation with good authentication and database security practices. However, it lacks several important security hardening measures that are considered baseline requirements for production web applications. Addressing the identified issues, particularly the HTTP server timeouts and XSS vulnerabilities, should significantly improve the security posture of the application.

The absence of time-based DoS protections and proper content security headers represents the most significant risks. These should be addressed before the application is deployed to production environments with broad internet access.

**Report generated using security best practices guidelines for Go (backend) and JavaScript (frontend) frameworks.**