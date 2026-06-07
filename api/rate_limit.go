package main

import (
	"sync"
	"time"
)

// slidingWindowLimiter caps events per key within a rolling time window.
// Not distributed — pair with nginx limit_req when running multiple API instances.
type slidingWindowLimiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	window time.Duration
	max    int
}

func newSlidingWindowLimiter(window time.Duration, max int) *slidingWindowLimiter {
	if max < 1 {
		max = 1
	}
	if window <= 0 {
		window = time.Minute
	}
	return &slidingWindowLimiter{
		hits:   make(map[string][]time.Time),
		window: window,
		max:    max,
	}
}

func (l *slidingWindowLimiter) allow(key string) bool {
	if key == "" {
		key = "unknown"
	}
	now := time.Now()
	cutoff := now.Add(-l.window)

	l.mu.Lock()
	defer l.mu.Unlock()

	times := l.hits[key]
	kept := times[:0]
	for _, t := range times {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= l.max {
		l.hits[key] = kept
		return false
	}
	kept = append(kept, now)
	l.hits[key] = kept
	return true
}
