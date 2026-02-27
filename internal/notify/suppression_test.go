package notify

import (
	"testing"
	"time"

	"github.com/rathix/command-center/internal/config"
	"github.com/rathix/command-center/internal/state"
)

func TestSuppression_FirstNotificationAllowed(t *testing.T) {
	now := time.Now()
	se := NewSuppressionEngine(WithClock(func() time.Time { return now }))

	rule := config.NotificationRule{
		SuppressionInterval: "15m",
		Channels:            []string{"webhook"},
	}

	d := se.Evaluate("default/api", 0, rule, state.StatusUnhealthy)
	if d.Action != Allow {
		t.Errorf("first notification should be allowed, got %v", d.Action)
	}
}

func TestSuppression_WithinIntervalSuppressed(t *testing.T) {
	now := time.Now()
	se := NewSuppressionEngine(WithClock(func() time.Time { return now }))

	rule := config.NotificationRule{
		SuppressionInterval: "15m",
		Channels:            []string{"webhook"},
	}

	// First: allowed
	d := se.Evaluate("default/api", 0, rule, state.StatusUnhealthy)
	if d.Action != Allow {
		t.Fatalf("first should be allowed")
	}

	// Second within 15m: suppressed
	now = now.Add(5 * time.Minute)
	d = se.Evaluate("default/api", 0, rule, state.StatusUnhealthy)
	if d.Action != Suppress {
		t.Errorf("second within interval should be suppressed, got %v", d.Action)
	}
}

func TestSuppression_AfterIntervalAllowed(t *testing.T) {
	now := time.Now()
	se := NewSuppressionEngine(WithClock(func() time.Time { return now }))

	rule := config.NotificationRule{
		SuppressionInterval: "15m",
		Channels:            []string{"webhook"},
	}

	// First
	se.Evaluate("default/api", 0, rule, state.StatusUnhealthy)

	// After 16m: allowed
	now = now.Add(16 * time.Minute)
	d := se.Evaluate("default/api", 0, rule, state.StatusUnhealthy)
	if d.Action != Allow {
		t.Errorf("after interval should be allowed, got %v", d.Action)
	}
}

func TestSuppression_MinimumGranularity(t *testing.T) {
	now := time.Now()
	se := NewSuppressionEngine(WithClock(func() time.Time { return now }))

	rule := config.NotificationRule{
		SuppressionInterval: "30s", // Below minimum, should be clamped to 1m
		Channels:            []string{"webhook"},
	}

	// First: allowed
	se.Evaluate("default/api", 0, rule, state.StatusUnhealthy)

	// After 45s: should still be suppressed (clamped to 1m)
	now = now.Add(45 * time.Second)
	d := se.Evaluate("default/api", 0, rule, state.StatusUnhealthy)
	if d.Action != Suppress {
		t.Errorf("should be suppressed at 45s (clamped to 1m minimum), got %v", d.Action)
	}

	// After 1m+: allowed
	now = now.Add(20 * time.Second) // total 65s
	d = se.Evaluate("default/api", 0, rule, state.StatusUnhealthy)
	if d.Action != Allow {
		t.Errorf("should be allowed after 1m minimum, got %v", d.Action)
	}
}

func TestSuppression_EscalationAfterDuration(t *testing.T) {
	now := time.Now()
	se := NewSuppressionEngine(WithClock(func() time.Time { return now }))

	rule := config.NotificationRule{
		SuppressionInterval: "15m",
		EscalateAfter:       "30m",
		Channels:            []string{"webhook"},
		EscalationChannels:  []string{"pushover"},
	}

	// First: allowed
	se.Evaluate("default/api", 0, rule, state.StatusUnhealthy)

	// After 31m: escalate
	now = now.Add(31 * time.Minute)
	d := se.Evaluate("default/api", 0, rule, state.StatusUnhealthy)
	if d.Action != Escalate {
		t.Errorf("expected escalation after 30m, got %v", d.Action)
	}
	// Channels should include both regular and escalation
	found := map[string]bool{}
	for _, ch := range d.Channels {
		found[ch] = true
	}
	if !found["webhook"] || !found["pushover"] {
		t.Errorf("expected webhook and pushover channels, got %v", d.Channels)
	}
}

func TestSuppression_EscalationOnlyOnce(t *testing.T) {
	now := time.Now()
	se := NewSuppressionEngine(WithClock(func() time.Time { return now }))

	rule := config.NotificationRule{
		SuppressionInterval: "15m",
		EscalateAfter:       "30m",
		Channels:            []string{"webhook"},
		EscalationChannels:  []string{"pushover"},
	}

	// First
	se.Evaluate("default/api", 0, rule, state.StatusUnhealthy)

	// After 31m: escalate
	now = now.Add(31 * time.Minute)
	d := se.Evaluate("default/api", 0, rule, state.StatusUnhealthy)
	if d.Action != Escalate {
		t.Fatalf("expected first escalation")
	}

	// After another 16m: regular allow, not escalate again
	now = now.Add(16 * time.Minute)
	d = se.Evaluate("default/api", 0, rule, state.StatusUnhealthy)
	if d.Action == Escalate {
		t.Errorf("escalation should fire only once, got %v", d.Action)
	}
}

func TestSuppression_RecoveryResetsTimers(t *testing.T) {
	now := time.Now()
	se := NewSuppressionEngine(WithClock(func() time.Time { return now }))

	rule := config.NotificationRule{
		SuppressionInterval: "15m",
		Channels:            []string{"webhook"},
	}

	// First unhealthy
	se.Evaluate("default/api", 0, rule, state.StatusUnhealthy)

	// Recovery resets
	se.Reset("default/api")

	// Next unhealthy should be treated as first
	now = now.Add(1 * time.Minute)
	d := se.Evaluate("default/api", 0, rule, state.StatusUnhealthy)
	if d.Action != Allow {
		t.Errorf("after reset, should be allowed, got %v", d.Action)
	}
}

func TestSuppression_RecoveryNotification(t *testing.T) {
	now := time.Now()
	se := NewSuppressionEngine(WithClock(func() time.Time { return now }))

	rule := config.NotificationRule{
		Channels: []string{"webhook"},
	}

	// Recovery is always allowed
	d := se.Evaluate("default/api", 0, rule, state.StatusHealthy)
	if d.Action != Allow {
		t.Errorf("recovery should always be allowed, got %v", d.Action)
	}
}

func TestSuppression_CheckReminders(t *testing.T) {
	now := time.Now()
	se := NewSuppressionEngine(WithClock(func() time.Time { return now }))

	rules := []config.NotificationRule{
		{
			Services:            []string{"*"},
			Transitions:         []string{"unhealthy"},
			Channels:            []string{"webhook"},
			SuppressionInterval: "15m",
		},
	}

	// Initial evaluation
	se.Evaluate("default/api", 0, rules[0], state.StatusUnhealthy)

	currentStates := map[string]state.HealthStatus{
		"default/api": state.StatusUnhealthy,
	}

	// Before interval: no reminders
	now = now.Add(10 * time.Minute)
	reminders := se.CheckReminders(rules, currentStates)
	if len(reminders) != 0 {
		t.Errorf("expected no reminders before interval, got %d", len(reminders))
	}

	// After interval: reminder
	now = now.Add(6 * time.Minute) // total 16m
	reminders = se.CheckReminders(rules, currentStates)
	if len(reminders) != 1 {
		t.Fatalf("expected 1 reminder, got %d", len(reminders))
	}
	if reminders[0].ServiceKey != "default/api" {
		t.Errorf("expected default/api, got %s", reminders[0].ServiceKey)
	}
}

func TestSuppression_NoSuppressionIfNotConfigured(t *testing.T) {
	now := time.Now()
	se := NewSuppressionEngine(WithClock(func() time.Time { return now }))

	rule := config.NotificationRule{
		Channels: []string{"webhook"},
		// No SuppressionInterval
	}

	// First
	d := se.Evaluate("default/api", 0, rule, state.StatusUnhealthy)
	if d.Action != Allow {
		t.Errorf("first should be allowed")
	}

	// Immediate second with no suppression: also allowed
	d = se.Evaluate("default/api", 0, rule, state.StatusUnhealthy)
	if d.Action != Allow {
		t.Errorf("without suppression, all events should be allowed, got %v", d.Action)
	}
}

func TestSuppression_NoEscalationIfNotConfigured(t *testing.T) {
	now := time.Now()
	se := NewSuppressionEngine(WithClock(func() time.Time { return now }))

	rule := config.NotificationRule{
		SuppressionInterval: "15m",
		Channels:            []string{"webhook"},
		// No EscalateAfter
	}

	// First
	se.Evaluate("default/api", 0, rule, state.StatusUnhealthy)

	// After 1h: still regular allow, no escalation
	now = now.Add(1 * time.Hour)
	d := se.Evaluate("default/api", 0, rule, state.StatusUnhealthy)
	if d.Action == Escalate {
		t.Errorf("without escalateAfter, should not escalate")
	}
}
