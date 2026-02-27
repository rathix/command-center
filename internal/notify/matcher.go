package notify

import (
	"path"
	"strings"

	"github.com/rathix/command-center/internal/config"
	"github.com/rathix/command-center/internal/state"
)

// RuleMatcher evaluates notification rules against service transitions.
type RuleMatcher struct {
	rules []config.NotificationRule
}

// NewRuleMatcher creates a rule matcher from notification rules.
func NewRuleMatcher(rules []config.NotificationRule) *RuleMatcher {
	return &RuleMatcher{rules: rules}
}

// Rules returns the configured rules.
func (m *RuleMatcher) Rules() []config.NotificationRule {
	return m.rules
}

// Match returns deduplicated adapter names that should receive a notification
// for the given service transition.
func (m *RuleMatcher) Match(serviceKey string, newState state.HealthStatus) []string {
	seen := make(map[string]struct{})
	var result []string

	for _, rule := range m.rules {
		if !ruleMatchesEvent(rule, serviceKey, newState) {
			continue
		}
		for _, ch := range rule.Channels {
			if _, ok := seen[ch]; !ok {
				seen[ch] = struct{}{}
				result = append(result, ch)
			}
		}
	}
	return result
}

// ruleMatchesEvent checks if a rule matches a given service key and new state.
func ruleMatchesEvent(rule config.NotificationRule, svcKey string, newState state.HealthStatus) bool {
	// Check service pattern match
	matched := false
	for _, pattern := range rule.Services {
		if matchGlob(pattern, svcKey) {
			matched = true
			break
		}
	}
	if !matched {
		return false
	}

	// Check transition filter (empty list matches all)
	if len(rule.Transitions) == 0 {
		return true
	}
	for _, t := range rule.Transitions {
		if strings.EqualFold(t, string(newState)) {
			return true
		}
	}
	return false
}

// matchGlob checks if a service key matches a glob pattern.
// The special pattern "*" is expanded to "*/*" for convenience.
func matchGlob(pattern, serviceKey string) bool {
	if pattern == "*" {
		pattern = "*/*"
	}
	matched, err := path.Match(pattern, serviceKey)
	if err != nil {
		return false
	}
	return matched
}
