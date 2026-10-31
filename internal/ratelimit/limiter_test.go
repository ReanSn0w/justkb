package ratelimit

import (
	"testing"
	"time"
)

func TestLimitAndReset(t *testing.T) {
	t.Parallel()
	limiter := New(2, time.Minute)
	if !limiter.Allow("actor") || !limiter.Allow("actor") {
		t.Fatal("initial attempts should pass")
	}
	if limiter.Allow("actor") {
		t.Fatal("third attempt should fail")
	}
	limiter.Reset("actor")
	if !limiter.Allow("actor") {
		t.Fatal("reset attempt should pass")
	}
}
