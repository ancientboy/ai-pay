package http

import (
	"testing"
	"time"
)

func TestFixedWindowLimiterGCRemovesExpiredCounters(t *testing.T) {
	limiter := newFixedWindowLimiter(2, time.Second)
	base := time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)

	if !limiter.allow("k1", base) || !limiter.allow("k2", base) {
		t.Fatalf("expected initial allows")
	}
	if len(limiter.counters) != 2 {
		t.Fatalf("expected 2 counters, got %d", len(limiter.counters))
	}

	// Move clock forward enough to pass gc threshold and expire old keys.
	if !limiter.allow("k3", base.Add(3*time.Second)) {
		t.Fatalf("expected allow for fresh key")
	}
	if len(limiter.counters) > 1 {
		t.Fatalf("expected stale counters to be cleaned, got %d", len(limiter.counters))
	}
}

func TestRedisLimiterFailOpenWhenClientMissing(t *testing.T) {
	limiter := newRedisFixedWindowLimiter(nil, "rl:test", 1, time.Second)
	if !limiter.allow("any", time.Now().UTC()) {
		t.Fatalf("expected redis limiter fail-open when client is nil")
	}
}
