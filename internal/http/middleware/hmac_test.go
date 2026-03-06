package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// makeSignedRequest creates an HTTP request with valid HMAC headers for the given secret and timestamp.
func makeSignedRequest(secret string, tsMs int64) *http.Request {
	ts := strconv.FormatInt(tsMs, 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts))
	sig := hex.EncodeToString(mac.Sum(nil))

	req, _ := http.NewRequest("GET", "/who", nil)
	req.Header.Set("X-Timestamp", ts)
	req.Header.Set("X-Signature", sig)
	return req
}

func setupHMACRouter(secret string) *gin.Engine {
	r := gin.New()
	r.GET("/who", HMACAuth(secret), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

func TestHMACAuth_ValidRequest(t *testing.T) {
	secret := "test-secret"
	router := setupHMACRouter(secret)

	req := makeSignedRequest(secret, time.Now().UnixMilli())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHMACAuth_InvalidSignature(t *testing.T) {
	router := setupHMACRouter("test-secret")

	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	req, _ := http.NewRequest("GET", "/who", nil)
	req.Header.Set("X-Timestamp", ts)
	req.Header.Set("X-Signature", "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHMACAuth_MissingHeaders(t *testing.T) {
	router := setupHMACRouter("test-secret")

	tests := []struct {
		name string
		req  func() *http.Request
	}{
		{
			name: "no headers at all",
			req: func() *http.Request {
				r, _ := http.NewRequest("GET", "/who", nil)
				return r
			},
		},
		{
			name: "only X-Timestamp",
			req: func() *http.Request {
				r, _ := http.NewRequest("GET", "/who", nil)
				r.Header.Set("X-Timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))
				return r
			},
		},
		{
			name: "only X-Signature",
			req: func() *http.Request {
				r, _ := http.NewRequest("GET", "/who", nil)
				r.Header.Set("X-Signature", "abc123")
				return r
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, tt.req())
			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})
	}
}

func TestHMACAuth_ExpiredTimestamp(t *testing.T) {
	secret := "test-secret"
	router := setupHMACRouter(secret)

	// Timestamp 6 minutes in the past — outside the 5-minute tolerance window.
	oldTs := time.Now().Add(-6 * time.Minute).UnixMilli()
	req := makeSignedRequest(secret, oldTs)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHMACAuth_FutureTimestamp(t *testing.T) {
	secret := "test-secret"
	router := setupHMACRouter(secret)

	// Timestamp 6 minutes in the future — also outside the tolerance window.
	futureTs := time.Now().Add(6 * time.Minute).UnixMilli()
	req := makeSignedRequest(secret, futureTs)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHMACAuth_InvalidTimestampFormat(t *testing.T) {
	router := setupHMACRouter("test-secret")

	req, _ := http.NewRequest("GET", "/who", nil)
	req.Header.Set("X-Timestamp", "not-a-number")
	req.Header.Set("X-Signature", "abc123")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHMACAuth_SecretNotConfigured(t *testing.T) {
	// Empty secret → 503.
	router := setupHMACRouter("")

	req := makeSignedRequest("any-secret", time.Now().UnixMilli())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestHMACAuth_WrongSecret(t *testing.T) {
	// Request signed with "wrong-secret" but server uses "correct-secret" → 401.
	router := setupHMACRouter("correct-secret")

	req := makeSignedRequest("wrong-secret", time.Now().UnixMilli())
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestHMACAuth_SignatureIsTimingMafeEqual is a sanity check that we're not comparing
// hex strings of different lengths (which would panic in hmac.Equal without proper hex decode).
func TestHMACAuth_ShortSignature(t *testing.T) {
	router := setupHMACRouter("test-secret")

	ts := fmt.Sprintf("%d", time.Now().UnixMilli())
	req, _ := http.NewRequest("GET", "/who", nil)
	req.Header.Set("X-Timestamp", ts)
	req.Header.Set("X-Signature", "ab")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
