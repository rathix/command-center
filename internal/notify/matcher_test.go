package notify

import (
	"testing"

	"github.com/rathix/command-center/internal/config"
	"github.com/rathix/command-center/internal/state"
)

func TestRuleMatcher_WildcardMatchesAll(t *testing.T) {
	rules := []config.NotificationRule{
		{
			Services:    []string{"*"},
			Transitions: []string{"unhealthy", "degraded"},
			Channels:    []string{"webhook"},
		},
	}
	m := NewRuleMatcher(rules)

	result := m.Match("default/myservice", state.StatusUnhealthy)
	if len(result) != 1 || result[0] != "webhook" {
		t.Errorf("expected [webhook], got %v", result)
	}
}

func TestRuleMatcher_NamespaceGlob(t *testing.T) {
	rules := []config.NotificationRule{
		{
			Services:    []string{"default/*"},
			Transitions: []string{"unhealthy"},
			Channels:    []string{"webhook"},
		},
	}
	m := NewRuleMatcher(rules)

	// Should match
	result := m.Match("default/api", state.StatusUnhealthy)
	if len(result) != 1 {
		t.Errorf("expected match for default/api, got %v", result)
	}

	// Should not match
	result = m.Match("kube-system/dns", state.StatusUnhealthy)
	if len(result) != 0 {
		t.Errorf("expected no match for kube-system/dns, got %v", result)
	}
}

func TestRuleMatcher_TransitionFilter(t *testing.T) {
	rules := []config.NotificationRule{
		{
			Services:    []string{"*"},
			Transitions: []string{"unhealthy"},
			Channels:    []string{"webhook"},
		},
	}
	m := NewRuleMatcher(rules)

	// Should match unhealthy
	result := m.Match("default/api", state.StatusUnhealthy)
	if len(result) != 1 {
		t.Errorf("expected match for unhealthy, got %v", result)
	}

	// Should not match degraded
	result = m.Match("default/api", state.StatusDegraded)
	if len(result) != 0 {
		t.Errorf("expected no match for degraded, got %v", result)
	}
}

func TestRuleMatcher_EmptyTransitionsMatchesAll(t *testing.T) {
	rules := []config.NotificationRule{
		{
			Services: []string{"*"},
			Channels: []string{"webhook"},
		},
	}
	m := NewRuleMatcher(rules)

	for _, status := range []state.HealthStatus{state.StatusHealthy, state.StatusDegraded, state.StatusUnhealthy} {
		result := m.Match("default/api", status)
		if len(result) != 1 {
			t.Errorf("expected match for %v with empty transitions, got %v", status, result)
		}
	}
}

func TestRuleMatcher_MultipleRulesDedup(t *testing.T) {
	rules := []config.NotificationRule{
		{
			Services:    []string{"default/*"},
			Transitions: []string{"unhealthy"},
			Channels:    []string{"webhook"},
		},
		{
			Services:    []string{"*"},
			Transitions: []string{"unhealthy"},
			Channels:    []string{"webhook"},
		},
	}
	m := NewRuleMatcher(rules)

	result := m.Match("default/api", state.StatusUnhealthy)
	if len(result) != 1 {
		t.Errorf("expected deduped to 1, got %v", result)
	}
}

func TestRuleMatcher_MultipleRulesIndependent(t *testing.T) {
	rules := []config.NotificationRule{
		{
			Services:    []string{"default/*"},
			Transitions: []string{"unhealthy"},
			Channels:    []string{"webhook"},
		},
		{
			Services:    []string{"*"},
			Transitions: []string{"unhealthy"},
			Channels:    []string{"slack"},
		},
	}
	m := NewRuleMatcher(rules)

	result := m.Match("default/api", state.StatusUnhealthy)
	if len(result) != 2 {
		t.Errorf("expected 2 channels, got %v", result)
	}
	// Check both present
	found := map[string]bool{}
	for _, ch := range result {
		found[ch] = true
	}
	if !found["webhook"] || !found["slack"] {
		t.Errorf("expected webhook and slack, got %v", result)
	}
}

func TestRuleMatcher_NoMatch(t *testing.T) {
	rules := []config.NotificationRule{
		{
			Services:    []string{"prod/*"},
			Transitions: []string{"unhealthy"},
			Channels:    []string{"webhook"},
		},
	}
	m := NewRuleMatcher(rules)

	result := m.Match("default/api", state.StatusUnhealthy)
	if len(result) != 0 {
		t.Errorf("expected no match, got %v", result)
	}
}

func TestRuleMatcher_ComplexGlob(t *testing.T) {
	tests := []struct {
		name       string
		pattern    string
		serviceKey string
		want       bool
	}{
		{"prod prefix", "prod/api-*", "prod/api-gateway", true},
		{"prod prefix no match", "prod/api-*", "prod/web-server", false},
		{"any namespace frontend", "*/frontend", "prod/frontend", true},
		{"any namespace frontend no match", "*/frontend", "prod/backend", false},
		{"exact match", "default/api-gateway", "default/api-gateway", true},
		{"exact no match", "default/api-gateway", "default/other", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules := []config.NotificationRule{
				{
					Services: []string{tt.pattern},
					Channels: []string{"hook"},
				},
			}
			m := NewRuleMatcher(rules)
			result := m.Match(tt.serviceKey, state.StatusUnhealthy)
			got := len(result) > 0
			if got != tt.want {
				t.Errorf("pattern %q vs %q: got %v, want %v", tt.pattern, tt.serviceKey, got, tt.want)
			}
		})
	}
}

func TestRuleMatcher_RecoveryTransition(t *testing.T) {
	rules := []config.NotificationRule{
		{
			Services:    []string{"*"},
			Transitions: []string{"healthy"},
			Channels:    []string{"webhook"},
		},
	}
	m := NewRuleMatcher(rules)

	result := m.Match("default/api", state.StatusHealthy)
	if len(result) != 1 {
		t.Errorf("expected match for recovery, got %v", result)
	}
}
