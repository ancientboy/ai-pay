package http

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type rateLimiter interface {
	allow(key string, now time.Time) bool
}

type fixedWindowLimiter struct {
	mu       sync.Mutex
	window   time.Duration
	limit    int
	counters map[string]*windowCounter
	lastGC   time.Time
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
		lastGC:   time.Now().UTC(),
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
		l.gc(now)
		return true
	}
	if c.count >= l.limit {
		l.gc(now)
		return false
	}
	c.count++
	l.gc(now)
	return true
}

func (l *fixedWindowLimiter) gc(now time.Time) {
	if now.Sub(l.lastGC) < l.window {
		return
	}
	expireBefore := now.Add(-2 * l.window)
	for key, counter := range l.counters {
		if counter.start.Before(expireBefore) {
			delete(l.counters, key)
		}
	}
	l.lastGC = now
}

type redisFixedWindowLimiter struct {
	client *redis.Client
	prefix string
	limit  int
	window time.Duration
}

func newRedisFixedWindowLimiter(client *redis.Client, prefix string, limit int, window time.Duration) *redisFixedWindowLimiter {
	return &redisFixedWindowLimiter{
		client: client,
		prefix: prefix,
		limit:  limit,
		window: window,
	}
}

func (l *redisFixedWindowLimiter) allow(key string, now time.Time) bool {
	if key == "" || l.client == nil {
		return true
	}
	sec := int64(l.window.Seconds())
	if sec <= 0 {
		return true
	}
	bucket := now.Unix() / sec
	redisKey := fmt.Sprintf("%s:%d:%s", l.prefix, bucket, key)
	ttl := l.window + 5*time.Second
	count, err := l.client.Incr(context.Background(), redisKey).Result()
	if err != nil {
		// Fail open to keep payment path available when cache is degraded.
		return true
	}
	if count == 1 {
		_ = l.client.Expire(context.Background(), redisKey, ttl).Err()
	}
	return count <= int64(l.limit)
}
