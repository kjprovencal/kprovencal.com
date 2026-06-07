package main

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultGetRateWindow  = time.Minute
	defaultGetRateMax     = 300 // per IP per minute
	defaultPostRateWindow = time.Minute
	defaultPostRateMax    = 60
)

var (
	getRateLimiter    *slidingWindowLimiter
	postRateLimiter   *slidingWindowLimiter
	rateLimitDisabled bool
)

func initRateLimits() {
	if strings.TrimSpace(os.Getenv("RATE_LIMIT_DISABLED")) == "1" {
		rateLimitDisabled = true
		return
	}
	getMax := envRateLimitMax("RATE_LIMIT_GET_MAX", defaultGetRateMax)
	postMax := envRateLimitMax("RATE_LIMIT_POST_MAX", defaultPostRateMax)
	getRateLimiter = newSlidingWindowLimiter(defaultGetRateWindow, getMax)
	postRateLimiter = newSlidingWindowLimiter(defaultPostRateWindow, postMax)
}

func envRateLimitMax(key string, def int) int {
	s := strings.TrimSpace(os.Getenv(key))
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return def
	}
	if n > 10_000 {
		return 10_000
	}
	return n
}

func withRateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rateLimitDisabled {
			next.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}

		key := requestRemoteAddr(r)
		var lim *slidingWindowLimiter
		switch r.Method {
		case http.MethodGet, http.MethodHead:
			lim = getRateLimiter
		case http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		default:
			lim = postRateLimiter
		}

		if lim != nil && !lim.allow(key) {
			writeRateLimited(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeRateLimited(w http.ResponseWriter) {
	w.Header().Set("Retry-After", "60")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusTooManyRequests)
	_, _ = w.Write([]byte("Too many requests."))
}
