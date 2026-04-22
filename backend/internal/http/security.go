package http

import (
	"sync"
	"time"
)

type fixedWindowLimiter struct {
	mu       sync.Mutex
	window   time.Duration
	limit    int
	counters map[string]*windowCounter
}

type windowCounter struct {
	start time.Time
	count int
}

func newFixedWindowLimiter(limit int, window time.Duration) *fixedWindowLimiter {
	return &fixedWindowLimiter{
		window:   window,
		limit:    limit,
		counters: map[string]*windowCounter{},
	}
}

func (l *fixedWindowLimiter) allow(key string, now time.Time) bool {
	if key == "" {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	c, ok := l.counters[key]
	if !ok || now.Sub(c.start) >= l.window {
		l.counters[key] = &windowCounter{start: now, count: 1}
		return true
	}
	if c.count >= l.limit {
		return false
	}
	c.count++
	return true
}
