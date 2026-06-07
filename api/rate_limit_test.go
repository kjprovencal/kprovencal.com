package main

import (
	"testing"
	"time"
)

func TestSlidingWindowLimiter_AllowsUpToMax(t *testing.T) {
	t.Parallel()
	l := newSlidingWindowLimiter(time.Minute, 5)
	const ip = "192.0.2.1"
	for i := 0; i < 5; i++ {
		if !l.allow(ip) {
			t.Fatalf("expected allow at iteration %d", i)
		}
	}
	if l.allow(ip) {
		t.Fatal("expected throttle after max")
	}
}

func TestSlidingWindowLimiter_TracksKeysIndependently(t *testing.T) {
	t.Parallel()
	l := newSlidingWindowLimiter(time.Minute, 2)
	if !l.allow("192.0.2.1") || !l.allow("192.0.2.1") {
		t.Fatal("expected two allows for first IP")
	}
	if l.allow("192.0.2.1") {
		t.Fatal("first IP should be blocked")
	}
	if !l.allow("192.0.2.2") {
		t.Fatal("second IP should still be allowed")
	}
}
