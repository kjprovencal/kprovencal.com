package main

import (
	"net/http/httptest"
	"testing"
)

func TestTurnstileClientIP(t *testing.T) {
	t.Run("uses RemoteAddr when TRUST_PROXY is unset", func(t *testing.T) {
		t.Setenv("TRUST_PROXY", "")
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = "198.51.100.2:54321"
		r.Header.Set("X-Forwarded-For", "203.0.113.1")

		got := turnstileClientIP(r)
		if got != "198.51.100.2" {
			t.Fatalf("got %q want direct client IP", got)
		}
	})

	t.Run("uses first X-Forwarded-For when TRUST_PROXY is set", func(t *testing.T) {
		t.Setenv("TRUST_PROXY", "1")
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = "10.0.0.1:443"
		r.Header.Set("X-Forwarded-For", "203.0.113.50, 10.0.0.2")

		got := turnstileClientIP(r)
		if got != "203.0.113.50" {
			t.Fatalf("got %q want first X-Forwarded-For hop", got)
		}
	})
}
