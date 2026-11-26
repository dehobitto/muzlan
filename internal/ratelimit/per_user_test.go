package ratelimit

import (
	"testing"
	"time"
)

func TestPerUserCooldown(t *testing.T) {
	now := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	limiter := NewPerUser(3*time.Second, func() time.Time { return now })

	if !limiter.Allow(1) {
		t.Fatal("expected first request to be allowed")
	}
	if limiter.Allow(1) {
		t.Fatal("expected second request inside cooldown to be blocked")
	}

	now = now.Add(3 * time.Second)
	if !limiter.Allow(1) {
		t.Fatal("expected request after cooldown to be allowed")
	}
}
