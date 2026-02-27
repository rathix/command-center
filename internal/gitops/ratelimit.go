package gitops

import (
	"context"
	"sync"
	"time"
)

// RateLimiter implements a token bucket rate limiter.
type RateLimiter struct {
	mu           sync.Mutex
	tokens       float64
	maxTokens    float64
	refillRate   float64 // tokens per second
	lastRefill   time.Time
	refillPeriod time.Duration
}

// NewRateLimiter creates a rate limiter with the given capacity and refill period.
// For example, NewRateLimiter(10, time.Minute) allows 10 requests per minute.
func NewRateLimiter(capacity int, period time.Duration) *RateLimiter {
	return &RateLimiter{
		tokens:       float64(capacity),
		maxTokens:    float64(capacity),
		refillRate:   float64(capacity) / period.Seconds(),
		lastRefill:   time.Now(),
		refillPeriod: period,
	}
}

// Allow returns true if a request is allowed (consuming one token), false if rate limited.
func (r *RateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.refill()

	if r.tokens >= 1 {
		r.tokens--
		return true
	}
	return false
}

// Wait blocks until a token is available or the context is cancelled.
func (r *RateLimiter) Wait(ctx context.Context) error {
	for {
		if r.Allow() {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
			// retry
		}
	}
}

// refill adds tokens based on elapsed time. Must be called with lock held.
func (r *RateLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(r.lastRefill).Seconds()
	r.tokens += elapsed * r.refillRate
	if r.tokens > r.maxTokens {
		r.tokens = r.maxTokens
	}
	r.lastRefill = now
}

// TokensRemaining returns the current number of available tokens (for diagnostics).
func (r *RateLimiter) TokensRemaining() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.refill()
	return int(r.tokens)
}
