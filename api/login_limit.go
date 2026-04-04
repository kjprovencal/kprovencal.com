package main

import (
	"net/http"
	"sync"
	"time"
)

// loginAttemptLimiter limits POST /admin/login per client key (see requestRemoteAddr).
// Not distributed — use a reverse-proxy limiter in production if you run multiple API instances.
type loginAttemptLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
}

func newLoginAttemptLimiter() *loginAttemptLimiter {
	return &loginAttemptLimiter{attempts: make(map[string][]time.Time)}
}

const (
	loginRateWindow = time.Minute
	loginRateMax    = 30 // attempts per window per IP (or X-Forwarded-For hop)
)

func (l *loginAttemptLimiter) allow(key string) bool {
	if key == "" {
		key = "unknown"
	}
	now := time.Now()
	cutoff := now.Add(-loginRateWindow)

	l.mu.Lock()
	defer l.mu.Unlock()

	times := l.attempts[key]
	kept := times[:0]
	for _, t := range times {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= loginRateMax {
		l.attempts[key] = kept
		return false
	}
	kept = append(kept, now)
	l.attempts[key] = kept
	return true
}

func writeTooManyLoginAttempts(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusTooManyRequests)
	_, _ = w.Write([]byte(`{"error":"too many login attempts"}`))
}
