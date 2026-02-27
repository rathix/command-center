package notify

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/rathix/command-center/internal/config"
	"github.com/rathix/command-center/internal/state"
)

// --- fakes ---

type fakeAdapter struct {
	mu     sync.Mutex
	name   string
	sent   []Notification
	errFn  func() error
}

func newFakeAdapter(name string) *fakeAdapter {
	return &fakeAdapter{name: name}
}

func (f *fakeAdapter) Name() string { return f.name }

func (f *fakeAdapter) Send(_ context.Context, n Notification) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errFn != nil {
		if err := f.errFn(); err != nil {
			return err
		}
	}
	f.sent = append(f.sent, n)
	return nil
}

func (f *fakeAdapter) sentNotifications() []Notification {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := make([]Notification, len(f.sent))
	copy(cp, f.sent)
	return cp
}

type fakeStateSource struct {
	ch   chan state.Event
	done chan struct{}
}

func newFakeStateSource() *fakeStateSource {
	return &fakeStateSource{
		ch:   make(chan state.Event, 128),
		done: make(chan struct{}),
	}
}

func (f *fakeStateSource) Subscribe() <-chan state.Event { return f.ch }
func (f *fakeStateSource) Unsubscribe(_ <-chan state.Event) {
	close(f.done)
}

// --- engine tests ---

func TestEngine_TransitionTriggersNotification(t *testing.T) {
	src := newFakeStateSource()
	adapter := newFakeAdapter("test-webhook")
	adapters := map[string]Adapter{"test-webhook": adapter}

	now := time.Now()
	dispatcher := NewRetryDispatcher(WithBaseDelay(0), WithMaxAttempts(1))
	engine := NewEngine(src, adapters,
		WithRetryDispatcher(dispatcher),
	)

	ctx, cancel := context.WithCancel(context.Background())
	go engine.Run(ctx)

	// Discover the service first
	src.ch <- state.Event{
		Type: state.EventDiscovered,
		Service: state.Service{
			Name:            "api",
			Namespace:       "default",
			CompositeStatus: state.StatusHealthy,
			Status:          state.StatusHealthy,
			LastChecked:     &now,
		},
	}
	time.Sleep(50 * time.Millisecond)

	// Transition to unhealthy
	src.ch <- state.Event{
		Type: state.EventUpdated,
		Service: state.Service{
			Name:            "api",
			Namespace:       "default",
			CompositeStatus: state.StatusUnhealthy,
			Status:          state.StatusUnhealthy,
			LastChecked:     &now,
		},
	}
	time.Sleep(100 * time.Millisecond)
	cancel()
	<-src.done

	sent := adapter.sentNotifications()
	if len(sent) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(sent))
	}
	if sent[0].ServiceName != "api" {
		t.Errorf("expected service name 'api', got %q", sent[0].ServiceName)
	}
	if sent[0].PrevState != state.StatusHealthy {
		t.Errorf("expected prev state healthy, got %v", sent[0].PrevState)
	}
	if sent[0].NewState != state.StatusUnhealthy {
		t.Errorf("expected new state unhealthy, got %v", sent[0].NewState)
	}
}

func TestEngine_NoTransitionNoNotification(t *testing.T) {
	src := newFakeStateSource()
	adapter := newFakeAdapter("hook")
	adapters := map[string]Adapter{"hook": adapter}

	now := time.Now()
	dispatcher := NewRetryDispatcher(WithBaseDelay(0), WithMaxAttempts(1))
	engine := NewEngine(src, adapters, WithRetryDispatcher(dispatcher))

	ctx, cancel := context.WithCancel(context.Background())
	go engine.Run(ctx)

	// Discover as unhealthy
	src.ch <- state.Event{
		Type: state.EventDiscovered,
		Service: state.Service{
			Name: "api", Namespace: "default",
			CompositeStatus: state.StatusUnhealthy,
			Status:          state.StatusUnhealthy,
			LastChecked:     &now,
		},
	}
	time.Sleep(50 * time.Millisecond)

	// Update with same status
	src.ch <- state.Event{
		Type: state.EventUpdated,
		Service: state.Service{
			Name: "api", Namespace: "default",
			CompositeStatus: state.StatusUnhealthy,
			Status:          state.StatusUnhealthy,
			LastChecked:     &now,
		},
	}
	time.Sleep(100 * time.Millisecond)
	cancel()
	<-src.done

	if len(adapter.sentNotifications()) != 0 {
		t.Fatalf("expected no notifications, got %d", len(adapter.sentNotifications()))
	}
}

func TestEngine_DiscoveredDoesNotNotify(t *testing.T) {
	src := newFakeStateSource()
	adapter := newFakeAdapter("hook")
	adapters := map[string]Adapter{"hook": adapter}

	now := time.Now()
	dispatcher := NewRetryDispatcher(WithBaseDelay(0), WithMaxAttempts(1))
	engine := NewEngine(src, adapters, WithRetryDispatcher(dispatcher))

	ctx, cancel := context.WithCancel(context.Background())
	go engine.Run(ctx)

	// Discover directly as unhealthy - should NOT notify
	src.ch <- state.Event{
		Type: state.EventDiscovered,
		Service: state.Service{
			Name: "api", Namespace: "default",
			CompositeStatus: state.StatusUnhealthy,
			Status:          state.StatusUnhealthy,
			LastChecked:     &now,
		},
	}
	time.Sleep(100 * time.Millisecond)
	cancel()
	<-src.done

	if len(adapter.sentNotifications()) != 0 {
		t.Fatalf("expected no notifications on discovery, got %d", len(adapter.sentNotifications()))
	}
}

func TestEngine_ContextCancellation(t *testing.T) {
	src := newFakeStateSource()
	engine := NewEngine(src, map[string]Adapter{})

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		engine.Run(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
		// good
	case <-time.After(2 * time.Second):
		t.Fatal("engine did not stop within 2s")
	}
}

func TestEngine_RemovedCleansUpState(t *testing.T) {
	src := newFakeStateSource()
	adapter := newFakeAdapter("hook")
	adapters := map[string]Adapter{"hook": adapter}

	now := time.Now()
	dispatcher := NewRetryDispatcher(WithBaseDelay(0), WithMaxAttempts(1))
	engine := NewEngine(src, adapters, WithRetryDispatcher(dispatcher))

	ctx, cancel := context.WithCancel(context.Background())
	go engine.Run(ctx)

	// Discover service
	src.ch <- state.Event{
		Type: state.EventDiscovered,
		Service: state.Service{
			Name: "api", Namespace: "default",
			CompositeStatus: state.StatusHealthy,
			Status:          state.StatusHealthy,
			LastChecked:     &now,
		},
	}
	time.Sleep(50 * time.Millisecond)

	// Remove service
	src.ch <- state.Event{
		Type:      state.EventRemoved,
		Namespace: "default",
		Name:      "api",
	}
	time.Sleep(50 * time.Millisecond)

	// Re-discover (should not trigger notification since it's a new discovery)
	src.ch <- state.Event{
		Type: state.EventDiscovered,
		Service: state.Service{
			Name: "api", Namespace: "default",
			CompositeStatus: state.StatusUnhealthy,
			Status:          state.StatusUnhealthy,
			LastChecked:     &now,
		},
	}
	time.Sleep(100 * time.Millisecond)
	cancel()
	<-src.done

	if len(adapter.sentNotifications()) != 0 {
		t.Fatalf("expected no notifications after remove+rediscover, got %d", len(adapter.sentNotifications()))
	}
}

// --- Rule matcher integration tests ---

func TestEngine_WithRuleMatcher_Routes(t *testing.T) {
	src := newFakeStateSource()
	webhook := newFakeAdapter("webhook")
	slack := newFakeAdapter("slack")
	adapters := map[string]Adapter{"webhook": webhook, "slack": slack}

	now := time.Now()
	rules := []config.NotificationRule{
		{
			Services:    []string{"default/*"},
			Transitions: []string{"unhealthy"},
			Channels:    []string{"webhook"},
		},
	}
	matcher := NewRuleMatcher(rules)
	dispatcher := NewRetryDispatcher(WithBaseDelay(0), WithMaxAttempts(1))
	engine := NewEngine(src, adapters,
		WithRuleMatcher(matcher),
		WithRetryDispatcher(dispatcher),
	)

	ctx, cancel := context.WithCancel(context.Background())
	go engine.Run(ctx)

	src.ch <- state.Event{
		Type: state.EventDiscovered,
		Service: state.Service{
			Name: "api", Namespace: "default",
			CompositeStatus: state.StatusHealthy,
			Status:          state.StatusHealthy,
			LastChecked:     &now,
		},
	}
	time.Sleep(50 * time.Millisecond)

	src.ch <- state.Event{
		Type: state.EventUpdated,
		Service: state.Service{
			Name: "api", Namespace: "default",
			CompositeStatus: state.StatusUnhealthy,
			Status:          state.StatusUnhealthy,
			LastChecked:     &now,
		},
	}
	time.Sleep(100 * time.Millisecond)
	cancel()
	<-src.done

	if len(webhook.sentNotifications()) != 1 {
		t.Errorf("expected 1 webhook notification, got %d", len(webhook.sentNotifications()))
	}
	if len(slack.sentNotifications()) != 0 {
		t.Errorf("expected 0 slack notifications, got %d", len(slack.sentNotifications()))
	}
}

func TestEngine_WithRuleMatcher_NoMatch(t *testing.T) {
	src := newFakeStateSource()
	adapter := newFakeAdapter("webhook")
	adapters := map[string]Adapter{"webhook": adapter}

	now := time.Now()
	rules := []config.NotificationRule{
		{
			Services:    []string{"prod/*"},
			Transitions: []string{"unhealthy"},
			Channels:    []string{"webhook"},
		},
	}
	matcher := NewRuleMatcher(rules)
	dispatcher := NewRetryDispatcher(WithBaseDelay(0), WithMaxAttempts(1))
	engine := NewEngine(src, adapters,
		WithRuleMatcher(matcher),
		WithRetryDispatcher(dispatcher),
	)

	ctx, cancel := context.WithCancel(context.Background())
	go engine.Run(ctx)

	src.ch <- state.Event{
		Type: state.EventDiscovered,
		Service: state.Service{
			Name: "api", Namespace: "default",
			CompositeStatus: state.StatusHealthy,
			Status:          state.StatusHealthy,
			LastChecked:     &now,
		},
	}
	time.Sleep(50 * time.Millisecond)

	src.ch <- state.Event{
		Type: state.EventUpdated,
		Service: state.Service{
			Name: "api", Namespace: "default",
			CompositeStatus: state.StatusUnhealthy,
			Status:          state.StatusUnhealthy,
			LastChecked:     &now,
		},
	}
	time.Sleep(100 * time.Millisecond)
	cancel()
	<-src.done

	if len(adapter.sentNotifications()) != 0 {
		t.Errorf("expected 0 notifications when no rules match, got %d", len(adapter.sentNotifications()))
	}
}

func TestEngine_NonBlockingDispatch(t *testing.T) {
	src := newFakeStateSource()
	slowAdapter := newFakeAdapter("slow")
	slowAdapter.errFn = func() error {
		time.Sleep(100 * time.Millisecond)
		return nil
	}
	adapters := map[string]Adapter{"slow": slowAdapter}

	now := time.Now()
	dispatcher := NewRetryDispatcher(WithBaseDelay(0), WithMaxAttempts(1))
	engine := NewEngine(src, adapters, WithRetryDispatcher(dispatcher))

	ctx, cancel := context.WithCancel(context.Background())
	go engine.Run(ctx)

	// Discover
	src.ch <- state.Event{
		Type: state.EventDiscovered,
		Service: state.Service{
			Name: "api", Namespace: "default",
			CompositeStatus: state.StatusHealthy,
			Status:          state.StatusHealthy,
			LastChecked:     &now,
		},
	}
	time.Sleep(50 * time.Millisecond)

	// Send many rapid transitions - engine should process them without blocking
	start := time.Now()
	for i := 0; i < 10; i++ {
		status := state.StatusUnhealthy
		if i%2 == 0 {
			status = state.StatusDegraded
		}
		src.ch <- state.Event{
			Type: state.EventUpdated,
			Service: state.Service{
				Name: "api", Namespace: "default",
				CompositeStatus: status,
				Status:          status,
				LastChecked:     &now,
			},
		}
	}
	// Engine should process all events quickly regardless of slow adapter
	time.Sleep(100 * time.Millisecond)
	elapsed := time.Since(start)
	cancel()
	<-src.done

	// Should not take 10 * 100ms = 1s (adapter latency * events)
	if elapsed > 500*time.Millisecond {
		t.Errorf("engine loop took too long (%v), likely blocking on adapter", elapsed)
	}
}

func TestBuildNotification_Signals(t *testing.T) {
	now := time.Now()
	ready := 2
	total := 3
	errSnip := "connection refused"

	svc := state.Service{
		Name:            "api",
		Namespace:       "default",
		Status:          state.StatusUnhealthy,
		CompositeStatus: state.StatusUnhealthy,
		AuthGuarded:     true,
		ErrorSnippet:    &errSnip,
		ReadyEndpoints:  &ready,
		TotalEndpoints:  &total,
		LastChecked:     &now,
	}

	n := buildNotification(svc, state.StatusHealthy)

	expected := []string{"http:unhealthy", "http:auth-guarded", "error:connection refused"}
	if len(n.Signals) != len(expected) {
		t.Fatalf("expected %d signals, got %d: %v", len(expected), len(n.Signals), n.Signals)
	}
	for i, sig := range expected {
		if n.Signals[i] != sig {
			t.Errorf("signal[%d]: expected %q, got %q", i, sig, n.Signals[i])
		}
	}
}

func TestBuildNotification_EndpointSignals(t *testing.T) {
	now := time.Now()
	ready := 2
	total := 3

	svc := state.Service{
		Name:            "api",
		Namespace:       "default",
		Status:          state.StatusHealthy,
		CompositeStatus: state.StatusDegraded,
		ReadyEndpoints:  &ready,
		TotalEndpoints:  &total,
		LastChecked:     &now,
	}

	n := buildNotification(svc, state.StatusHealthy)

	found := false
	for _, sig := range n.Signals {
		if sig == "endpoints:2/3-ready" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected endpoint signal, got %v", n.Signals)
	}
}

func TestBuildNotification_PodDiagnostics(t *testing.T) {
	now := time.Now()
	reason := "CrashLoopBackOff"

	svc := state.Service{
		Name:            "api",
		Namespace:       "default",
		Status:          state.StatusUnhealthy,
		CompositeStatus: state.StatusUnhealthy,
		LastChecked:     &now,
		PodDiagnostic: &state.PodDiagnostic{
			Reason:       &reason,
			RestartCount: 5,
		},
	}

	n := buildNotification(svc, state.StatusHealthy)

	if n.PodDiag == nil {
		t.Fatal("expected pod diagnostics")
	}
	if *n.PodDiag.Reason != "CrashLoopBackOff" {
		t.Errorf("expected reason CrashLoopBackOff, got %v", *n.PodDiag.Reason)
	}
	if n.PodDiag.RestartCount != 5 {
		t.Errorf("expected restart count 5, got %d", n.PodDiag.RestartCount)
	}
}

func TestBuildNotification_NilPodDiag(t *testing.T) {
	now := time.Now()

	svc := state.Service{
		Name:            "api",
		Namespace:       "default",
		Status:          state.StatusUnhealthy,
		CompositeStatus: state.StatusUnhealthy,
		LastChecked:     &now,
	}

	n := buildNotification(svc, state.StatusHealthy)

	if n.PodDiag != nil {
		t.Errorf("expected nil pod diagnostics, got %v", n.PodDiag)
	}
}

func TestBuildNotification_AllFields(t *testing.T) {
	now := time.Now()
	svc := state.Service{
		Name:            "api-gateway",
		Namespace:       "prod",
		Status:          state.StatusDegraded,
		CompositeStatus: state.StatusDegraded,
		LastChecked:     &now,
	}

	n := buildNotification(svc, state.StatusHealthy)

	if n.ServiceName != "api-gateway" {
		t.Errorf("ServiceName: got %q", n.ServiceName)
	}
	if n.Namespace != "prod" {
		t.Errorf("Namespace: got %q", n.Namespace)
	}
	if n.PrevState != state.StatusHealthy {
		t.Errorf("PrevState: got %v", n.PrevState)
	}
	if n.NewState != state.StatusDegraded {
		t.Errorf("NewState: got %v", n.NewState)
	}
	if n.Timestamp.IsZero() {
		t.Error("Timestamp is zero")
	}
	if len(n.Signals) != 1 || n.Signals[0] != "http:degraded" {
		t.Errorf("Signals: got %v", n.Signals)
	}
}

func TestEngine_TransitionToDegraded(t *testing.T) {
	src := newFakeStateSource()
	adapter := newFakeAdapter("hook")
	adapters := map[string]Adapter{"hook": adapter}

	now := time.Now()
	dispatcher := NewRetryDispatcher(WithBaseDelay(0), WithMaxAttempts(1))
	engine := NewEngine(src, adapters, WithRetryDispatcher(dispatcher))

	ctx, cancel := context.WithCancel(context.Background())
	go engine.Run(ctx)

	src.ch <- state.Event{
		Type: state.EventDiscovered,
		Service: state.Service{
			Name: "api", Namespace: "default",
			CompositeStatus: state.StatusHealthy,
			Status:          state.StatusHealthy,
			LastChecked:     &now,
		},
	}
	time.Sleep(50 * time.Millisecond)

	src.ch <- state.Event{
		Type: state.EventUpdated,
		Service: state.Service{
			Name: "api", Namespace: "default",
			CompositeStatus: state.StatusDegraded,
			Status:          state.StatusDegraded,
			LastChecked:     &now,
		},
	}
	time.Sleep(100 * time.Millisecond)
	cancel()
	<-src.done

	sent := adapter.sentNotifications()
	if len(sent) != 1 {
		t.Fatalf("expected 1 notification for degraded, got %d", len(sent))
	}
}

func TestEngine_RecoveryWithRules(t *testing.T) {
	src := newFakeStateSource()
	adapter := newFakeAdapter("webhook")
	adapters := map[string]Adapter{"webhook": adapter}

	now := time.Now()
	rules := []config.NotificationRule{
		{
			Services:    []string{"*"},
			Transitions: []string{"unhealthy", "healthy"},
			Channels:    []string{"webhook"},
		},
	}
	matcher := NewRuleMatcher(rules)
	dispatcher := NewRetryDispatcher(WithBaseDelay(0), WithMaxAttempts(1))
	engine := NewEngine(src, adapters,
		WithRuleMatcher(matcher),
		WithRetryDispatcher(dispatcher),
	)

	ctx, cancel := context.WithCancel(context.Background())
	go engine.Run(ctx)

	// Discover healthy
	src.ch <- state.Event{
		Type: state.EventDiscovered,
		Service: state.Service{
			Name: "api", Namespace: "default",
			CompositeStatus: state.StatusHealthy,
			Status:          state.StatusHealthy,
			LastChecked:     &now,
		},
	}
	time.Sleep(50 * time.Millisecond)

	// Go unhealthy
	src.ch <- state.Event{
		Type: state.EventUpdated,
		Service: state.Service{
			Name: "api", Namespace: "default",
			CompositeStatus: state.StatusUnhealthy,
			Status:          state.StatusUnhealthy,
			LastChecked:     &now,
		},
	}
	time.Sleep(50 * time.Millisecond)

	// Recover
	src.ch <- state.Event{
		Type: state.EventUpdated,
		Service: state.Service{
			Name: "api", Namespace: "default",
			CompositeStatus: state.StatusHealthy,
			Status:          state.StatusHealthy,
			LastChecked:     &now,
		},
	}
	time.Sleep(100 * time.Millisecond)
	cancel()
	<-src.done

	sent := adapter.sentNotifications()
	if len(sent) != 2 {
		t.Fatalf("expected 2 notifications (unhealthy + recovery), got %d: %+v", len(sent), sent)
	}
	if sent[0].NewState != state.StatusUnhealthy {
		t.Errorf("first notification should be unhealthy, got %v", sent[0].NewState)
	}
	if sent[1].NewState != state.StatusHealthy {
		t.Errorf("second notification should be recovery (healthy), got %v", sent[1].NewState)
	}
}

func TestEngine_DegradedWithSignals(t *testing.T) {
	src := newFakeStateSource()
	adapter := newFakeAdapter("hook")
	adapters := map[string]Adapter{"hook": adapter}

	now := time.Now()
	ready := 1
	total := 3
	dispatcher := NewRetryDispatcher(WithBaseDelay(0), WithMaxAttempts(1))
	engine := NewEngine(src, adapters, WithRetryDispatcher(dispatcher))

	ctx, cancel := context.WithCancel(context.Background())
	go engine.Run(ctx)

	src.ch <- state.Event{
		Type: state.EventDiscovered,
		Service: state.Service{
			Name: "api", Namespace: "default",
			CompositeStatus: state.StatusHealthy,
			Status:          state.StatusHealthy,
			LastChecked:     &now,
		},
	}
	time.Sleep(50 * time.Millisecond)

	src.ch <- state.Event{
		Type: state.EventUpdated,
		Service: state.Service{
			Name: "api", Namespace: "default",
			Status:          state.StatusHealthy,
			CompositeStatus: state.StatusDegraded,
			ReadyEndpoints:  &ready,
			TotalEndpoints:  &total,
			LastChecked:     &now,
		},
	}
	time.Sleep(100 * time.Millisecond)
	cancel()
	<-src.done

	sent := adapter.sentNotifications()
	if len(sent) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(sent))
	}
	found := false
	for _, sig := range sent[0].Signals {
		if sig == fmt.Sprintf("endpoints:%d/%d-ready", ready, total) {
			found = true
		}
	}
	if !found {
		t.Errorf("expected endpoint signal, got %v", sent[0].Signals)
	}
}
