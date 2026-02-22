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

	"github.com/rathix/command-center/internal/history"
	"github.com/rathix/command-center/internal/state"
)

// capturedRequest records a request for later assertion.
type capturedRequest struct {
	url    string
	header http.Header
}

// mockHTTPProber is a configurable mock for HTTPProber.
type mockHTTPProber struct {
	mu               sync.Mutex
	responses        map[string]mockResponse // keyed by URL
	capturedRequests []capturedRequest
}

type mockResponse struct {
	statusCode int
	body       string
	err        error
}

func (m *mockHTTPProber) Do(req *http.Request) (*http.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.capturedRequests = append(m.capturedRequests, capturedRequest{
		url:    req.URL.String(),
		header: req.Header.Clone(),
	})
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

func (m *mockHTTPProber) getCapturedRequests() []capturedRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]capturedRequest, len(m.capturedRequests))
	copy(cp, m.capturedRequests)
	return cp
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

	checker := NewChecker(store, store, client, time.Hour, history.NoopWriter{}, nil)
	svc, _ := store.Get("ns1", "svc1")
	checker.applyResult(&svc, checker.probeService(context.Background(), svc.URL))

	if svc.Status != state.StatusHealthy {
		t.Errorf("expected status %q, got %q", state.StatusHealthy, svc.Status)
	}
	if svc.HTTPCode == nil || *svc.HTTPCode != 200 {
		t.Errorf("expected HTTPCode 200, got %v", svc.HTTPCode)
	}
	if svc.ResponseTimeMs == nil || *svc.ResponseTimeMs < 0 {
		t.Errorf("expected non-negative ResponseTimeMs, got %v", svc.ResponseTimeMs)
	}
	if svc.ErrorSnippet != nil {
		t.Errorf("expected nil ErrorSnippet for healthy service, got %q", *svc.ErrorSnippet)
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

	checker := NewChecker(store, store, client, time.Hour, history.NoopWriter{}, nil)
	svc, _ := store.Get("ns1", "svc1")
	checker.applyResult(&svc, checker.probeService(context.Background(), svc.URL))

	if svc.Status != state.StatusAuthBlocked {
		t.Errorf("expected status %q, got %q", state.StatusAuthBlocked, svc.Status)
	}
	if svc.HTTPCode == nil || *svc.HTTPCode != 401 {
		t.Errorf("expected HTTPCode 401, got %v", svc.HTTPCode)
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

	checker := NewChecker(store, store, client, time.Hour, history.NoopWriter{}, nil)
	svc, _ := store.Get("ns1", "svc1")
	checker.applyResult(&svc, checker.probeService(context.Background(), svc.URL))

	if svc.Status != state.StatusAuthBlocked {
		t.Errorf("expected status %q, got %q", state.StatusAuthBlocked, svc.Status)
	}
	if svc.HTTPCode == nil || *svc.HTTPCode != 403 {
		t.Errorf("expected HTTPCode 403, got %v", svc.HTTPCode)
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

	checker := NewChecker(store, store, client, time.Hour, history.NoopWriter{}, nil)
	svc, _ := store.Get("ns1", "svc1")
	checker.applyResult(&svc, checker.probeService(context.Background(), svc.URL))

	if svc.Status != state.StatusUnhealthy {
		t.Errorf("expected status %q, got %q", state.StatusUnhealthy, svc.Status)
	}
	if svc.HTTPCode == nil || *svc.HTTPCode != 500 {
		t.Errorf("expected HTTPCode 500, got %v", svc.HTTPCode)
	}
	if svc.ErrorSnippet == nil {
		t.Fatal("expected non-nil ErrorSnippet for unhealthy service")
	}
	if *svc.ErrorSnippet != "Internal Server Error" {
		t.Errorf("expected ErrorSnippet %q, got %q", "Internal Server Error", *svc.ErrorSnippet)
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

	checker := NewChecker(store, store, client, time.Hour, history.NoopWriter{}, nil)
	svc, _ := store.Get("ns1", "svc1")
	checker.applyResult(&svc, checker.probeService(context.Background(), svc.URL))

	if svc.Status != state.StatusUnhealthy {
		t.Errorf("expected status %q, got %q", state.StatusUnhealthy, svc.Status)
	}
	if svc.HTTPCode != nil {
		t.Errorf("expected nil HTTPCode for connection error, got %d", *svc.HTTPCode)
	}
	if svc.ErrorSnippet == nil {
		t.Fatal("expected non-nil ErrorSnippet for connection error")
	}
	if !strings.Contains(*svc.ErrorSnippet, "connection refused") {
		t.Errorf("expected ErrorSnippet to contain 'connection refused', got %q", *svc.ErrorSnippet)
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

	checker := NewChecker(store, store, client, time.Hour, history.NoopWriter{}, nil)
	svc, _ := store.Get("ns1", "svc1")
	checker.applyResult(&svc, checker.probeService(context.Background(), svc.URL))

	if svc.Status != state.StatusUnhealthy {
		t.Errorf("expected status %q, got %q", state.StatusUnhealthy, svc.Status)
	}
	if svc.LastStateChange == nil {
		t.Fatal("expected LastStateChange to be set")
	}
	if !svc.LastStateChange.After(oldTime) {
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

	checker := NewChecker(store, store, client, time.Hour, history.NoopWriter{}, nil)
	svc, _ := store.Get("ns1", "svc1")
	checker.applyResult(&svc, checker.probeService(context.Background(), svc.URL))

	if svc.Status != state.StatusHealthy {
		t.Errorf("expected status %q, got %q", state.StatusHealthy, svc.Status)
	}
	if svc.LastStateChange == nil {
		t.Fatal("expected LastStateChange to be preserved")
	}
	if !svc.LastStateChange.Equal(oldTime) {
		t.Errorf("expected LastStateChange to be preserved as %v, got %v", oldTime, *svc.LastStateChange)
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
	checker := NewChecker(store, store, client, time.Hour, history.NoopWriter{}, nil)
	svc, _ := store.Get("ns1", "svc1")
	checker.applyResult(&svc, checker.probeService(context.Background(), svc.URL))

	if svc.LastChecked == nil {
		t.Fatal("expected LastChecked to be set")
	}
	if svc.LastChecked.Before(before) {
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

	checker := NewChecker(store, store, client, time.Hour, history.NoopWriter{}, nil)
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

	checker := NewChecker(store, store, client, time.Hour, history.NoopWriter{}, nil)

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

	checker := NewChecker(store, store, client, time.Hour, history.NoopWriter{}, nil)
	svc, _ := store.Get("ns1", "svc1")
	checker.applyResult(&svc, checker.probeService(context.Background(), svc.URL))

	if svc.ErrorSnippet == nil {
		t.Fatal("expected non-nil ErrorSnippet")
	}
	if len(*svc.ErrorSnippet) > 256 {
		t.Errorf("expected ErrorSnippet <= 256 chars, got %d", len(*svc.ErrorSnippet))
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

	checker := NewChecker(store, store, client, time.Hour, history.NoopWriter{}, nil)
	svc, _ := store.Get("ns1", "svc1")
	checker.applyResult(&svc, checker.probeService(context.Background(), svc.URL))

	if svc.ErrorSnippet == nil {
		t.Fatal("expected non-nil ErrorSnippet")
	}
	if *svc.ErrorSnippet != "First line of error" {
		t.Errorf("expected ErrorSnippet %q, got %q", "First line of error", *svc.ErrorSnippet)
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

	checker := NewChecker(store, store, client, time.Hour, history.NoopWriter{}, nil)
	svc, _ := store.Get("ns1", "svc1")
	checker.applyResult(&svc, checker.probeService(context.Background(), svc.URL))

	if svc.Name != "svc1" {
		t.Errorf("expected Name %q, got %q", "svc1", svc.Name)
	}
	if svc.DisplayName != "My Service" {
		t.Errorf("expected DisplayName %q, got %q", "My Service", svc.DisplayName)
	}
	if svc.Namespace != "ns1" {
		t.Errorf("expected Namespace %q, got %q", "ns1", svc.Namespace)
	}
	if svc.URL != "https://svc1.example.com" {
		t.Errorf("expected URL %q, got %q", "https://svc1.example.com", svc.URL)
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

	checker := NewChecker(store, store, client, time.Hour, history.NoopWriter{}, nil)

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

	checker := NewChecker(store, store, client, time.Hour, history.NoopWriter{}, nil)
	svc, _ := store.Get("ns1", "svc1")
	checker.applyResult(&svc, checker.probeService(context.Background(), svc.URL))

	if svc.ErrorSnippet != nil {
		t.Errorf("expected nil ErrorSnippet for authBlocked service, got %q", *svc.ErrorSnippet)
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

	checker := NewChecker(store, store, client, time.Hour, history.NoopWriter{}, nil)
	svc, _ := store.Get("ns1", "svc1")
	checker.applyResult(&svc, checker.probeService(context.Background(), svc.URL))

	if svc.ErrorSnippet != nil {
		t.Errorf("expected nil ErrorSnippet for healthy service, got %q", *svc.ErrorSnippet)
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

	checker := NewChecker(store, store, client, time.Hour, history.NoopWriter{}, nil)

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

			checker := NewChecker(store, store, client, time.Hour, history.NoopWriter{}, nil)
			svc, _ := store.Get("ns1", "svc1")
			checker.applyResult(&svc, checker.probeService(context.Background(), svc.URL))

			if svc.Status != tt.want {
				t.Errorf("code %d: expected status %q, got %q", tt.code, tt.want, svc.Status)
			}
		})
	}
}

func TestCheckAll_HealthURLOverride(t *testing.T) {
	store := state.NewStore()
	store.AddOrUpdate(state.Service{
		Name:      "truenas",
		Namespace: "custom",
		URL:       "https://truenas.local",
		HealthURL: "https://truenas.local/api/v2.0/system/state",
		Status:    state.StatusUnknown,
	})

	client := &mockHTTPProber{
		responses: map[string]mockResponse{
			// Only the healthUrl responds, NOT the base URL
			"https://truenas.local/api/v2.0/system/state": {statusCode: 200, body: "OK"},
		},
	}

	checker := NewChecker(store, store, client, time.Hour, history.NoopWriter{}, nil)
	checker.checkAll(context.Background())

	svc, _ := store.Get("custom", "truenas")
	if svc.Status != state.StatusHealthy {
		t.Errorf("expected %q (probed healthUrl), got %q", state.StatusHealthy, svc.Status)
	}
}

func TestCheckAll_ExpectedStatusCodes401Healthy(t *testing.T) {
	store := state.NewStore()
	store.AddOrUpdate(state.Service{
		Name:                "authsvc",
		Namespace:           "custom",
		URL:                 "https://auth.local",
		ExpectedStatusCodes: []int{200, 401},
		Status:              state.StatusUnknown,
	})

	client := &mockHTTPProber{
		responses: map[string]mockResponse{
			"https://auth.local": {statusCode: 401, body: "Unauthorized"},
		},
	}

	checker := NewChecker(store, store, client, time.Hour, history.NoopWriter{}, nil)
	checker.checkAll(context.Background())

	svc, _ := store.Get("custom", "authsvc")
	if svc.Status != state.StatusHealthy {
		t.Errorf("expected %q (401 in expected codes), got %q", state.StatusHealthy, svc.Status)
	}
}

func TestCheckAll_ExpectedStatusCodesNotInList(t *testing.T) {
	store := state.NewStore()
	store.AddOrUpdate(state.Service{
		Name:                "svc",
		Namespace:           "custom",
		URL:                 "https://svc.local",
		ExpectedStatusCodes: []int{200},
		Status:              state.StatusUnknown,
	})

	client := &mockHTTPProber{
		responses: map[string]mockResponse{
			"https://svc.local": {statusCode: 401, body: "Unauthorized"},
		},
	}

	checker := NewChecker(store, store, client, time.Hour, history.NoopWriter{}, nil)
	checker.checkAll(context.Background())

	svc, _ := store.Get("custom", "svc")
	if svc.Status != state.StatusAuthBlocked {
		t.Errorf("expected %q (401 not in expected [200]), got %q", state.StatusAuthBlocked, svc.Status)
	}
}

func TestCheckAll_HealthURLDifferentHost(t *testing.T) {
	store := state.NewStore()
	store.AddOrUpdate(state.Service{
		Name:      "svc",
		Namespace: "custom",
		URL:       "https://site.local",
		HealthURL: "https://monitor.other.local/status",
		Status:    state.StatusUnknown,
	})

	client := &mockHTTPProber{
		responses: map[string]mockResponse{
			// Only the healthUrl responds — base URL would fail
			"https://monitor.other.local/status": {statusCode: 200, body: "OK"},
		},
	}

	checker := NewChecker(store, store, client, time.Hour, history.NoopWriter{}, nil)
	checker.checkAll(context.Background())

	svc, _ := store.Get("custom", "svc")
	if svc.Status != state.StatusHealthy {
		t.Errorf("expected %q (probed healthUrl on different host), got %q", state.StatusHealthy, svc.Status)
	}
}

// mockHistoryWriter captures Record calls for testing.
type mockHistoryWriter struct {
	mu      sync.Mutex
	records []history.TransitionRecord
	err     error // if non-nil, Record returns this error
}

func (m *mockHistoryWriter) Record(rec history.TransitionRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records = append(m.records, rec)
	return m.err
}

func (m *mockHistoryWriter) Close() error { return nil }

func (m *mockHistoryWriter) getRecords() []history.TransitionRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]history.TransitionRecord, len(m.records))
	copy(cp, m.records)
	return cp
}

func TestCheckAll_TransitionRecordsHistory(t *testing.T) {
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

	hw := &mockHistoryWriter{}
	checker := NewChecker(store, store, client, time.Hour, hw, nil)
	checker.checkAll(context.Background())

	recs := hw.getRecords()
	if len(recs) != 1 {
		t.Fatalf("expected 1 history record, got %d", len(recs))
	}
	rec := recs[0]
	if rec.ServiceKey != "ns1/svc1" {
		t.Errorf("expected ServiceKey %q, got %q", "ns1/svc1", rec.ServiceKey)
	}
	if rec.PrevStatus != state.StatusUnknown {
		t.Errorf("expected PrevStatus %q, got %q", state.StatusUnknown, rec.PrevStatus)
	}
	if rec.NextStatus != state.StatusHealthy {
		t.Errorf("expected NextStatus %q, got %q", state.StatusHealthy, rec.NextStatus)
	}
	if rec.HTTPCode == nil || *rec.HTTPCode != 200 {
		t.Errorf("expected HTTPCode 200, got %v", rec.HTTPCode)
	}
	if rec.ResponseMs == nil || *rec.ResponseMs < 0 {
		t.Errorf("expected non-negative ResponseMs, got %v", rec.ResponseMs)
	}
}

func TestCheckAll_NoTransitionNoHistory(t *testing.T) {
	store := state.NewStore()
	store.AddOrUpdate(state.Service{
		Name: "svc1", Namespace: "ns1", URL: "https://svc1.example.com",
		Status: state.StatusHealthy,
	})

	client := &mockHTTPProber{
		responses: map[string]mockResponse{
			"https://svc1.example.com": {statusCode: 200, body: "OK"},
		},
	}

	hw := &mockHistoryWriter{}
	checker := NewChecker(store, store, client, time.Hour, hw, nil)
	checker.checkAll(context.Background())

	recs := hw.getRecords()
	if len(recs) != 0 {
		t.Errorf("expected 0 history records when status unchanged, got %d", len(recs))
	}
}

func TestCheckAll_HistoryWriteErrorDoesNotBlockUpdates(t *testing.T) {
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

	hw := &mockHistoryWriter{err: errors.New("disk full")}
	checker := NewChecker(store, store, client, time.Hour, hw, nil)
	checker.checkAll(context.Background())

	// Status should still be updated despite history write failure
	svc, _ := store.Get("ns1", "svc1")
	if svc.Status != state.StatusHealthy {
		t.Errorf("expected status %q despite history error, got %q", state.StatusHealthy, svc.Status)
	}
}

func TestNewChecker_NilHistoryWriterDefaultsToNoop(t *testing.T) {
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

	checker := NewChecker(store, store, client, time.Hour, nil, nil)
	checker.checkAll(context.Background())

	svc, _ := store.Get("ns1", "svc1")
	if svc.Status != state.StatusHealthy {
		t.Errorf("expected status %q with nil history writer, got %q", state.StatusHealthy, svc.Status)
	}
}

// --- OIDC mock types ---

// mockTokenProvider is a configurable mock for TokenProvider.
type mockTokenProvider struct {
	token string
	err   error
	calls atomic.Int32
}

func (m *mockTokenProvider) GetToken(_ context.Context) (string, error) {
	m.calls.Add(1)
	return m.token, m.err
}

// mockEndpointDiscoverer is a configurable mock for EndpointDiscoverer.
type mockEndpointDiscoverer struct {
	mu          sync.Mutex
	strategies  map[string]*EndpointStrategy
	discovered  map[string]*EndpointStrategy // keyed by serviceKey
	discoverErr error
	cleared     []string
}

func (m *mockEndpointDiscoverer) GetStrategy(key string) *EndpointStrategy {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.strategies[key]
}

func (m *mockEndpointDiscoverer) Discover(_ context.Context, serviceKey, _ string) (*EndpointStrategy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.discoverErr != nil {
		return nil, m.discoverErr
	}
	return m.discovered[serviceKey], nil
}

func (m *mockEndpointDiscoverer) ClearStrategy(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.strategies, key)
	m.cleared = append(m.cleared, key)
}

func (m *mockEndpointDiscoverer) getCleared() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]string, len(m.cleared))
	copy(cp, m.cleared)
	return cp
}

// --- Task 7: MVP parity tests ---

func TestCheckAll_401NilTokenProviderStaysAuthBlocked(t *testing.T) {
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

	checker := NewChecker(store, store, client, time.Hour, history.NoopWriter{}, nil)
	// No WithTokenProvider — nil tokenProvider = MVP behavior
	checker.checkAll(context.Background())

	svc, _ := store.Get("ns1", "svc1")
	if svc.Status != state.StatusAuthBlocked {
		t.Errorf("expected %q with nil tokenProvider, got %q", state.StatusAuthBlocked, svc.Status)
	}
}

func TestCheckAll_403NilTokenProviderStaysAuthBlocked(t *testing.T) {
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

	checker := NewChecker(store, store, client, time.Hour, history.NoopWriter{}, nil)
	checker.checkAll(context.Background())

	svc, _ := store.Get("ns1", "svc1")
	if svc.Status != state.StatusAuthBlocked {
		t.Errorf("expected %q with nil tokenProvider, got %q", state.StatusAuthBlocked, svc.Status)
	}
}

// --- Task 8: OIDC retry happy path tests ---

func TestCheckAll_OIDCRetry401BecomesHealthy(t *testing.T) {
	store := state.NewStore()
	store.AddOrUpdate(state.Service{
		Name: "svc1", Namespace: "ns1", URL: "https://svc1.example.com",
		Status: state.StatusUnknown,
	})

	tp := &mockTokenProvider{token: "test-token-123"}
	disc := &mockEndpointDiscoverer{
		strategies: make(map[string]*EndpointStrategy),
		discovered: map[string]*EndpointStrategy{
			"ns1/svc1": {Type: "oidcAuth"},
		},
	}

	authClient := &authAwareMockProber{
		unauthResponses: map[string]mockResponse{
			"https://svc1.example.com": {statusCode: 401, body: "Unauthorized"},
		},
		authResponses: map[string]mockResponse{
			"https://svc1.example.com": {statusCode: 200, body: "OK"},
		},
	}

	checker := NewChecker(store, store, authClient, time.Hour, history.NoopWriter{}, nil)
	checker.WithTokenProvider(tp).WithDiscoverer(disc)
	checker.checkAll(context.Background())

	svc, _ := store.Get("ns1", "svc1")
	if svc.Status != state.StatusHealthy {
		t.Errorf("expected %q after OIDC retry, got %q", state.StatusHealthy, svc.Status)
	}
	if tp.calls.Load() != 1 {
		t.Errorf("expected token provider called once, got %d", tp.calls.Load())
	}
}

func TestCheckAll_OIDCRetry403BecomesHealthy(t *testing.T) {
	store := state.NewStore()
	store.AddOrUpdate(state.Service{
		Name: "svc1", Namespace: "ns1", URL: "https://svc1.example.com",
		Status: state.StatusUnknown,
	})

	authClient := &authAwareMockProber{
		unauthResponses: map[string]mockResponse{
			"https://svc1.example.com": {statusCode: 403, body: "Forbidden"},
		},
		authResponses: map[string]mockResponse{
			"https://svc1.example.com": {statusCode: 200, body: "OK"},
		},
	}

	tp := &mockTokenProvider{token: "test-token-456"}
	disc := &mockEndpointDiscoverer{
		strategies: make(map[string]*EndpointStrategy),
		discovered: map[string]*EndpointStrategy{
			"ns1/svc1": {Type: "oidcAuth"},
		},
	}

	checker := NewChecker(store, store, authClient, time.Hour, history.NoopWriter{}, nil)
	checker.WithTokenProvider(tp).WithDiscoverer(disc)
	checker.checkAll(context.Background())

	svc, _ := store.Get("ns1", "svc1")
	if svc.Status != state.StatusHealthy {
		t.Errorf("expected %q after OIDC retry for 403, got %q", state.StatusHealthy, svc.Status)
	}
}

func TestCheckAll_OIDCRetryBearerHeaderSent(t *testing.T) {
	store := state.NewStore()
	store.AddOrUpdate(state.Service{
		Name: "svc1", Namespace: "ns1", URL: "https://svc1.example.com",
		Status: state.StatusUnknown,
	})

	authClient := &authAwareMockProber{
		unauthResponses: map[string]mockResponse{
			"https://svc1.example.com": {statusCode: 401, body: "Unauthorized"},
		},
		authResponses: map[string]mockResponse{
			"https://svc1.example.com": {statusCode: 200, body: "OK"},
		},
	}

	tp := &mockTokenProvider{token: "my-secret-token"}
	disc := &mockEndpointDiscoverer{
		strategies: make(map[string]*EndpointStrategy),
		discovered: map[string]*EndpointStrategy{
			"ns1/svc1": {Type: "oidcAuth"},
		},
	}

	checker := NewChecker(store, store, authClient, time.Hour, history.NoopWriter{}, nil)
	checker.WithTokenProvider(tp).WithDiscoverer(disc)
	checker.checkAll(context.Background())

	reqs := authClient.getCapturedRequests()
	foundAuth := false
	for _, r := range reqs {
		if r.header.Get("Authorization") == "Bearer my-secret-token" {
			foundAuth = true
			break
		}
	}
	if !foundAuth {
		t.Error("expected Authorization: Bearer header in retry request")
	}
}

func TestCheckAll_200InitialNoOIDCInteraction(t *testing.T) {
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

	tp := &mockTokenProvider{token: "should-not-be-used"}

	checker := NewChecker(store, store, client, time.Hour, history.NoopWriter{}, nil)
	checker.WithTokenProvider(tp)
	checker.checkAll(context.Background())

	svc, _ := store.Get("ns1", "svc1")
	if svc.Status != state.StatusHealthy {
		t.Errorf("expected %q, got %q", state.StatusHealthy, svc.Status)
	}
	if tp.calls.Load() != 0 {
		t.Errorf("expected token provider NOT called for healthy service, got %d calls", tp.calls.Load())
	}
}

// --- Task 9: Health endpoint discovery tests ---

func TestCheckAll_DiscoveryFindsHealthEndpointNoTokenNeeded(t *testing.T) {
	store := state.NewStore()
	store.AddOrUpdate(state.Service{
		Name: "svc1", Namespace: "ns1", URL: "https://svc1.example.com",
		Status: state.StatusUnknown,
	})

	client := &mockHTTPProber{
		responses: map[string]mockResponse{
			"https://svc1.example.com":         {statusCode: 401, body: "Unauthorized"},
			"https://svc1.example.com/healthz": {statusCode: 200, body: "OK"},
		},
	}

	tp := &mockTokenProvider{token: "should-not-be-used"}
	disc := &mockEndpointDiscoverer{
		strategies: make(map[string]*EndpointStrategy),
		discovered: map[string]*EndpointStrategy{
			"ns1/svc1": {Type: "healthEndpoint", Endpoint: "https://svc1.example.com/healthz"},
		},
	}

	checker := NewChecker(store, store, client, time.Hour, history.NoopWriter{}, nil)
	checker.WithTokenProvider(tp).WithDiscoverer(disc)
	checker.checkAll(context.Background())

	svc, _ := store.Get("ns1", "svc1")
	if svc.Status != state.StatusHealthy {
		t.Errorf("expected %q via health endpoint discovery, got %q", state.StatusHealthy, svc.Status)
	}
	if tp.calls.Load() != 0 {
		t.Errorf("expected token provider NOT called when health endpoint found, got %d calls", tp.calls.Load())
	}
}

func TestCheckAll_CachedHealthEndpointWorks(t *testing.T) {
	store := state.NewStore()
	store.AddOrUpdate(state.Service{
		Name: "svc1", Namespace: "ns1", URL: "https://svc1.example.com",
		Status: state.StatusUnknown,
	})

	client := &mockHTTPProber{
		responses: map[string]mockResponse{
			"https://svc1.example.com":         {statusCode: 401, body: "Unauthorized"},
			"https://svc1.example.com/healthz": {statusCode: 200, body: "OK"},
		},
	}

	tp := &mockTokenProvider{token: "should-not-be-used"}
	disc := &mockEndpointDiscoverer{
		strategies: map[string]*EndpointStrategy{
			"ns1/svc1": {Type: "healthEndpoint", Endpoint: "https://svc1.example.com/healthz"},
		},
	}

	checker := NewChecker(store, store, client, time.Hour, history.NoopWriter{}, nil)
	checker.WithTokenProvider(tp).WithDiscoverer(disc)
	checker.checkAll(context.Background())

	svc, _ := store.Get("ns1", "svc1")
	if svc.Status != state.StatusHealthy {
		t.Errorf("expected %q via cached health endpoint, got %q", state.StatusHealthy, svc.Status)
	}
	if tp.calls.Load() != 0 {
		t.Errorf("expected token provider NOT called when cached endpoint works, got %d calls", tp.calls.Load())
	}
}

func TestCheckAll_CachedEndpointFailsFallsToOIDC(t *testing.T) {
	store := state.NewStore()
	store.AddOrUpdate(state.Service{
		Name: "svc1", Namespace: "ns1", URL: "https://svc1.example.com",
		Status: state.StatusUnknown,
	})

	authClient := &authAwareMockProber{
		unauthResponses: map[string]mockResponse{
			"https://svc1.example.com":         {statusCode: 401, body: "Unauthorized"},
			"https://svc1.example.com/healthz": {statusCode: 500, body: "Error"}, // cached endpoint now fails
		},
		authResponses: map[string]mockResponse{
			"https://svc1.example.com": {statusCode: 200, body: "OK"}, // OIDC retry succeeds
		},
	}

	tp := &mockTokenProvider{token: "fallback-token"}
	disc := &mockEndpointDiscoverer{
		strategies: map[string]*EndpointStrategy{
			"ns1/svc1": {Type: "healthEndpoint", Endpoint: "https://svc1.example.com/healthz"},
		},
	}

	checker := NewChecker(store, store, authClient, time.Hour, history.NoopWriter{}, nil)
	checker.WithTokenProvider(tp).WithDiscoverer(disc)
	checker.checkAll(context.Background())

	svc, _ := store.Get("ns1", "svc1")
	if svc.Status != state.StatusHealthy {
		t.Errorf("expected %q after fallback to OIDC, got %q", state.StatusHealthy, svc.Status)
	}
	if tp.calls.Load() != 1 {
		t.Errorf("expected token provider called once after cached endpoint failed, got %d", tp.calls.Load())
	}
	cleared := disc.getCleared()
	if len(cleared) != 1 || cleared[0] != "ns1/svc1" {
		t.Errorf("expected ClearStrategy called for ns1/svc1, got %v", cleared)
	}
}

func TestCheckAll_DiscoveryFindsNoEndpointsFallsToOIDC(t *testing.T) {
	store := state.NewStore()
	store.AddOrUpdate(state.Service{
		Name: "svc1", Namespace: "ns1", URL: "https://svc1.example.com",
		Status: state.StatusUnknown,
	})

	authClient := &authAwareMockProber{
		unauthResponses: map[string]mockResponse{
			"https://svc1.example.com": {statusCode: 401, body: "Unauthorized"},
		},
		authResponses: map[string]mockResponse{
			"https://svc1.example.com": {statusCode: 200, body: "OK"},
		},
	}

	tp := &mockTokenProvider{token: "oidc-token"}
	disc := &mockEndpointDiscoverer{
		strategies: make(map[string]*EndpointStrategy),
		discovered: map[string]*EndpointStrategy{
			"ns1/svc1": {Type: "oidcAuth"}, // No health endpoint found
		},
	}

	checker := NewChecker(store, store, authClient, time.Hour, history.NoopWriter{}, nil)
	checker.WithTokenProvider(tp).WithDiscoverer(disc)
	checker.checkAll(context.Background())

	svc, _ := store.Get("ns1", "svc1")
	if svc.Status != state.StatusHealthy {
		t.Errorf("expected %q after OIDC retry, got %q", state.StatusHealthy, svc.Status)
	}
	if tp.calls.Load() != 1 {
		t.Errorf("expected token provider called once, got %d", tp.calls.Load())
	}
}

// --- Task 10: Failure path tests ---

func TestCheckAll_OIDCProviderDownStaysAuthBlocked(t *testing.T) {
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

	tp := &mockTokenProvider{err: errors.New("OIDC provider unreachable")}

	checker := NewChecker(store, store, client, time.Hour, history.NoopWriter{}, nil)
	checker.WithTokenProvider(tp)
	checker.checkAll(context.Background())

	svc, _ := store.Get("ns1", "svc1")
	if svc.Status != state.StatusAuthBlocked {
		t.Errorf("expected %q when OIDC provider down, got %q", state.StatusAuthBlocked, svc.Status)
	}
	if tp.calls.Load() != 1 {
		t.Errorf("expected token provider called once on retry cycle, got %d", tp.calls.Load())
	}
}

func TestCheckAll_OIDCProviderDownHealthyServicesUnaffected(t *testing.T) {
	store := state.NewStore()
	store.AddOrUpdate(state.Service{
		Name: "healthy-svc", Namespace: "ns1", URL: "https://healthy.example.com",
		Status: state.StatusUnknown,
	})
	store.AddOrUpdate(state.Service{
		Name: "blocked-svc", Namespace: "ns1", URL: "https://blocked.example.com",
		Status: state.StatusUnknown,
	})

	client := &mockHTTPProber{
		responses: map[string]mockResponse{
			"https://healthy.example.com": {statusCode: 200, body: "OK"},
			"https://blocked.example.com": {statusCode: 401, body: "Unauthorized"},
		},
	}

	tp := &mockTokenProvider{err: errors.New("OIDC provider unreachable")}

	checker := NewChecker(store, store, client, time.Hour, history.NoopWriter{}, nil)
	checker.WithTokenProvider(tp)
	checker.checkAll(context.Background())

	healthy, _ := store.Get("ns1", "healthy-svc")
	if healthy.Status != state.StatusHealthy {
		t.Errorf("healthy service: expected %q, got %q", state.StatusHealthy, healthy.Status)
	}

	blocked, _ := store.Get("ns1", "blocked-svc")
	if blocked.Status != state.StatusAuthBlocked {
		t.Errorf("blocked service: expected %q, got %q", state.StatusAuthBlocked, blocked.Status)
	}
	if tp.calls.Load() != 1 {
		t.Errorf("expected token provider called once for blocked service, got %d", tp.calls.Load())
	}
}

func TestCheckAll_DiscoveryErrorFallsToOIDC(t *testing.T) {
	store := state.NewStore()
	store.AddOrUpdate(state.Service{
		Name: "svc1", Namespace: "ns1", URL: "https://svc1.example.com",
		Status: state.StatusUnknown,
	})

	authClient := &authAwareMockProber{
		unauthResponses: map[string]mockResponse{
			"https://svc1.example.com": {statusCode: 401, body: "Unauthorized"},
		},
		authResponses: map[string]mockResponse{
			"https://svc1.example.com": {statusCode: 200, body: "OK"},
		},
	}

	tp := &mockTokenProvider{token: "fallback-token"}
	disc := &mockEndpointDiscoverer{
		strategies:  make(map[string]*EndpointStrategy),
		discoverErr: errors.New("discovery network error"),
	}

	checker := NewChecker(store, store, authClient, time.Hour, history.NoopWriter{}, nil)
	checker.WithTokenProvider(tp).WithDiscoverer(disc)
	checker.checkAll(context.Background())

	svc, _ := store.Get("ns1", "svc1")
	if svc.Status != state.StatusHealthy {
		t.Errorf("expected %q after discovery error fallback to OIDC, got %q", state.StatusHealthy, svc.Status)
	}
	if tp.calls.Load() != 1 {
		t.Errorf("expected token provider called once after discovery error, got %d", tp.calls.Load())
	}
}

func TestProbeServiceWithAuth_ConnectionError(t *testing.T) {
	client := &mockHTTPProber{
		responses: map[string]mockResponse{
			"https://svc1.example.com": {err: errors.New("connection refused")},
		},
	}

	checker := NewChecker(nil, nil, client, time.Hour, history.NoopWriter{}, nil)
	result := checker.probeServiceWithAuth(context.Background(), "https://svc1.example.com", "some-token")

	if result.status != state.StatusUnhealthy {
		t.Errorf("expected %q on connection error, got %q", state.StatusUnhealthy, result.status)
	}
	if result.errorSnippet == nil || !strings.Contains(*result.errorSnippet, "connection refused") {
		t.Errorf("expected error snippet containing 'connection refused'")
	}
}

func TestCheckAll_AuthMethodClearedWhenNotUsed(t *testing.T) {
	store := state.NewStore()
	store.AddOrUpdate(state.Service{
		Name: "svc1", Namespace: "ns1", URL: "https://svc1.example.com",
		Status:     state.StatusHealthy,
		AuthMethod: "oidc",
	})

	client := &mockHTTPProber{
		responses: map[string]mockResponse{
			"https://svc1.example.com": {statusCode: 200, body: "OK"},
		},
	}

	checker := NewChecker(store, store, client, time.Hour, history.NoopWriter{}, nil)
	checker.checkAll(context.Background())

	svc, _ := store.Get("ns1", "svc1")
	if svc.AuthMethod != "" {
		t.Errorf("expected AuthMethod to be cleared, got %q", svc.AuthMethod)
	}
}

// --- Task 11: Concurrency tests ---

func TestCheckAll_MultipleAuthBlockedServicesConcurrent(t *testing.T) {
	store := state.NewStore()
	numServices := 5
	for i := 0; i < numServices; i++ {
		name := "svc" + strings.Repeat("x", i) // unique names
		store.AddOrUpdate(state.Service{
			Name: name, Namespace: "ns1",
			URL:    "https://" + name + ".example.com",
			Status: state.StatusUnknown,
		})
	}

	responses := map[string]mockResponse{}
	authResponses := map[string]mockResponse{}
	for i := 0; i < numServices; i++ {
		name := "svc" + strings.Repeat("x", i)
		url := "https://" + name + ".example.com"
		responses[url] = mockResponse{statusCode: 401, body: "Unauthorized"}
		authResponses[url] = mockResponse{statusCode: 200, body: "OK"}
	}

	authClient := &authAwareMockProber{
		unauthResponses: responses,
		authResponses:   authResponses,
	}

	tp := &mockTokenProvider{token: "concurrent-token"}

	checker := NewChecker(store, store, authClient, time.Hour, history.NoopWriter{}, nil)
	checker.WithTokenProvider(tp)
	checker.checkAll(context.Background())

	for i := 0; i < numServices; i++ {
		name := "svc" + strings.Repeat("x", i)
		svc, _ := store.Get("ns1", name)
		if svc.Status != state.StatusHealthy {
			t.Errorf("service %s: expected %q, got %q", name, state.StatusHealthy, svc.Status)
		}
	}

	// All services should have triggered a token call
	if tp.calls.Load() != int32(numServices) {
		t.Errorf("expected %d token calls, got %d", numServices, tp.calls.Load())
	}
}

func TestCheckAll_ExpectedStatusCodesStillApplyAfterOIDC(t *testing.T) {
	// A service with expectedCodes: [401] should be healthy even when OIDC is configured.
	// OIDC retry runs first (resolving the 401 to healthy via auth), but if the
	// initial probe returned 401 and expectedCodes includes 401, the override applies.
	store := state.NewStore()
	store.AddOrUpdate(state.Service{
		Name:                "authsvc",
		Namespace:           "custom",
		URL:                 "https://auth.local",
		ExpectedStatusCodes: []int{200, 401},
		Status:              state.StatusUnknown,
	})

	client := &mockHTTPProber{
		responses: map[string]mockResponse{
			"https://auth.local": {statusCode: 401, body: "Unauthorized"},
		},
	}

	tp := &mockTokenProvider{err: errors.New("OIDC down")}

	checker := NewChecker(store, store, client, time.Hour, history.NoopWriter{}, nil)
	checker.WithTokenProvider(tp)
	checker.checkAll(context.Background())

	svc, _ := store.Get("custom", "authsvc")
	// Even though OIDC retry failed (stays authBlocked), ExpectedStatusCodes should override
	if svc.Status != state.StatusHealthy {
		t.Errorf("expected %q (401 in expected codes overrides auth-blocked), got %q", state.StatusHealthy, svc.Status)
	}
}

func TestWithTokenProvider_ReturnsChecker(t *testing.T) {
	checker := NewChecker(nil, nil, nil, time.Hour, history.NoopWriter{}, nil)
	tp := &mockTokenProvider{}
	result := checker.WithTokenProvider(tp)
	if result != checker {
		t.Error("expected WithTokenProvider to return same checker for chaining")
	}
}

func TestWithDiscoverer_ReturnsChecker(t *testing.T) {
	checker := NewChecker(nil, nil, nil, time.Hour, history.NoopWriter{}, nil)
	disc := &mockEndpointDiscoverer{}
	result := checker.WithDiscoverer(disc)
	if result != checker {
		t.Error("expected WithDiscoverer to return same checker for chaining")
	}
}

// authAwareMockProber returns different responses based on whether an Authorization header is present.
type authAwareMockProber struct {
	mu               sync.Mutex
	unauthResponses  map[string]mockResponse
	authResponses    map[string]mockResponse
	capturedRequests []capturedRequest
}

func (m *authAwareMockProber) Do(req *http.Request) (*http.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.capturedRequests = append(m.capturedRequests, capturedRequest{
		url:    req.URL.String(),
		header: req.Header.Clone(),
	})

	var responses map[string]mockResponse
	if req.Header.Get("Authorization") != "" {
		responses = m.authResponses
	} else {
		responses = m.unauthResponses
	}

	resp, ok := responses[req.URL.String()]
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

func (m *authAwareMockProber) getCapturedRequests() []capturedRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]capturedRequest, len(m.capturedRequests))
	copy(cp, m.capturedRequests)
	return cp
}
