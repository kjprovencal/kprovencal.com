package main

import (
	"net/http"
	"time"
)

const (
	loginRateWindow = time.Minute
	loginRateMax    = 30 // attempts per window per IP (or X-Forwarded-For hop)
)

var loginLimiter = newSlidingWindowLimiter(loginRateWindow, loginRateMax)

func writeTooManyLoginAttempts(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusTooManyRequests)
	_, _ = w.Write([]byte(`{"error":"too many login attempts"}`))
}
