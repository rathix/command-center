package notify

import (
	"context"
	"log/slog"
	"time"

	"github.com/rathix/command-center/internal/state"
)

// Adapter delivers a notification to an external system.
type Adapter interface {
	Name() string
	Send(ctx context.Context, n Notification) error
}

// Notification is the payload sent to adapters.
type Notification struct {
	ServiceName string             `json:"serviceName"`
	Namespace   string             `json:"namespace"`
	PrevState   state.HealthStatus `json:"prevState"`
	NewState    state.HealthStatus `json:"newState"`
	Timestamp   time.Time          `json:"timestamp"`
	Signals     []string           `json:"signals,omitempty"`
	PodDiag     *state.PodDiagnostic `json:"podDiagnostic,omitempty"`
	Escalated   bool               `json:"escalated,omitempty"`
}

// Option configures the Engine.
type Option func(*Engine)

// WithLogger sets the logger for the engine.
func WithLogger(l *slog.Logger) Option {
	return func(e *Engine) {
		e.logger = l
	}
}
