package http

import (
	"time"

	"github.com/redis/go-redis/v9"
)

// EnableRedisRateLimiters replaces in-memory limiters with Redis-backed fixed-window limiters.
// This is recommended for multi-instance deployments to keep throttling behavior consistent.
func (s *Server) EnableRedisRateLimiters(client *redis.Client, ipLimit, agentLimit int, window time.Duration) {
	if client == nil {
		return
	}
	if ipLimit > 0 {
		s.ipRateLimiter = newRedisFixedWindowLimiter(client, "rl:ip", ipLimit, window)
	}
	if agentLimit > 0 {
		s.agentRateLimiter = newRedisFixedWindowLimiter(client, "rl:agent", agentLimit, window)
	}
}
