package sse

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/rathix/command-center/internal/state"
)

// StateSource is the interface the broker uses to read current state and subscribe to changes.
// Defined here at the consumer, not in the state package.
type StateSource interface {
	All() []state.Service
	Subscribe() <-chan state.Event
	K8sConnected() bool
	LastK8sEvent() time.Time
	ConfigErrors() []string
}

// OIDCStatusProvider returns the current OIDC status for SSE state events.
// Returns nil when OIDC is not configured.
type OIDCStatusProvider interface {
	GetOIDCStatus() *OIDCStatus
}

const defaultKeepaliveInterval = 15 * time.Second

// sseEvent is an internal representation of a formatted SSE message ready to write.
type sseEvent struct {
	data []byte
}

// Broker manages SSE client connections and broadcasts state events.
type Broker struct {
	source              StateSource
	oidcStatusProvider  OIDCStatusProvider
	logger              *slog.Logger
	appVersion          string
	healthCheckInterval time.Duration
	clients             map[chan sseEvent]struct{}
	keepaliveInterval   time.Duration
	mu                  sync.Mutex
}

// NewBroker creates a new SSE broker. The oidcStatus parameter may be nil
// when OIDC is not configured — in that case, oidcStatus is omitted from state events.
func NewBroker(source StateSource, oidcStatus OIDCStatusProvider, logger *slog.Logger, appVersion string, healthCheckInterval time.Duration) *Broker {
	return newBrokerWithKeepalive(source, oidcStatus, logger, appVersion, healthCheckInterval, defaultKeepaliveInterval)
}

func newBrokerWithKeepalive(source StateSource, oidcStatus OIDCStatusProvider, logger *slog.Logger, appVersion string, healthCheckInterval time.Duration, keepaliveInterval time.Duration) *Broker {
	if keepaliveInterval <= 0 {
		keepaliveInterval = defaultKeepaliveInterval
	}

	return &Broker{
		source:              source,
		oidcStatusProvider:  oidcStatus,
		logger:              logger,
		appVersion:          appVersion,
		healthCheckInterval: healthCheckInterval,
		clients:             make(map[chan sseEvent]struct{}),
		keepaliveInterval:   keepaliveInterval,
	}
}

// Run listens on the store's event channel and broadcasts to all connected clients.
// It blocks until the context is cancelled.
func (b *Broker) Run(ctx context.Context) {
	events := b.source.Subscribe()

	for {
		select {
		case <-ctx.Done():
			b.closeAllClients()
			b.logger.Info("SSE broker stopped")
			return
		case evt, ok := <-events:
			if !ok {
				b.closeAllClients()
				b.logger.Warn("SSE broker source channel closed")
				return
			}

			var data []byte
			var err error

			switch evt.Type {
			case state.EventDiscovered:
				data, err = formatSSEEvent("discovered", discoveredEventPayloadFromService(evt.Service))
			case state.EventUpdated:
				data, err = formatSSEEvent("update", discoveredEventPayloadFromService(evt.Service))
			case state.EventRemoved:
				data, err = formatSSEEvent("removed", RemovedEventPayload{
					Name:      evt.Name,
					Namespace: evt.Namespace,
				})
			case state.EventK8sStatus:
				k8sLastEvent := b.source.LastK8sEvent().UTC().Format(time.RFC3339)
				data, err = formatSSEEvent("k8sStatus", K8sStatusPayload{
					K8sConnected: b.source.K8sConnected(),
					K8sLastEvent: k8sLastEvent,
				})
			case state.EventConfigErrors:
				data, err = b.buildStateEvent()
			default:
				b.logger.Debug("unknown state event type", "type", evt.Type)
				continue
			}

			if err != nil {
				b.logger.Debug("failed to format SSE event", "error", err)
				continue
			}

			b.broadcast(sseEvent{data: data})
			b.logger.Debug("SSE event broadcast", "type", evt.Type)
		}
	}
}

func (b *Broker) closeAllClients() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.clients {
		close(ch)
		delete(b.clients, ch)
	}
}

// broadcast sends an event to all connected clients using non-blocking sends.
func (b *Broker) broadcast(evt sseEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.clients {
		select {
		case ch <- evt:
		default:
			// Client too slow, skip this event
		}
	}
}

// addClient registers a new client channel.
func (b *Broker) addClient(ch chan sseEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.clients[ch] = struct{}{}
	b.logger.Info("SSE client connected", "clients", len(b.clients))
}

// removeClient unregisters and closes a client channel.
func (b *Broker) removeClient(ch chan sseEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.clients, ch)
	b.logger.Info("SSE client disconnected", "clients", len(b.clients))
}

// ServeHTTP handles SSE connections: sets headers, sends initial state, and streams events.
func (b *Broker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Register client before sending the initial snapshot so no updates are missed.
	clientCh := make(chan sseEvent, 64)
	b.addClient(clientCh)
	defer b.removeClient(clientCh)

	// Send initial state event with all current services.
	initialData, err := b.buildStateEvent()
	if err != nil {
		b.logger.Debug("failed to format initial state event", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := writeAndFlush(w, flusher, initialData); err != nil {
		b.logger.Debug("failed to write initial state event", "error", err)
		return
	}

	// Keepalive ticker — sends comment every 15 seconds when no events.
	keepalive := time.NewTicker(b.keepaliveInterval)
	defer keepalive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case evt, ok := <-clientCh:
			if !ok {
				// Channel closed by broker shutdown.
				return
			}
			if err := writeAndFlush(w, flusher, evt.data); err != nil {
				b.logger.Debug("failed to write SSE event", "error", err)
				return
			}
			keepalive.Reset(b.keepaliveInterval)
		case <-keepalive.C:
			if err := writeAndFlush(w, flusher, formatKeepalive()); err != nil {
				b.logger.Debug("failed to write keepalive", "error", err)
				return
			}
		}
	}
}

func writeAndFlush(w http.ResponseWriter, flusher http.Flusher, payload []byte) error {
	if _, err := w.Write(payload); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

func (b *Broker) buildStateEvent() ([]byte, error) {
	services := b.source.All()
	var k8sLastEvent *time.Time
	if t := b.source.LastK8sEvent(); !t.IsZero() {
		k8sLastEvent = &t
	}
	var oidcStatus *OIDCStatus
	if b.oidcStatusProvider != nil {
		oidcStatus = b.oidcStatusProvider.GetOIDCStatus()
	}
	return formatSSEEvent("state", StateEventPayload{
		AppVersion:            b.appVersion,
		Services:              services,
		K8sConnected:          b.source.K8sConnected(),
		K8sLastEvent:          k8sLastEvent,
		HealthCheckIntervalMs: int(b.healthCheckInterval.Milliseconds()),
		ConfigErrors:          b.source.ConfigErrors(),
		OIDCStatus:            oidcStatus,
	})
}
