package health

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rathix/command-center/internal/history"
	"github.com/rathix/command-center/internal/state"
)

// HTTPProber abstracts *http.Client for testability.
type HTTPProber interface {
	Do(req *http.Request) (*http.Response, error)
}

// StateReader provides read access to the service store.
type StateReader interface {
	All() []state.Service
	Get(namespace, name string) (state.Service, bool)
}

// StateWriter provides write access to the service store.
type StateWriter interface {
	AddOrUpdate(svc state.Service)
	Update(namespace, name string, fn func(*state.Service))
}

// TokenProvider provides OIDC tokens for authenticated health checks.
type TokenProvider interface {
	GetToken(ctx context.Context) (string, error)
}

// EndpointStrategy represents the auth resolution strategy for an auth-blocked service.
type EndpointStrategy struct {
	Type     string // "healthEndpoint" or "oidcAuth"
	Endpoint string // non-empty when Type == "healthEndpoint"
}

// EndpointDiscoverer discovers health endpoints for auth-blocked services.
type EndpointDiscoverer interface {
	GetStrategy(serviceKey string) *EndpointStrategy
	Discover(ctx context.Context, serviceKey string, baseURL string) (*EndpointStrategy, error)
	ClearStrategy(serviceKey string)
}

// Checker performs periodic HTTP health checks against discovered services.
type Checker struct {
	reader        StateReader
	writer        StateWriter
	client        HTTPProber
	interval      time.Duration
	historyWriter history.HistoryWriter
	logger        *slog.Logger

	// Optional OIDC fields — nil means MVP behavior (no auth retry).
	tokenProvider TokenProvider
	discoverer    EndpointDiscoverer
}

// NewChecker creates a new health checker. If logger is nil, a no-op logger is used.
func NewChecker(reader StateReader, writer StateWriter, client HTTPProber, interval time.Duration, historyWriter history.HistoryWriter, logger *slog.Logger) *Checker {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if historyWriter == nil {
		historyWriter = history.NoopWriter{}
	}
	return &Checker{
		reader:        reader,
		writer:        writer,
		client:        client,
		interval:      interval,
		historyWriter: historyWriter,
		logger:        logger,
	}
}

// WithTokenProvider sets the OIDC token provider for authenticated health check retries.
func (c *Checker) WithTokenProvider(tp TokenProvider) *Checker {
	c.tokenProvider = tp
	return c
}

// WithDiscoverer sets the endpoint discoverer for auth-blocked services.
func (c *Checker) WithDiscoverer(d EndpointDiscoverer) *Checker {
	c.discoverer = d
	return c
}

// Run starts the health check loop. It performs an immediate check on start,
// then checks at the configured interval. It returns when ctx is cancelled.
func (c *Checker) Run(ctx context.Context) {
	c.checkAll(ctx)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.checkAll(ctx)
		}
	}
}

// checkAll performs a health check cycle across all discovered services.
func (c *Checker) checkAll(ctx context.Context) {
	services := c.reader.All()
	if len(services) == 0 {
		return
	}

	start := time.Now()

	var wg sync.WaitGroup
	wg.Add(len(services))
	for _, svc := range services {
		go func(s state.Service) {
			defer wg.Done()

			// Determine probe URL: HealthURL overrides base URL
			probeURL := s.URL
			if s.HealthURL != "" {
				probeURL = s.HealthURL
			}

			// Perform the probe
			result := c.probeService(ctx, probeURL)

			// After initial probe returns auth-blocked, attempt OIDC resolution
			if result.status == state.StatusAuthBlocked {
				if c.tokenProvider != nil {
					result = c.resolveAuthBlocked(ctx, s, result)
				}
			}

			// Override status classification if ExpectedStatusCodes is set
			if len(s.ExpectedStatusCodes) > 0 && result.httpCode != nil {
				if containsInt(s.ExpectedStatusCodes, *result.httpCode) {
					result.status = state.StatusHealthy
					result.errorSnippet = nil
				}
			}

			var transition *history.TransitionRecord
			// Atomically update only health fields
			c.writer.Update(s.Namespace, s.Name, func(svc *state.Service) {
				transition = c.applyResult(svc, result)
			})
			if transition != nil {
				c.recordTransition(*transition)
			}
		}(svc)
	}
	wg.Wait()

	c.logger.Info("health check cycle complete",
		"services", len(services),
		"durationMs", time.Since(start).Milliseconds(),
	)
}

const maxSnippetLen = 256

type probeResult struct {
	status         state.HealthStatus
	httpCode       *int
	responseTimeMs int64
	errorSnippet   *string
	authMethod     string
}

// probeService performs a single HTTP GET health check against a service URL.
func (c *Checker) probeService(ctx context.Context, url string) probeResult {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return probeResult{
			status:       state.StatusUnhealthy,
			errorSnippet: ptrString(err.Error()),
			authMethod:   "",
		}
	}

	start := time.Now()
	resp, err := c.client.Do(req)
	responseTimeMs := time.Since(start).Milliseconds()

	if err != nil {
		errMsg := err.Error()
		return probeResult{
			status:         state.StatusUnhealthy,
			responseTimeMs: responseTimeMs,
			errorSnippet:   &errMsg,
			authMethod:     "",
		}
	}
	defer resp.Body.Close()

	code := resp.StatusCode
	newStatus := classifyStatus(code)

	var snippet *string
	if newStatus == state.StatusUnhealthy {
		snippet = readSnippet(resp.Body)
	}

	return probeResult{
		status:         newStatus,
		httpCode:       &code,
		responseTimeMs: responseTimeMs,
		errorSnippet:   snippet,
		authMethod:     "",
	}
}

// resolveAuthBlocked attempts to resolve an auth-blocked service using endpoint
// discovery and/or OIDC authentication. If tokenProvider is nil, it returns the
// initial result unchanged (MVP behavior).
func (c *Checker) resolveAuthBlocked(ctx context.Context, svc state.Service, initialResult probeResult) probeResult {
	if c.tokenProvider == nil {
		return initialResult
	}

	serviceKey := svc.Namespace + "/" + svc.Name

	if c.discoverer != nil {
		// Check for a cached strategy first
		strategy := c.discoverer.GetStrategy(serviceKey)

		if strategy == nil {
			// No cache — run discovery
			discovered, err := c.discoverer.Discover(ctx, serviceKey, svc.URL)
			if err == nil && discovered != nil && discovered.Type == "healthEndpoint" && discovered.Endpoint != "" {
				result := c.probeService(ctx, discovered.Endpoint)
				if result.status == state.StatusHealthy {
					return result
				}
				// Discovery endpoint didn't work — clear and fall through to OIDC
				c.discoverer.ClearStrategy(serviceKey)
			}
		} else if strategy.Type == "healthEndpoint" && strategy.Endpoint != "" {
			// Cached health endpoint — probe it
			result := c.probeService(ctx, strategy.Endpoint)
			if result.status == state.StatusHealthy {
				return result
			}
			// Cached endpoint failed — clear and fall through to OIDC
			c.discoverer.ClearStrategy(serviceKey)
		}
	}

	// OIDC authenticated retry (fall-through path)
	token, err := c.tokenProvider.GetToken(ctx)
	if err != nil {
		c.logger.Warn("OIDC token acquisition failed",
			"service", svc.Name,
			"namespace", svc.Namespace,
			"error", err,
		)
		return initialResult
	}

	return c.probeServiceWithAuth(ctx, svc.URL, token)
}

// probeServiceWithAuth performs a single HTTP GET health check with an Authorization header.
func (c *Checker) probeServiceWithAuth(ctx context.Context, url, token string) probeResult {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return probeResult{
			status:       state.StatusUnhealthy,
			errorSnippet: ptrString(err.Error()),
			authMethod:   "",
		}
	}

	req.Header.Set("Authorization", "Bearer "+token)

	start := time.Now()
	resp, err := c.client.Do(req)
	responseTimeMs := time.Since(start).Milliseconds()

	if err != nil {
		errMsg := err.Error()
		return probeResult{
			status:         state.StatusUnhealthy,
			responseTimeMs: responseTimeMs,
			errorSnippet:   &errMsg,
			authMethod:     "oidc",
		}
	}
	defer resp.Body.Close()

	code := resp.StatusCode
	newStatus := classifyStatus(code)

	var snippet *string
	if newStatus == state.StatusUnhealthy {
		snippet = readSnippet(resp.Body)
	}

	return probeResult{
		status:         newStatus,
		httpCode:       &code,
		responseTimeMs: responseTimeMs,
		errorSnippet:   snippet,
		authMethod:     "oidc",
	}
}

func (c *Checker) recordTransition(rec history.TransitionRecord) {
	if err := c.historyWriter.Record(rec); err != nil {
		c.logger.Warn("history write failed", "service", rec.ServiceKey, "error", err)
	}
}

// applyResult updates health fields on a service, preserving non-health fields.
// It returns a history transition record when a status transition occurred.
func (c *Checker) applyResult(svc *state.Service, res probeResult) *history.TransitionRecord {
	previousStatus := svc.Status

	svc.Status = res.status
	svc.HTTPCode = res.httpCode
	svc.ResponseTimeMs = &res.responseTimeMs
	svc.ErrorSnippet = res.errorSnippet
	svc.AuthMethod = res.authMethod

	now := time.Now()
	svc.LastChecked = &now

	var transition *history.TransitionRecord
	if res.status != previousStatus {
		svc.LastStateChange = &now
		c.logger.Info("service health changed",
			"service", svc.Name,
			"namespace", svc.Namespace,
			"from", string(previousStatus),
			"to", string(res.status),
		)

		rec := history.TransitionRecord{
			Timestamp:  now,
			ServiceKey: svc.Namespace + "/" + svc.Name,
			PrevStatus: previousStatus,
			NextStatus: res.status,
			HTTPCode:   res.httpCode,
			ResponseMs: &res.responseTimeMs,
		}
		transition = &rec
	}

	logArgs := []any{
		"service", svc.Name,
		"namespace", svc.Namespace,
		"status", string(res.status),
		"responseTimeMs", res.responseTimeMs,
	}
	if res.httpCode != nil {
		logArgs = append(logArgs, "httpCode", *res.httpCode)
	}
	c.logger.Debug("health check completed", logArgs...)
	return transition
}

// classifyStatus maps an HTTP status code to a HealthStatus.
func classifyStatus(code int) state.HealthStatus {
	switch {
	case code >= 200 && code <= 299:
		return state.StatusHealthy
	case code == 401 || code == 403:
		return state.StatusAuthBlocked
	default:
		return state.StatusUnhealthy
	}
}

// readSnippet reads the first line of the response body, truncated to maxSnippetLen.
func readSnippet(body io.Reader) *string {
	// Use a LimitedReader to avoid reading massive bodies
	lr := &io.LimitedReader{R: body, N: maxSnippetLen}
	data, err := io.ReadAll(lr)
	if err != nil || len(data) == 0 {
		return nil
	}

	s := string(data)
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		s = s[:idx]
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

func ptrString(s string) *string {
	return &s
}

func containsInt(slice []int, val int) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}
