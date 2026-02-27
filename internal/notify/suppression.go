package notify

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/rathix/command-center/internal/config"
	"github.com/rathix/command-center/internal/state"
)

// DecisionAction indicates the outcome of suppression evaluation.
type DecisionAction int

const (
	// Allow means the notification should be sent.
	Allow DecisionAction = iota
	// Suppress means the notification should be dropped.
	Suppress
	// Escalate means the notification should be sent to escalation channels.
	Escalate
)

// Decision is the result of evaluating suppression/escalation for a notification.
type Decision struct {
	Action   DecisionAction
	Channels []string
}

// ReminderAction describes a reminder notification due for a still-unhealthy service.
type ReminderAction struct {
	ServiceKey   string
	RuleIdx      int
	Channels     []string
	Notification Notification
}

// ServiceRuleState tracks suppression/escalation state for one (service, rule) pair.
type ServiceRuleState struct {
	LastNotifiedAt time.Time
	UnhealthySince time.Time
	Escalated      bool
}

// SuppressionOption configures the SuppressionEngine.
type SuppressionOption func(*SuppressionEngine)

// SuppressionEngine manages per-(service, rule) suppression and escalation state.
type SuppressionEngine struct {
	mu     sync.Mutex
	states map[string]*ServiceRuleState
	clock  func() time.Time
	logger *slog.Logger
}

// NewSuppressionEngine creates a new suppression engine.
func NewSuppressionEngine(opts ...SuppressionOption) *SuppressionEngine {
	se := &SuppressionEngine{
		states: make(map[string]*ServiceRuleState),
		clock:  time.Now,
		logger: slog.Default(),
	}
	for _, opt := range opts {
		opt(se)
	}
	return se
}

// WithClock sets an injectable clock for testing.
func WithClock(clock func() time.Time) SuppressionOption {
	return func(se *SuppressionEngine) {
		se.clock = clock
	}
}

// WithSuppressionLogger sets the logger.
func WithSuppressionLogger(l *slog.Logger) SuppressionOption {
	return func(se *SuppressionEngine) {
		se.logger = l
	}
}

func stateKey(serviceKey string, ruleIdx int) string {
	return fmt.Sprintf("%s:%d", serviceKey, ruleIdx)
}

// Evaluate decides whether a notification should be sent, suppressed, or escalated.
func (se *SuppressionEngine) Evaluate(
	serviceKey string,
	ruleIdx int,
	rule config.NotificationRule,
	newState state.HealthStatus,
) Decision {
	se.mu.Lock()
	defer se.mu.Unlock()

	now := se.clock()
	key := stateKey(serviceKey, ruleIdx)

	// Recovery always allowed; state is reset separately
	if newState == state.StatusHealthy {
		return Decision{Action: Allow, Channels: rule.Channels}
	}

	// Parse suppression interval
	suppressionInterval := parseDurationClamped(rule.SuppressionInterval, time.Minute, se.logger)
	escalateAfter := parseDurationUnclamped(rule.EscalateAfter)

	st, exists := se.states[key]
	if !exists {
		// First notification for this service/rule: allow
		se.states[key] = &ServiceRuleState{
			LastNotifiedAt: now,
			UnhealthySince: now,
		}
		return Decision{Action: Allow, Channels: rule.Channels}
	}

	// Check escalation
	if escalateAfter > 0 && !st.Escalated && now.Sub(st.UnhealthySince) >= escalateAfter {
		st.Escalated = true
		st.LastNotifiedAt = now
		channels := append([]string{}, rule.Channels...)
		channels = append(channels, rule.EscalationChannels...)
		return Decision{Action: Escalate, Channels: channels}
	}

	// Check suppression
	if suppressionInterval > 0 {
		if now.Sub(st.LastNotifiedAt) < suppressionInterval {
			return Decision{Action: Suppress}
		}
		// Interval elapsed: allow reminder
		st.LastNotifiedAt = now
		return Decision{Action: Allow, Channels: rule.Channels}
	}

	// No suppression configured: always allow
	st.LastNotifiedAt = now
	return Decision{Action: Allow, Channels: rule.Channels}
}

// Reset clears all state for a service (called on recovery).
func (se *SuppressionEngine) Reset(serviceKey string) {
	se.mu.Lock()
	defer se.mu.Unlock()

	prefix := serviceKey + ":"
	for k := range se.states {
		if strings.HasPrefix(k, prefix) {
			delete(se.states, k)
		}
	}
}

// CheckReminders returns reminder actions for services still in a bad state
// past their suppression interval.
func (se *SuppressionEngine) CheckReminders(
	rules []config.NotificationRule,
	currentStates map[string]state.HealthStatus,
) []ReminderAction {
	se.mu.Lock()
	defer se.mu.Unlock()

	now := se.clock()
	var reminders []ReminderAction

	for key, st := range se.states {
		parts := strings.SplitN(key, ":", 2)
		if len(parts) != 2 {
			continue
		}
		serviceKey := parts[0]
		ruleIdx := 0
		fmt.Sscanf(parts[1], "%d", &ruleIdx)

		if ruleIdx >= len(rules) {
			continue
		}
		rule := rules[ruleIdx]

		currentStatus, ok := currentStates[serviceKey]
		if !ok {
			continue
		}
		// Only remind for non-healthy states
		if currentStatus == state.StatusHealthy {
			continue
		}

		suppressionInterval := parseDurationClamped(rule.SuppressionInterval, time.Minute, se.logger)
		if suppressionInterval <= 0 {
			continue
		}

		if now.Sub(st.LastNotifiedAt) >= suppressionInterval {
			st.LastNotifiedAt = now

			channels := rule.Channels
			// Check escalation during reminders
			escalateAfter := parseDurationUnclamped(rule.EscalateAfter)
			if escalateAfter > 0 && !st.Escalated && now.Sub(st.UnhealthySince) >= escalateAfter {
				st.Escalated = true
				channels = append(append([]string{}, rule.Channels...), rule.EscalationChannels...)
			}

			nsParts := strings.SplitN(serviceKey, "/", 2)
			ns, name := "", serviceKey
			if len(nsParts) == 2 {
				ns, name = nsParts[0], nsParts[1]
			}

			reminders = append(reminders, ReminderAction{
				ServiceKey: serviceKey,
				RuleIdx:    ruleIdx,
				Channels:   channels,
				Notification: Notification{
					ServiceName: name,
					Namespace:   ns,
					PrevState:   currentStatus,
					NewState:    currentStatus,
					Timestamp:   now,
				},
			})
		}
	}

	return reminders
}

// parseDurationClamped parses a duration string and clamps to a minimum of minDur.
// Returns 0 for empty strings.
func parseDurationClamped(s string, minDur time.Duration, logger *slog.Logger) time.Duration {
	if s == "" {
		return 0
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		logger.Warn("invalid duration, ignoring", "value", s, "error", err)
		return 0
	}
	if d < minDur {
		logger.Warn("duration below minimum, clamping", "value", s, "minimum", minDur)
		return minDur
	}
	return d
}

// parseDurationUnclamped parses a duration string with no minimum.
func parseDurationUnclamped(s string) time.Duration {
	if s == "" {
		return 0
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0
	}
	return d
}
