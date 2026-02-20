package health

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rathix/command-center/internal/state"
)

// mockHTTPProber is a configurable mock for HTTPProber.
type mockHTTPProber struct {
	mu        sync.Mutex
	responses map[string]mockResponse // keyed by URL
}

type mockResponse struct {
	statusCode int
	body       string
	err        error
}

func (m *mockHTTPProber) Do(req *http.Request) (*http.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	resp, ok := m.responses[req.URL.String()]
	if !ok {
		return nil, errors.New("connection refused")
	}
	if resp.err != nil {
		return nil, resp.err
	}
	return &http.Response{
		StatusCode: resp.statusCode,
		Body:       io.NopCloser(strings.NewReader(resp.body)),
	}, nil
}

func TestCheckService_Healthy(t *testing.T) {
	store := state.NewStore()
	store.AddOrUpdate(state.Service{
		Name: "svc1", DisplayName: "Service One", Namespace: "ns1", URL: "https://svc1.example.com",
		Status: state.StatusUnknown,
	})

	client := &mockHTTPProber{
		responses: map[string]mockResponse{
			"https://svc1.example.com": {statusCode: 200, body: "OK"},
		},
	}

	checker := NewChecker(store, store, client, time.Hour, nil)
	svc, _ := store.Get("ns1", "svc1")
	result := checker.applyResult(svc, checker.probeService(context.Background(), svc.URL))

	if result.Status != state.StatusHealthy {
		t.Errorf("expected status %q, got %q", state.StatusHealthy, result.Status)
	}
	if result.HTTPCode == nil || *result.HTTPCode != 200 {
		t.Errorf("expected HTTPCode 200, got %v", result.HTTPCode)
	}
	if result.ResponseTimeMs == nil || *result.ResponseTimeMs < 0 {
		t.Errorf("expected non-negative ResponseTimeMs, got %v", result.ResponseTimeMs)
	}
	if result.ErrorSnippet != nil {
		t.Errorf("expected nil ErrorSnippet for healthy service, got %q", *result.ErrorSnippet)
	}
}

func TestCheckService_AuthBlocked401(t *testing.T) {
	store := state.NewStore()
	store.AddOrUpdate(state.Service{
		Name: "svc1", Namespace: "ns1", URL: "https://svc1.example.com",
		Status: state.StatusUnknown,
	})

	client := &mockHTTPProber{
		responses: map[string]mockResponse{
			"https://svc1.example.com": {statusCode: 401, body: "Unauthorized"},
		},
	}

	checker := NewChecker(store, store, client, time.Hour, nil)
	svc, _ := store.Get("ns1", "svc1")
	result := checker.applyResult(svc, checker.probeService(context.Background(), svc.URL))

	if result.Status != state.StatusAuthBlocked {
		t.Errorf("expected status %q, got %q", state.StatusAuthBlocked, result.Status)
	}
	if result.HTTPCode == nil || *result.HTTPCode != 401 {
		t.Errorf("expected HTTPCode 401, got %v", result.HTTPCode)
	}
}

func TestCheckService_AuthBlocked403(t *testing.T) {
	store := state.NewStore()
	store.AddOrUpdate(state.Service{
		Name: "svc1", Namespace: "ns1", URL: "https://svc1.example.com",
		Status: state.StatusUnknown,
	})

	client := &mockHTTPProber{
		responses: map[string]mockResponse{
			"https://svc1.example.com": {statusCode: 403, body: "Forbidden"},
		},
	}

	checker := NewChecker(store, store, client, time.Hour, nil)
	svc, _ := store.Get("ns1", "svc1")
	result := checker.applyResult(svc, checker.probeService(context.Background(), svc.URL))

	if result.Status != state.StatusAuthBlocked {
		t.Errorf("expected status %q, got %q", state.StatusAuthBlocked, result.Status)
	}
	if result.HTTPCode == nil || *result.HTTPCode != 403 {
		t.Errorf("expected HTTPCode 403, got %v", result.HTTPCode)
	}
}

func TestCheckService_Unhealthy500(t *testing.T) {
	store := state.NewStore()
	store.AddOrUpdate(state.Service{
		Name: "svc1", Namespace: "ns1", URL: "https://svc1.example.com",
		Status: state.StatusUnknown,
	})

	client := &mockHTTPProber{
		responses: map[string]mockResponse{
			"https://svc1.example.com": {statusCode: 500, body: "Internal Server Error"},
		},
	}

	checker := NewChecker(store, store, client, time.Hour, nil)
	svc, _ := store.Get("ns1", "svc1")
	result := checker.applyResult(svc, checker.probeService(context.Background(), svc.URL))

	if result.Status != state.StatusUnhealthy {
		t.Errorf("expected status %q, got %q", state.StatusUnhealthy, result.Status)
	}
	if result.HTTPCode == nil || *result.HTTPCode != 500 {
		t.Errorf("expected HTTPCode 500, got %v", result.HTTPCode)
	}
	if result.ErrorSnippet == nil {
		t.Fatal("expected non-nil ErrorSnippet for unhealthy service")
	}
	if *result.ErrorSnippet != "Internal Server Error" {
		t.Errorf("expected ErrorSnippet %q, got %q", "Internal Server Error", *result.ErrorSnippet)
	}
}

func TestCheckService_ConnectionError(t *testing.T) {
	store := state.NewStore()
	store.AddOrUpdate(state.Service{
		Name: "svc1", Namespace: "ns1", URL: "https://svc1.example.com",
		Status: state.StatusUnknown,
	})

	client := &mockHTTPProber{
		responses: map[string]mockResponse{
			"https://svc1.example.com": {err: errors.New("dial tcp: connection refused")},
		},
	}

	checker := NewChecker(store, store, client, time.Hour, nil)
	svc, _ := store.Get("ns1", "svc1")
	result := checker.applyResult(svc, checker.probeService(context.Background(), svc.URL))

	if result.Status != state.StatusUnhealthy {
		t.Errorf("expected status %q, got %q", state.StatusUnhealthy, result.Status)
	}
	if result.HTTPCode != nil {
		t.Errorf("expected nil HTTPCode for connection error, got %d", *result.HTTPCode)
	}
	if result.ErrorSnippet == nil {
		t.Fatal("expected non-nil ErrorSnippet for connection error")
	}
	if !strings.Contains(*result.ErrorSnippet, "connection refused") {
		t.Errorf("expected ErrorSnippet to contain 'connection refused', got %q", *result.ErrorSnippet)
	}
}

func TestCheckService_StateTransitionUpdatesLastStateChange(t *testing.T) {
	store := state.NewStore()
	oldTime := time.Now().Add(-time.Hour)
	store.AddOrUpdate(state.Service{
		Name: "svc1", Namespace: "ns1", URL: "https://svc1.example.com",
		Status:          state.StatusHealthy,
		LastStateChange: &oldTime,
	})

	client := &mockHTTPProber{
		responses: map[string]mockResponse{
			"https://svc1.example.com": {statusCode: 500, body: "error"},
		},
	}

	checker := NewChecker(store, store, client, time.Hour, nil)
	svc, _ := store.Get("ns1", "svc1")
	result := checker.applyResult(svc, checker.probeService(context.Background(), svc.URL))

	if result.Status != state.StatusUnhealthy {
		t.Errorf("expected status %q, got %q", state.StatusUnhealthy, result.Status)
	}
	if result.LastStateChange == nil {
		t.Fatal("expected LastStateChange to be set")
	}
	if !result.LastStateChange.After(oldTime) {
		t.Error("expected LastStateChange to be updated to a newer time")
	}
}

func TestCheckService_SameStatePreservesLastStateChange(t *testing.T) {
	store := state.NewStore()
	oldTime := time.Now().Add(-time.Hour)
	store.AddOrUpdate(state.Service{
		Name: "svc1", Namespace: "ns1", URL: "https://svc1.example.com",
		Status:          state.StatusHealthy,
		LastStateChange: &oldTime,
	})

	client := &mockHTTPProber{
		responses: map[string]mockResponse{
			"https://svc1.example.com": {statusCode: 200, body: "OK"},
		},
	}

	checker := NewChecker(store, store, client, time.Hour, nil)
	svc, _ := store.Get("ns1", "svc1")
	result := checker.applyResult(svc, checker.probeService(context.Background(), svc.URL))

	if result.Status != state.StatusHealthy {
		t.Errorf("expected status %q, got %q", state.StatusHealthy, result.Status)
	}
	if result.LastStateChange == nil {
		t.Fatal("expected LastStateChange to be preserved")
	}
	if !result.LastStateChange.Equal(oldTime) {
		t.Errorf("expected LastStateChange to be preserved as %v, got %v", oldTime, *result.LastStateChange)
	}
}

func TestCheckService_LastCheckedAlwaysUpdated(t *testing.T) {
	store := state.NewStore()
	store.AddOrUpdate(state.Service{
		Name: "svc1", Namespace: "ns1", URL: "https://svc1.example.com",
		Status: state.StatusUnknown,
	})

	client := &mockHTTPProber{
		responses: map[string]mockResponse{
			"https://svc1.example.com": {statusCode: 200, body: "OK"},
		},
	}

	before := time.Now()
	checker := NewChecker(store, store, client, time.Hour, nil)
	svc, _ := store.Get("ns1", "svc1")
	result := checker.applyResult(svc, checker.probeService(context.Background(), svc.URL))

	if result.LastChecked == nil {
		t.Fatal("expected LastChecked to be set")
	}
	if result.LastChecked.Before(before) {
		t.Error("expected LastChecked to be at or after test start time")
	}
}

func TestCheckService_ConcurrentChecks(t *testing.T) {
	store := state.NewStore()
	store.AddOrUpdate(state.Service{Name: "svc1", Namespace: "ns1", URL: "https://svc1.example.com", Status: state.StatusUnknown})
	store.AddOrUpdate(state.Service{Name: "svc2", Namespace: "ns1", URL: "https://svc2.example.com", Status: state.StatusUnknown})
	store.AddOrUpdate(state.Service{Name: "svc3", Namespace: "ns1", URL: "https://svc3.example.com", Status: state.StatusUnknown})

	client := &mockHTTPProber{
		responses: map[string]mockResponse{
			"https://svc1.example.com": {statusCode: 200, body: "OK"},
			"https://svc2.example.com": {statusCode: 401, body: "Unauthorized"},
			"https://svc3.example.com": {statusCode: 500, body: "Error"},
		},
	}

	checker := NewChecker(store, store, client, time.Hour, nil)
	checker.checkAll(context.Background())

	svc1, _ := store.Get("ns1", "svc1")
	svc2, _ := store.Get("ns1", "svc2")
	svc3, _ := store.Get("ns1", "svc3")

	if svc1.Status != state.StatusHealthy {
		t.Errorf("svc1: expected %q, got %q", state.StatusHealthy, svc1.Status)
	}
	if svc2.Status != state.StatusAuthBlocked {
		t.Errorf("svc2: expected %q, got %q", state.StatusAuthBlocked, svc2.Status)
	}
	if svc3.Status != state.StatusUnhealthy {
		t.Errorf("svc3: expected %q, got %q", state.StatusUnhealthy, svc3.Status)
	}
}

func TestRun_ContextCancellation(t *testing.T) {
	store := state.NewStore()
	client := &mockHTTPProber{responses: map[string]mockResponse{}}

	checker := NewChecker(store, store, client, time.Hour, nil)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		checker.Run(ctx)
		close(done)
	}()

	// Give it a moment to start
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// OK — Run returned promptly
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancellation")
	}
}

func TestCheckService_ErrorSnippetTruncation(t *testing.T) {
	store := state.NewStore()
	store.AddOrUpdate(state.Service{
		Name: "svc1", Namespace: "ns1", URL: "https://svc1.example.com",
		Status: state.StatusUnknown,
	})

	longBody := strings.Repeat("x", 1000)
	client := &mockHTTPProber{
		responses: map[string]mockResponse{
			"https://svc1.example.com": {statusCode: 500, body: longBody},
		},
	}

	checker := NewChecker(store, store, client, time.Hour, nil)
	svc, _ := store.Get("ns1", "svc1")
	result := checker.applyResult(svc, checker.probeService(context.Background(), svc.URL))

	if result.ErrorSnippet == nil {
		t.Fatal("expected non-nil ErrorSnippet")
	}
	if len(*result.ErrorSnippet) > 256 {
		t.Errorf("expected ErrorSnippet <= 256 chars, got %d", len(*result.ErrorSnippet))
	}
}

func TestCheckService_ErrorSnippetFirstLine(t *testing.T) {
	store := state.NewStore()
	store.AddOrUpdate(state.Service{
		Name: "svc1", Namespace: "ns1", URL: "https://svc1.example.com",
		Status: state.StatusUnknown,
	})

	multiLineBody := "First line of error\nSecond line\nThird line"
	client := &mockHTTPProber{
		responses: map[string]mockResponse{
			"https://svc1.example.com": {statusCode: 500, body: multiLineBody},
		},
	}

	checker := NewChecker(store, store, client, time.Hour, nil)
	svc, _ := store.Get("ns1", "svc1")
	result := checker.applyResult(svc, checker.probeService(context.Background(), svc.URL))

	if result.ErrorSnippet == nil {
		t.Fatal("expected non-nil ErrorSnippet")
	}
	if *result.ErrorSnippet != "First line of error" {
		t.Errorf("expected ErrorSnippet %q, got %q", "First line of error", *result.ErrorSnippet)
	}
}

func TestCheckService_PreservesNonHealthFields(t *testing.T) {
	store := state.NewStore()
	store.AddOrUpdate(state.Service{
		Name:        "svc1",
		DisplayName: "My Service",
		Namespace:   "ns1",
		URL:         "https://svc1.example.com",
		Status:      state.StatusUnknown,
	})

	client := &mockHTTPProber{
		responses: map[string]mockResponse{
			"https://svc1.example.com": {statusCode: 200, body: "OK"},
		},
	}

	checker := NewChecker(store, store, client, time.Hour, nil)
	svc, _ := store.Get("ns1", "svc1")
	result := checker.applyResult(svc, checker.probeService(context.Background(), svc.URL))

	if result.Name != "svc1" {
		t.Errorf("expected Name %q, got %q", "svc1", result.Name)
	}
	if result.DisplayName != "My Service" {
		t.Errorf("expected DisplayName %q, got %q", "My Service", result.DisplayName)
	}
	if result.Namespace != "ns1" {
		t.Errorf("expected Namespace %q, got %q", "ns1", result.Namespace)
	}
	if result.URL != "https://svc1.example.com" {
		t.Errorf("expected URL %q, got %q", "https://svc1.example.com", result.URL)
	}
}

func TestRun_ImmediateCheckOnStart(t *testing.T) {
	store := state.NewStore()
	store.AddOrUpdate(state.Service{
		Name: "svc1", Namespace: "ns1", URL: "https://svc1.example.com",
		Status: state.StatusUnknown,
	})

	var checkCount atomic.Int32
	client := &mockHTTPProber{
		responses: map[string]mockResponse{
			"https://svc1.example.com": {statusCode: 200, body: "OK"},
		},
	}

	checker := NewChecker(store, store, client, time.Hour, nil)

	// Subscribe to store events to count updates
	sub := store.Subscribe()
	defer store.Unsubscribe(sub)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		for range sub {
			checkCount.Add(1)
		}
	}()

	go checker.Run(ctx)

	// Wait for the immediate check to complete
	time.Sleep(200 * time.Millisecond)
	cancel()

	if checkCount.Load() < 1 {
		t.Error("expected at least one health check on immediate start")
	}

	svc, _ := store.Get("ns1", "svc1")
	if svc.Status != state.StatusHealthy {
		t.Errorf("expected status %q after immediate check, got %q", state.StatusHealthy, svc.Status)
	}
}

func TestCheckService_AuthBlockedNoErrorSnippet(t *testing.T) {
	store := state.NewStore()
	store.AddOrUpdate(state.Service{
		Name: "svc1", Namespace: "ns1", URL: "https://svc1.example.com",
		Status: state.StatusUnknown,
	})

	client := &mockHTTPProber{
		responses: map[string]mockResponse{
			"https://svc1.example.com": {statusCode: 401, body: "Unauthorized"},
		},
	}

	checker := NewChecker(store, store, client, time.Hour, nil)
	svc, _ := store.Get("ns1", "svc1")
	result := checker.applyResult(svc, checker.probeService(context.Background(), svc.URL))

	if result.ErrorSnippet != nil {
		t.Errorf("expected nil ErrorSnippet for authBlocked service, got %q", *result.ErrorSnippet)
	}
}

func TestCheckService_HealthyNoErrorSnippet(t *testing.T) {
	store := state.NewStore()
	store.AddOrUpdate(state.Service{
		Name: "svc1", Namespace: "ns1", URL: "https://svc1.example.com",
		Status: state.StatusUnknown,
	})

	client := &mockHTTPProber{
		responses: map[string]mockResponse{
			"https://svc1.example.com": {statusCode: 200, body: "OK"},
		},
	}

	checker := NewChecker(store, store, client, time.Hour, nil)
	svc, _ := store.Get("ns1", "svc1")
	result := checker.applyResult(svc, checker.probeService(context.Background(), svc.URL))

	if result.ErrorSnippet != nil {
		t.Errorf("expected nil ErrorSnippet for healthy service, got %q", *result.ErrorSnippet)
	}
}

func TestCheckAll_ServiceRemovedDuringCheck(t *testing.T) {
	store := state.NewStore()
	store.AddOrUpdate(state.Service{
		Name: "svc1", Namespace: "ns1", URL: "https://svc1.example.com",
		Status: state.StatusUnknown,
	})

	client := &mockHTTPProber{
		responses: map[string]mockResponse{
			"https://svc1.example.com": {statusCode: 200, body: "OK"},
		},
	}

	checker := NewChecker(store, store, client, time.Hour, nil)

	// Remove the service before checkAll completes
	store.Remove("ns1", "svc1")

	// Execute checkAll which uses the Read-Modify-Write pattern
	checker.checkAll(context.Background())

	// Service should NOT be re-added to the store
	_, ok := store.Get("ns1", "svc1")
	if ok {
		t.Fatal("expected service NOT to be in store (zombie bug)")
	}
}

func TestCheckService_200Range(t *testing.T) {
	tests := []struct {
		code int
		want state.HealthStatus
	}{
		{200, state.StatusHealthy},
		{201, state.StatusHealthy},
		{204, state.StatusHealthy},
		{299, state.StatusHealthy},
		{300, state.StatusUnhealthy},
		{404, state.StatusUnhealthy},
	}

	for _, tt := range tests {
		t.Run(http.StatusText(tt.code), func(t *testing.T) {
			store := state.NewStore()
			store.AddOrUpdate(state.Service{
				Name: "svc1", Namespace: "ns1", URL: "https://svc1.example.com",
				Status: state.StatusUnknown,
			})

			client := &mockHTTPProber{
				responses: map[string]mockResponse{
					"https://svc1.example.com": {statusCode: tt.code, body: "body"},
				},
			}

			checker := NewChecker(store, store, client, time.Hour, nil)
			svc, _ := store.Get("ns1", "svc1")
			result := checker.applyResult(svc, checker.probeService(context.Background(), svc.URL))

			if result.Status != tt.want {
				t.Errorf("code %d: expected status %q, got %q", tt.code, tt.want, result.Status)
			}
		})
	}
}
