package health

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

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

// Checker performs periodic HTTP health checks against discovered services.
type Checker struct {
	reader   StateReader
	writer   StateWriter
	client   HTTPProber
	interval time.Duration
	logger   *slog.Logger
}

// NewChecker creates a new health checker. If logger is nil, a no-op logger is used.
func NewChecker(reader StateReader, writer StateWriter, client HTTPProber, interval time.Duration, logger *slog.Logger) *Checker {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Checker{
		reader:   reader,
		writer:   writer,
		client:   client,
		interval: interval,
		logger:   logger,
	}
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

			// Override status classification if ExpectedStatusCodes is set
			if len(s.ExpectedStatusCodes) > 0 && result.httpCode != nil {
				if containsInt(s.ExpectedStatusCodes, *result.httpCode) {
					result.status = state.StatusHealthy
					result.errorSnippet = nil
				}
			}

			// Atomically update only health fields
			c.writer.Update(s.Namespace, s.Name, func(svc *state.Service) {
				c.applyResult(svc, result)
			})
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
}

// probeService performs a single HTTP GET health check against a service URL.
func (c *Checker) probeService(ctx context.Context, url string) probeResult {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return probeResult{
			status:       state.StatusUnhealthy,
			errorSnippet: ptrString(err.Error()),
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
	}
}

// applyResult updates health fields on a service, preserving non-health fields.
func (c *Checker) applyResult(svc *state.Service, res probeResult) {
	previousStatus := svc.Status

	svc.Status = res.status
	svc.HTTPCode = res.httpCode
	svc.ResponseTimeMs = &res.responseTimeMs
	svc.ErrorSnippet = res.errorSnippet

	now := time.Now()
	svc.LastChecked = &now

	if res.status != previousStatus {
		svc.LastStateChange = &now
		c.logger.Info("service health changed",
			"service", svc.Name,
			"namespace", svc.Namespace,
			"from", string(previousStatus),
			"to", string(res.status),
		)
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
