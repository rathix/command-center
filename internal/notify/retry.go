package notify

import (
	"context"
	"log/slog"
	"time"
)

// RetryOption configures the RetryDispatcher.
type RetryOption func(*RetryDispatcher)

// RetryDispatcher wraps adapter dispatch with exponential backoff retry.
type RetryDispatcher struct {
	maxAttempts   int
	baseDelay     time.Duration
	sem           chan struct{}
	logger        *slog.Logger
}

// NewRetryDispatcher creates a retry dispatcher with default settings.
func NewRetryDispatcher(opts ...RetryOption) *RetryDispatcher {
	d := &RetryDispatcher{
		maxAttempts: 3,
		baseDelay:   1 * time.Second,
		sem:         make(chan struct{}, 32),
		logger:      slog.Default(),
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// WithMaxAttempts sets the maximum number of delivery attempts.
func WithMaxAttempts(n int) RetryOption {
	return func(d *RetryDispatcher) {
		d.maxAttempts = n
	}
}

// WithBaseDelay sets the base delay for exponential backoff.
func WithBaseDelay(delay time.Duration) RetryOption {
	return func(d *RetryDispatcher) {
		d.baseDelay = delay
	}
}

// WithMaxConcurrent sets the maximum concurrent retry goroutines.
func WithMaxConcurrent(n int) RetryOption {
	return func(d *RetryDispatcher) {
		d.sem = make(chan struct{}, n)
	}
}

// WithRetryLogger sets the logger for the retry dispatcher.
func WithRetryLogger(l *slog.Logger) RetryOption {
	return func(d *RetryDispatcher) {
		d.logger = l
	}
}

// Dispatch sends a notification to an adapter with retry.
// It runs asynchronously and never blocks the caller.
func (d *RetryDispatcher) Dispatch(ctx context.Context, adapter Adapter, n Notification) {
	// Non-blocking semaphore acquisition
	select {
	case d.sem <- struct{}{}:
		go func() {
			defer func() { <-d.sem }()
			d.dispatch(ctx, adapter, n)
		}()
	default:
		d.logger.Warn("notification dropped: retry semaphore full",
			"adapter", adapter.Name(),
			"service", n.ServiceName,
		)
	}
}

func (d *RetryDispatcher) dispatch(ctx context.Context, adapter Adapter, n Notification) {
	for attempt := 0; attempt < d.maxAttempts; attempt++ {
		err := adapter.Send(ctx, n)
		if err == nil {
			return
		}
		d.logger.Warn("notification delivery failed",
			"adapter", adapter.Name(),
			"service", n.ServiceName,
			"attempt", attempt+1,
			"error", err,
		)
		if attempt < d.maxAttempts-1 {
			delay := d.baseDelay * time.Duration(1<<uint(attempt))
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
		}
	}
	d.logger.Warn("notification delivery exhausted retries",
		"adapter", adapter.Name(),
		"service", n.ServiceName,
		"attempts", d.maxAttempts,
	)
}
