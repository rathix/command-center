package auth

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
)

// EndpointStrategy represents the resolved health check approach for a service.
type EndpointStrategy struct {
	Type     string // "healthEndpoint" or "oidcAuth"
	Endpoint string // Full probe URL (only set when Type == "healthEndpoint")
}

// HTTPProber abstracts *http.Client for testability.
// Defined at the consumer (same shape as health.HTTPProber but independently defined).
type HTTPProber interface {
	Do(req *http.Request) (*http.Response, error)
}

// probePaths is the ordered list of health endpoint paths to probe.
// Discovery stops at the first path returning a 2xx status code.
var probePaths = []string{"/healthz", "/health", "/ping", "/api/health"}

// EndpointDiscoverer probes common health endpoints and caches the result per service.
type EndpointDiscoverer struct {
	client HTTPProber
	mu     sync.RWMutex
	cache  map[string]*EndpointStrategy // keyed by "namespace/name"
	logger *slog.Logger
}

// NewEndpointDiscoverer creates a new EndpointDiscoverer. If logger is nil, a no-op logger is used.
func NewEndpointDiscoverer(client HTTPProber, logger *slog.Logger) *EndpointDiscoverer {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &EndpointDiscoverer{
		client: client,
		cache:  make(map[string]*EndpointStrategy),
		logger: logger,
	}
}

// Discover probes health endpoints in order for the given service.
// It returns the first successful strategy and caches the result.
// If a cached strategy exists, it is returned without re-probing.
func (d *EndpointDiscoverer) Discover(ctx context.Context, serviceKey, baseURL string) (*EndpointStrategy, error) {
	// Check cache under read-lock first
	d.mu.RLock()
	if cached, ok := d.cache[serviceKey]; ok {
		d.mu.RUnlock()
		return cached, nil
	}
	d.mu.RUnlock()

	// Probe each path sequentially (mutex NOT held during I/O)
	for _, path := range probePaths {
		probeURL := buildProbeURL(baseURL, path)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, probeURL, nil)
		if err != nil {
			d.logger.Debug("probing health endpoint",
				"serviceKey", serviceKey,
				"url", probeURL,
				"error", err,
			)
			continue
		}

		resp, err := d.client.Do(req)
		if err != nil {
			// Context cancellation should stop discovery immediately
			if ctx.Err() != nil {
				return nil, fmt.Errorf("endpoint discovery cancelled for %s: %w", serviceKey, ctx.Err())
			}
			d.logger.Debug("probing health endpoint",
				"serviceKey", serviceKey,
				"url", probeURL,
				"error", err,
			)
			continue
		}
		code := resp.StatusCode
		resp.Body.Close()

		d.logger.Debug("probing health endpoint",
			"serviceKey", serviceKey,
			"url", probeURL,
			"statusCode", code,
		)

		if code >= 200 && code <= 299 {
			strategy := &EndpointStrategy{
				Type:     "healthEndpoint",
				Endpoint: probeURL,
			}
			d.mu.Lock()
			d.cache[serviceKey] = strategy
			d.mu.Unlock()
			d.logger.Info("health endpoint discovered",
				"serviceKey", serviceKey,
				"endpoint", probeURL,
			)
			return strategy, nil
		}
	}

	// All probes failed — fall back to OIDC
	strategy := &EndpointStrategy{
		Type: "oidcAuth",
	}
	d.mu.Lock()
	d.cache[serviceKey] = strategy
	d.mu.Unlock()
	d.logger.Info("no health endpoint found, using OIDC",
		"serviceKey", serviceKey,
	)
	return strategy, nil
}

// GetStrategy returns the cached endpoint strategy for a service, or nil if not cached.
func (d *EndpointDiscoverer) GetStrategy(serviceKey string) *EndpointStrategy {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.cache[serviceKey]
}

// ClearStrategy removes the cached endpoint strategy for a service.
func (d *EndpointDiscoverer) ClearStrategy(serviceKey string) {
	d.mu.Lock()
	delete(d.cache, serviceKey)
	d.mu.Unlock()
	d.logger.Info("cleared endpoint strategy",
		"serviceKey", serviceKey,
	)
}

// buildProbeURL constructs a probe URL from a base URL and probe path.
// It strips trailing slashes from baseURL to prevent double-slash URLs.
func buildProbeURL(baseURL, path string) string {
	return strings.TrimRight(baseURL, "/") + path
}
