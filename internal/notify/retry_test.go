package notify

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetryDispatcher_SuccessOnFirstAttempt(t *testing.T) {
	adapter := newFakeAdapter("test")
	d := NewRetryDispatcher(WithBaseDelay(0), WithMaxAttempts(3))

	ctx := context.Background()
	d.Dispatch(ctx, adapter, Notification{ServiceName: "api"})

	time.Sleep(50 * time.Millisecond)

	sent := adapter.sentNotifications()
	if len(sent) != 1 {
		t.Errorf("expected 1 send, got %d", len(sent))
	}
}

func TestRetryDispatcher_RetryOnFailure(t *testing.T) {
	var attempts atomic.Int32
	adapter := newFakeAdapter("test")
	adapter.errFn = func() error {
		n := attempts.Add(1)
		if n <= 2 {
			return fmt.Errorf("transient error")
		}
		return nil
	}

	d := NewRetryDispatcher(WithBaseDelay(1*time.Millisecond), WithMaxAttempts(3))

	ctx := context.Background()
	d.Dispatch(ctx, adapter, Notification{ServiceName: "api"})

	time.Sleep(100 * time.Millisecond)

	total := int(attempts.Load())
	if total != 3 {
		t.Errorf("expected 3 attempts, got %d", total)
	}
	// The last attempt succeeds
	sent := adapter.sentNotifications()
	if len(sent) != 1 {
		t.Errorf("expected 1 successful send, got %d", len(sent))
	}
}

func TestRetryDispatcher_ExhaustedRetries(t *testing.T) {
	var attempts atomic.Int32
	adapter := newFakeAdapter("test")
	adapter.errFn = func() error {
		attempts.Add(1)
		return fmt.Errorf("permanent error")
	}

	d := NewRetryDispatcher(WithBaseDelay(1*time.Millisecond), WithMaxAttempts(3))

	ctx := context.Background()
	d.Dispatch(ctx, adapter, Notification{ServiceName: "api"})

	time.Sleep(100 * time.Millisecond)

	total := int(attempts.Load())
	if total != 3 {
		t.Errorf("expected exactly 3 attempts, got %d", total)
	}
	sent := adapter.sentNotifications()
	if len(sent) != 0 {
		t.Errorf("expected 0 successful sends, got %d", len(sent))
	}
}

func TestRetryDispatcher_ContextCancellation(t *testing.T) {
	var attempts atomic.Int32
	adapter := newFakeAdapter("test")
	adapter.errFn = func() error {
		attempts.Add(1)
		return fmt.Errorf("fail")
	}

	d := NewRetryDispatcher(WithBaseDelay(500*time.Millisecond), WithMaxAttempts(5))

	ctx, cancel := context.WithCancel(context.Background())
	d.Dispatch(ctx, adapter, Notification{ServiceName: "api"})

	// Let first attempt happen, then cancel
	time.Sleep(50 * time.Millisecond)
	cancel()

	time.Sleep(100 * time.Millisecond)

	total := int(attempts.Load())
	if total >= 5 {
		t.Errorf("should have stopped retrying after cancel, got %d attempts", total)
	}
}

func TestRetryDispatcher_SemaphoreBackpressure(t *testing.T) {
	blockCh := make(chan struct{})
	adapter := newFakeAdapter("slow")
	adapter.errFn = func() error {
		<-blockCh
		return nil
	}

	// Semaphore of size 2
	d := NewRetryDispatcher(WithMaxConcurrent(2), WithMaxAttempts(1))

	ctx := context.Background()

	// Fill semaphore
	d.Dispatch(ctx, adapter, Notification{ServiceName: "api1"})
	d.Dispatch(ctx, adapter, Notification{ServiceName: "api2"})

	time.Sleep(50 * time.Millisecond)

	// Third should be dropped
	d.Dispatch(ctx, adapter, Notification{ServiceName: "api3"})

	time.Sleep(50 * time.Millisecond)

	// Unblock
	close(blockCh)
	time.Sleep(50 * time.Millisecond)

	sent := adapter.sentNotifications()
	if len(sent) != 2 {
		t.Errorf("expected 2 successful sends (third dropped), got %d", len(sent))
	}
}
