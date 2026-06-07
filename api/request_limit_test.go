package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWithRateLimit_BlocksExcessGETs(t *testing.T) {
	t.Parallel()
	rateLimitDisabled = false
	getRateLimiter = newSlidingWindowLimiter(defaultGetRateWindow, 3)
	postRateLimiter = newSlidingWindowLimiter(defaultPostRateWindow, 3)

	h := withRateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	const ip = "192.0.2.9"
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
		req.RemoteAddr = ip + ":1234"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("request %d: status %d", i, rec.Code)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	req.RemoteAddr = ip + ":1234"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status %d want 429", rec.Code)
	}
}

func TestWithRateLimit_HealthzExempt(t *testing.T) {
	t.Parallel()
	rateLimitDisabled = false
	getRateLimiter = newSlidingWindowLimiter(defaultGetRateWindow, 1)
	postRateLimiter = newSlidingWindowLimiter(defaultPostRateWindow, 1)

	h := withRateLimit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	const ip = "192.0.2.10"
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		req.RemoteAddr = ip + ":1234"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("healthz %d: status %d", i, rec.Code)
		}
	}
}
