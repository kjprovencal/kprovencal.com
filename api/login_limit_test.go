package main

import (
	"testing"
)

func TestLoginAttemptLimiter_AllowsUpToMax(t *testing.T) {
	t.Parallel()
	l := newLoginAttemptLimiter()
	const ip = "192.0.2.1"
	for i := 0; i < loginRateMax; i++ {
		if !l.allow(ip) {
			t.Fatalf("expected allow at iteration %d", i)
		}
	}
	if l.allow(ip) {
		t.Fatal("expected throttle after max attempts")
	}
}

func TestLoginAttemptLimiter_TracksKeysIndependently(t *testing.T) {
	t.Parallel()
	l := newLoginAttemptLimiter()
	for i := 0; i < loginRateMax; i++ {
		if !l.allow("192.0.2.1") {
			t.Fatal("unexpected block for first IP")
		}
	}
	if !l.allow("192.0.2.2") {
		t.Fatal("second IP should still be allowed")
	}
}
