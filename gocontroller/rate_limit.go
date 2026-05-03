package gocontroller

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

type rateLimitEntry struct {
	tokens     float64
	lastRefill time.Time
}

// RateLimiter implements a token bucket rate limiter.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*rateLimitEntry
	rate    float64
	burst   int
	cleanup time.Duration
	maxKeys int
	keyFunc func(*Context) string
	handler func(*Context)
}

// RateLimitConfig configures a rate limiter middleware.
type RateLimitConfig struct {
	Rate    float64
	Burst   int
	Cleanup time.Duration
	MaxKeys int
	KeyFunc func(*Context) string
	Handler func(*Context)
}

func (c *RateLimitConfig) applyDefaults() {
	if c.Rate <= 0 {
		c.Rate = 10
	}
	if c.Burst <= 0 {
		c.Burst = int(c.Rate)
	}
	if c.Cleanup <= 0 {
		c.Cleanup = 5 * time.Minute
	}
	if c.MaxKeys <= 0 {
		c.MaxKeys = 10000
	}
	if c.KeyFunc == nil {
		c.KeyFunc = func(ctx *Context) string {
			if ip := ctx.Request.Header.Get("X-Forwarded-For"); ip != "" {
				return ip
			}
			if ip := ctx.Request.Header.Get("X-Real-IP"); ip != "" {
				return ip
			}
			return ctx.Request.RemoteAddr
		}
	}
	if c.Handler == nil {
		c.Handler = func(ctx *Context) {
			_ = ctx.JSON(http.StatusTooManyRequests, map[string]any{
				"success": false,
				"error": map[string]any{
					"code":    "rate_limit_exceeded",
					"message": "rate limit exceeded",
				},
			})
		}
	}
}

// NewRateLimiter creates a new token bucket rate limiter.
func NewRateLimiter(cfg RateLimitConfig) *RateLimiter {
	cfg.applyDefaults()
	rl := &RateLimiter{
		buckets: make(map[string]*rateLimitEntry),
		rate:    cfg.Rate,
		burst:   cfg.Burst,
		cleanup: cfg.Cleanup,
		maxKeys: cfg.MaxKeys,
		keyFunc: cfg.KeyFunc,
		handler: cfg.Handler,
	}
	go rl.evictLoop()
	return rl
}

func (rl *RateLimiter) evictLoop() {
	ticker := time.NewTicker(rl.cleanup)
	defer ticker.Stop()
	for range ticker.C {
		rl.evict()
	}
}

func (rl *RateLimiter) evict() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	for key, entry := range rl.buckets {
		if now.Sub(entry.lastRefill) > rl.cleanup*2 {
			delete(rl.buckets, key)
		}
	}
}

func (rl *RateLimiter) allow(key string) (bool, float64) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	entry, ok := rl.buckets[key]
	if !ok {
		if len(rl.buckets) >= rl.maxKeys {
			return false, 0
		}
		entry = &rateLimitEntry{
			tokens:     float64(rl.burst),
			lastRefill: now,
		}
		rl.buckets[key] = entry
	}

	elapsed := now.Sub(entry.lastRefill).Seconds()
	entry.tokens += elapsed * rl.rate
	if entry.tokens > float64(rl.burst) {
		entry.tokens = float64(rl.burst)
	}
	entry.lastRefill = now

	if entry.tokens < 1 {
		return false, 0
	}

	entry.tokens--
	return true, entry.tokens
}

// RateLimit returns a middleware that applies the rate limiter.
func (rl *RateLimiter) RateLimit() Middleware {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx *Context) error {
			key := rl.keyFunc(ctx)
			allowed, remaining := rl.allow(key)
			ctx.ResponseWriter.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", rl.burst))
			ctx.ResponseWriter.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", int(remaining)))
			ctx.ResponseWriter.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(time.Second).Unix()))

			if !allowed {
				if rl.handler != nil {
					rl.handler(ctx)
					return nil
				}
				return fmt.Errorf("rate_limit: %w", &APIError{
					StatusCode: http.StatusTooManyRequests,
					Code:       "rate_limit_exceeded",
					Message:    "rate limit exceeded",
				})
			}
			return next(ctx)
		}
	}
}

// Allow checks if a key is allowed to proceed.
func (rl *RateLimiter) Allow(key string) bool {
	allowed, _ := rl.allow(key)
	return allowed
}
