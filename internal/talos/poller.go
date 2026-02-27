package talos

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

const defaultHistorySize = 60

// Poller periodically fetches node health and metrics from a NodeClient.
// It stores the latest state and provides thread-safe access via GetNodes
// and GetMetricsHistory.
type Poller struct {
	client   NodeClient
	interval time.Duration
	logger   *slog.Logger

	mu             sync.RWMutex
	nodes          []NodeHealth
	lastPoll       time.Time
	lastError      error
	metricsHistory map[string][]NodeMetrics
	historySize    int
}

// NewPoller creates a Poller that polls the given client at the specified interval.
func NewPoller(client NodeClient, interval time.Duration, logger *slog.Logger) *Poller {
	return &Poller{
		client:         client,
		interval:       interval,
		logger:         logger,
		metricsHistory: make(map[string][]NodeMetrics),
		historySize:    defaultHistorySize,
	}
}

// Run polls in a loop at the configured interval until ctx is cancelled.
func (p *Poller) Run(ctx context.Context) {
	p.poll(ctx)
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.poll(ctx)
		}
	}
}

func (p *Poller) poll(ctx context.Context) {
	nodes, err := p.client.ListNodes(ctx)

	p.mu.Lock()
	defer p.mu.Unlock()

	if err != nil {
		p.lastError = err
		p.logger.Warn("talos node poll failed", "error", err)
		return
	}

	p.nodes = nodes
	p.lastError = nil
	p.lastPoll = time.Now()

	// Collect metrics (independent of node health)
	metrics, metricsErr := p.client.GetMetrics(ctx)
	if metricsErr != nil {
		p.logger.Warn("talos metrics poll failed", "error", metricsErr)
		// Retain last-known metrics on nodes; don't clear them
	} else {
		// Merge metrics into nodes by name
		for i := range p.nodes {
			if m, ok := metrics[p.nodes[i].Name]; ok {
				m.Timestamp = time.Now()
				p.nodes[i].Metrics = &m
				// Append to history ring buffer
				hist := p.metricsHistory[p.nodes[i].Name]
				if len(hist) >= p.historySize {
					hist = hist[1:]
				}
				hist = append(hist, m)
				p.metricsHistory[p.nodes[i].Name] = hist
			}
		}
	}
}

// GetNodes returns the current node health data, the time of the last successful
// poll, and the last error (if any). Thread-safe via RWMutex.
func (p *Poller) GetNodes() ([]NodeHealth, time.Time, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	// Return a copy to prevent callers from mutating internal state
	out := make([]NodeHealth, len(p.nodes))
	copy(out, p.nodes)
	return out, p.lastPoll, p.lastError
}

// GetMetricsHistory returns the metrics history for a specific node.
func (p *Poller) GetMetricsHistory(nodeName string) []NodeMetrics {
	p.mu.RLock()
	defer p.mu.RUnlock()
	hist := p.metricsHistory[nodeName]
	out := make([]NodeMetrics, len(hist))
	copy(out, hist)
	return out
}

// Interval returns the configured poll interval.
func (p *Poller) Interval() time.Duration {
	return p.interval
}
