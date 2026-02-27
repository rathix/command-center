package gitops

import (
	"context"
	"testing"
	"time"
)

func TestRateLimiter_AllowBurst(t *testing.T) {
	rl := NewRateLimiter(10, time.Minute)

	// 10 rapid calls should succeed
	for i := range 10 {
		if !rl.Allow() {
			t.Fatalf("call %d should be allowed", i+1)
		}
	}

	// 11th should be denied
	if rl.Allow() {
		t.Error("11th call should be denied")
	}
}

func TestRateLimiter_Refill(t *testing.T) {
	rl := NewRateLimiter(10, time.Minute)

	// Exhaust all tokens
	for range 10 {
		rl.Allow()
	}

	if rl.Allow() {
		t.Error("should be denied after exhaustion")
	}

	// Wait for one token to refill (6 seconds for 10 tokens/min)
	time.Sleep(7 * time.Second)

	if !rl.Allow() {
		t.Error("should be allowed after refill wait")
	}
}

func TestRateLimiter_Wait(t *testing.T) {
	rl := NewRateLimiter(10, time.Minute)

	// Exhaust all tokens
	for range 10 {
		rl.Allow()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := rl.Wait(ctx)
	if err == nil {
		t.Error("expected error when context times out before refill")
	}
}

func TestRateLimiter_WaitSuccess(t *testing.T) {
	// Use a fast refill for testing
	rl := NewRateLimiter(1, 100*time.Millisecond)

	// Exhaust token
	if !rl.Allow() {
		t.Fatal("first call should be allowed")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := rl.Wait(ctx)
	if err != nil {
		t.Errorf("expected successful wait, got: %v", err)
	}
}
