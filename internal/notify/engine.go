package notify

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/rathix/command-center/internal/state"
)

// StateSource provides event subscription for the notification engine.
// Defined at the consumer following the same pattern as SSE broker.
type StateSource interface {
	Subscribe() <-chan state.Event
	Unsubscribe(ch <-chan state.Event)
}

// Engine listens to state transitions and dispatches notifications.
type Engine struct {
	source      StateSource
	adapters    map[string]Adapter
	matcher     *RuleMatcher
	suppression *SuppressionEngine
	dispatcher  *RetryDispatcher
	logger      *slog.Logger
	prevState   map[string]state.HealthStatus
}

// NewEngine creates a notification engine with the given source and adapters.
func NewEngine(source StateSource, adapters map[string]Adapter, opts ...Option) *Engine {
	e := &Engine{
		source:    source,
		adapters:  adapters,
		logger:    slog.Default(),
		prevState: make(map[string]state.HealthStatus),
	}
	for _, opt := range opts {
		opt(e)
	}
	if e.dispatcher == nil {
		e.dispatcher = NewRetryDispatcher(WithRetryLogger(e.logger))
	}
	if e.suppression == nil {
		e.suppression = NewSuppressionEngine()
	}
	return e
}

// WithRuleMatcher sets the rule matcher for routing notifications.
func WithRuleMatcher(m *RuleMatcher) Option {
	return func(e *Engine) {
		e.matcher = m
	}
}

// WithSuppression sets the suppression engine.
func WithSuppression(s *SuppressionEngine) Option {
	return func(e *Engine) {
		e.suppression = s
	}
}

// WithRetryDispatcher sets the retry dispatcher.
func WithRetryDispatcher(d *RetryDispatcher) Option {
	return func(e *Engine) {
		e.dispatcher = d
	}
}

// Run blocks until context cancellation, processing state events and dispatching notifications.
func (e *Engine) Run(ctx context.Context) {
	ch := e.source.Subscribe()
	defer e.source.Unsubscribe(ch)

	var reminderTicker <-chan struct{}
	// Reminder ticker is handled by suppression engine integration in story 16.3.
	// For now we use a nil channel that never fires.
	_ = reminderTicker

	for {
		select {
		case <-ctx.Done():
			e.logger.Debug("notification engine stopped")
			return
		case evt, ok := <-ch:
			if !ok {
				e.logger.Debug("notification engine source channel closed")
				return
			}
			e.handleEvent(ctx, evt)
		}
	}
}

func (e *Engine) handleEvent(ctx context.Context, evt state.Event) {
	switch evt.Type {
	case state.EventDiscovered:
		key := serviceKey(evt.Service.Namespace, evt.Service.Name)
		e.prevState[key] = evt.Service.CompositeStatus
		e.logger.Debug("service discovered, stored initial state",
			"service", key,
			"status", evt.Service.CompositeStatus,
		)

	case state.EventUpdated:
		key := serviceKey(evt.Service.Namespace, evt.Service.Name)
		prev, exists := e.prevState[key]
		newStatus := evt.Service.CompositeStatus
		e.prevState[key] = newStatus

		if !exists {
			// Treat as discovery if we somehow missed the discovered event
			e.logger.Debug("service first seen via update",
				"service", key,
				"status", newStatus,
			)
			return
		}

		if prev == newStatus {
			return
		}

		e.logger.Debug("composite state transition detected",
			"service", key,
			"from", prev,
			"to", newStatus,
		)

		notification := buildNotification(evt.Service, prev)

		e.dispatchForTransition(ctx, key, newStatus, notification)

	case state.EventRemoved:
		key := serviceKey(evt.Namespace, evt.Name)
		delete(e.prevState, key)
		e.suppression.Reset(key)
		e.logger.Debug("service removed, cleaned up state", "service", key)
	}
}

func (e *Engine) dispatchForTransition(ctx context.Context, serviceKey string, newStatus state.HealthStatus, notification Notification) {
	if e.matcher == nil {
		// No rules configured: dispatch to all adapters for unhealthy/degraded
		if newStatus == state.StatusUnhealthy || newStatus == state.StatusDegraded {
			for _, adapter := range e.adapters {
				e.dispatcher.Dispatch(ctx, adapter, notification)
			}
		}
		return
	}

	// With rule matcher: evaluate each matching rule through suppression
	rules := e.matcher.Rules()
	for ruleIdx, rule := range rules {
		if !ruleMatchesEvent(rule, serviceKey, newStatus) {
			continue
		}

		decision := e.suppression.Evaluate(serviceKey, ruleIdx, rule, newStatus)
		switch decision.Action {
		case Allow, Escalate:
			if decision.Action == Escalate {
				notification.Escalated = true
			}
			e.dispatchToChannels(ctx, decision.Channels, notification)
		case Suppress:
			e.logger.Debug("notification suppressed",
				"service", serviceKey,
				"rule", ruleIdx,
			)
		}
	}

	// Recovery resets suppression state
	if newStatus == state.StatusHealthy {
		e.suppression.Reset(serviceKey)
	}
}

func (e *Engine) dispatchToChannels(ctx context.Context, channels []string, n Notification) {
	seen := make(map[string]struct{})
	for _, ch := range channels {
		if _, ok := seen[ch]; ok {
			continue
		}
		seen[ch] = struct{}{}
		adapter, ok := e.adapters[ch]
		if !ok {
			e.logger.Warn("adapter not found for channel", "channel", ch)
			continue
		}
		e.dispatcher.Dispatch(ctx, adapter, n)
	}
}

func serviceKey(namespace, name string) string {
	return namespace + "/" + name
}

// buildNotification constructs a Notification with diagnostic context from the service.
func buildNotification(svc state.Service, prevStatus state.HealthStatus) Notification {
	n := Notification{
		ServiceName: svc.Name,
		Namespace:   svc.Namespace,
		PrevState:   prevStatus,
		NewState:    svc.CompositeStatus,
		Timestamp:   svc.LastChecked.UTC(),
		PodDiag:     svc.PodDiagnostic,
	}

	// Derive triggering signals
	if svc.Status == state.StatusUnhealthy {
		n.Signals = append(n.Signals, "http:unhealthy")
	} else if svc.Status == state.StatusDegraded {
		n.Signals = append(n.Signals, "http:degraded")
	}
	if svc.AuthGuarded {
		n.Signals = append(n.Signals, "http:auth-guarded")
	}
	if svc.CompositeStatus != svc.Status {
		if svc.ReadyEndpoints != nil && svc.TotalEndpoints != nil {
			n.Signals = append(n.Signals, fmt.Sprintf("endpoints:%d/%d-ready",
				*svc.ReadyEndpoints, *svc.TotalEndpoints))
		}
	}
	if svc.ErrorSnippet != nil {
		n.Signals = append(n.Signals, "error:"+*svc.ErrorSnippet)
	}
	return n
}
