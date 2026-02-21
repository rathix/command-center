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
