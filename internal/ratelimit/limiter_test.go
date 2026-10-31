package ratelimit

import (
	"testing"
	"time"
)

func TestLimitAndReset(t *testing.T) {
	t.Parallel()
	limiter := New(2, time.Minute)
	for attempt := 1; attempt <= 2; attempt++ {
		if !limiter.Allow("actor") {
			t.Fatalf("initial attempt %d should pass", attempt)
		}
	}
	if limiter.Allow("actor") {
		t.Fatal("third attempt should fail")
	}
	limiter.Reset("actor")
	if !limiter.Allow("actor") {
		t.Fatal("reset attempt should pass")
	}
}
