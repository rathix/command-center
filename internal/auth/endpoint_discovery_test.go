package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
)

// testServer creates an httptest.Server with configurable status codes per path.
// pathStatuses maps path (e.g., "/healthz") to HTTP status code.
// requestCounts tracks how many times each path was hit.
func testServer(pathStatuses map[string]int, requestCounts map[string]*atomic.Int64) *httptest.Server {
	mux := http.NewServeMux()
	for p, status := range pathStatuses {
		s := status
		counter := requestCounts[p]
		mux.HandleFunc(p, func(w http.ResponseWriter, r *http.Request) {
			if counter != nil {
				counter.Add(1)
			}
			w.WriteHeader(s)
		})
	}
	return httptest.NewServer(mux)
}

func newCounters() map[string]*atomic.Int64 {
	return map[string]*atomic.Int64{
		"/healthz":     {},
		"/health":      {},
		"/ping":        {},
		"/api/health":  {},
	}
}

func TestDiscover_FirstPathSucceeds(t *testing.T) {
	counters := newCounters()
	ts := testServer(map[string]int{
		"/healthz":    200,
		"/health":     404,
		"/ping":       404,
		"/api/health": 404,
	}, counters)
	defer ts.Close()

	d := NewEndpointDiscoverer(ts.Client(), nil)
	strategy, err := d.Discover(context.Background(), "default/myapp", ts.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strategy.Type != "healthEndpoint" {
		t.Fatalf("expected healthEndpoint, got %s", strategy.Type)
	}
	if strategy.Endpoint != ts.URL+"/healthz" {
		t.Fatalf("expected %s/healthz, got %s", ts.URL, strategy.Endpoint)
	}
	// Should NOT have probed subsequent paths
	if counters["/health"].Load() != 0 {
		t.Errorf("expected /health not probed, got %d", counters["/health"].Load())
	}
	if counters["/ping"].Load() != 0 {
		t.Errorf("expected /ping not probed, got %d", counters["/ping"].Load())
	}
	if counters["/api/health"].Load() != 0 {
		t.Errorf("expected /api/health not probed, got %d", counters["/api/health"].Load())
	}
}

func TestDiscover_SecondPathSucceeds(t *testing.T) {
	counters := newCounters()
	ts := testServer(map[string]int{
		"/healthz":    404,
		"/health":     200,
		"/ping":       404,
		"/api/health": 404,
	}, counters)
	defer ts.Close()

	d := NewEndpointDiscoverer(ts.Client(), nil)
	strategy, err := d.Discover(context.Background(), "default/myapp", ts.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strategy.Type != "healthEndpoint" {
		t.Fatalf("expected healthEndpoint, got %s", strategy.Type)
	}
	if strategy.Endpoint != ts.URL+"/health" {
		t.Fatalf("expected %s/health, got %s", ts.URL, strategy.Endpoint)
	}
	if counters["/ping"].Load() != 0 {
		t.Errorf("expected /ping not probed, got %d", counters["/ping"].Load())
	}
}

func TestDiscover_ThirdPathSucceeds(t *testing.T) {
	counters := newCounters()
	ts := testServer(map[string]int{
		"/healthz":    404,
		"/health":     404,
		"/ping":       200,
		"/api/health": 404,
	}, counters)
	defer ts.Close()

	d := NewEndpointDiscoverer(ts.Client(), nil)
	strategy, err := d.Discover(context.Background(), "default/myapp", ts.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strategy.Type != "healthEndpoint" {
		t.Fatalf("expected healthEndpoint, got %s", strategy.Type)
	}
	if strategy.Endpoint != ts.URL+"/ping" {
		t.Fatalf("expected %s/ping, got %s", ts.URL, strategy.Endpoint)
	}
	if counters["/api/health"].Load() != 0 {
		t.Errorf("expected /api/health not probed, got %d", counters["/api/health"].Load())
	}
}

func TestDiscover_FourthPathSucceeds(t *testing.T) {
	counters := newCounters()
	ts := testServer(map[string]int{
		"/healthz":    404,
		"/health":     404,
		"/ping":       404,
		"/api/health": 200,
	}, counters)
	defer ts.Close()

	d := NewEndpointDiscoverer(ts.Client(), nil)
	strategy, err := d.Discover(context.Background(), "default/myapp", ts.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strategy.Type != "healthEndpoint" {
		t.Fatalf("expected healthEndpoint, got %s", strategy.Type)
	}
	if strategy.Endpoint != ts.URL+"/api/health" {
		t.Fatalf("expected %s/api/health, got %s", ts.URL, strategy.Endpoint)
	}
}

func TestDiscover_AllPathsFail(t *testing.T) {
	counters := newCounters()
	ts := testServer(map[string]int{
		"/healthz":    404,
		"/health":     500,
		"/ping":       403,
		"/api/health": 502,
	}, counters)
	defer ts.Close()

	d := NewEndpointDiscoverer(ts.Client(), nil)
	strategy, err := d.Discover(context.Background(), "default/myapp", ts.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strategy.Type != "oidcAuth" {
		t.Fatalf("expected oidcAuth, got %s", strategy.Type)
	}
	if strategy.Endpoint != "" {
		t.Fatalf("expected empty endpoint, got %s", strategy.Endpoint)
	}
	// All paths should have been probed
	for path, counter := range counters {
		if counter.Load() != 1 {
			t.Errorf("expected %s probed once, got %d", path, counter.Load())
		}
	}
}

func TestDiscover_AllConnectionErrors(t *testing.T) {
	// Use a server that's already closed to get connection errors
	ts := httptest.NewServer(http.NotFoundHandler())
	client := ts.Client()
	ts.Close()

	d := NewEndpointDiscoverer(client, nil)
	strategy, err := d.Discover(context.Background(), "default/myapp", ts.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strategy.Type != "oidcAuth" {
		t.Fatalf("expected oidcAuth, got %s", strategy.Type)
	}
	if strategy.Endpoint != "" {
		t.Fatalf("expected empty endpoint, got %s", strategy.Endpoint)
	}
}

func TestDiscover_CacheHit(t *testing.T) {
	counters := newCounters()
	ts := testServer(map[string]int{
		"/healthz":    200,
		"/health":     404,
		"/ping":       404,
		"/api/health": 404,
	}, counters)
	defer ts.Close()

	d := NewEndpointDiscoverer(ts.Client(), nil)

	// First call — should probe
	s1, err := d.Discover(context.Background(), "default/myapp", ts.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if counters["/healthz"].Load() != 1 {
		t.Fatalf("expected 1 probe to /healthz, got %d", counters["/healthz"].Load())
	}

	// Second call — should return cached, no additional probes
	s2, err := d.Discover(context.Background(), "default/myapp", ts.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if counters["/healthz"].Load() != 1 {
		t.Fatalf("expected still 1 probe to /healthz after cache hit, got %d", counters["/healthz"].Load())
	}
	if s1.Type != s2.Type || s1.Endpoint != s2.Endpoint {
		t.Fatalf("cached result mismatch: %+v vs %+v", s1, s2)
	}
}

func TestClearStrategy_ThenRediscover(t *testing.T) {
	counters := newCounters()
	ts := testServer(map[string]int{
		"/healthz":    200,
		"/health":     404,
		"/ping":       404,
		"/api/health": 404,
	}, counters)
	defer ts.Close()

	d := NewEndpointDiscoverer(ts.Client(), nil)

	// Discover — should probe
	_, err := d.Discover(context.Background(), "default/myapp", ts.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if counters["/healthz"].Load() != 1 {
		t.Fatalf("expected 1 probe, got %d", counters["/healthz"].Load())
	}

	// Clear cache
	d.ClearStrategy("default/myapp")

	// GetStrategy should return nil after clear
	if s := d.GetStrategy("default/myapp"); s != nil {
		t.Fatalf("expected nil after clear, got %+v", s)
	}

	// Re-discover — should probe again
	_, err = d.Discover(context.Background(), "default/myapp", ts.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if counters["/healthz"].Load() != 2 {
		t.Fatalf("expected 2 probes after clear+rediscover, got %d", counters["/healthz"].Load())
	}
}

func TestGetStrategy_UnknownKey(t *testing.T) {
	d := NewEndpointDiscoverer(http.DefaultClient, nil)
	if s := d.GetStrategy("unknown/service"); s != nil {
		t.Fatalf("expected nil for unknown key, got %+v", s)
	}
}

func TestBuildProbeURL_TrailingSlash(t *testing.T) {
	got := buildProbeURL("https://svc.local/", "/healthz")
	want := "https://svc.local/healthz"
	if got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestBuildProbeURL_NoTrailingSlash(t *testing.T) {
	got := buildProbeURL("https://svc.local", "/healthz")
	want := "https://svc.local/healthz"
	if got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestBuildProbeURL_WithPathComponent(t *testing.T) {
	got := buildProbeURL("https://svc.local/app", "/healthz")
	want := "https://svc.local/app/healthz"
	if got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestBuildProbeURL_WithPathComponentTrailingSlash(t *testing.T) {
	got := buildProbeURL("https://svc.local/app/", "/api/health")
	want := "https://svc.local/app/api/health"
	if got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestBuildProbeURL_EmptyBaseURL(t *testing.T) {
	got := buildProbeURL("", "/healthz")
	want := "/healthz"
	if got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestDiscover_ContextCancellation(t *testing.T) {
	counters := newCounters()
	ts := testServer(map[string]int{
		"/healthz":    404,
		"/health":     404,
		"/ping":       404,
		"/api/health": 404,
	}, counters)
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	d := NewEndpointDiscoverer(ts.Client(), nil)
	_, err := d.Discover(ctx, "default/myapp", ts.URL)
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

func TestDiscover_ConcurrentAccess(t *testing.T) {
	counters := newCounters()
	ts := testServer(map[string]int{
		"/healthz":    200,
		"/health":     404,
		"/ping":       404,
		"/api/health": 404,
	}, counters)
	defer ts.Close()

	d := NewEndpointDiscoverer(ts.Client(), nil)

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := range goroutines {
		go func(idx int) {
			defer wg.Done()
			key := fmt.Sprintf("ns/svc-%d", idx%5) // 5 different keys

			s, err := d.Discover(context.Background(), key, ts.URL)
			if err != nil {
				t.Errorf("goroutine %d: unexpected error: %v", idx, err)
				return
			}
			if s.Type != "healthEndpoint" {
				t.Errorf("goroutine %d: expected healthEndpoint, got %s", idx, s.Type)
				return
			}

			// Interleave GetStrategy and ClearStrategy
			_ = d.GetStrategy(key)
			if idx%3 == 0 {
				d.ClearStrategy(key)
			}
		}(i)
	}
	wg.Wait()
}

// mockProber records whether Do was called on the injected client.
type mockProber struct {
	calls atomic.Int64
	inner HTTPProber
}

func (m *mockProber) Do(req *http.Request) (*http.Response, error) {
	m.calls.Add(1)
	return m.inner.Do(req)
}

func TestDiscover_UsesInjectedClient(t *testing.T) {
	ts := testServer(map[string]int{
		"/healthz":    200,
		"/health":     404,
		"/ping":       404,
		"/api/health": 404,
	}, nil)
	defer ts.Close()

	mock := &mockProber{inner: ts.Client()}
	d := NewEndpointDiscoverer(mock, nil)

	_, err := d.Discover(context.Background(), "default/myapp", ts.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock.calls.Load() == 0 {
		t.Fatal("expected injected client Do to be called, but it was not")
	}
}

func TestDiscover_2xxRange(t *testing.T) {
	for _, code := range []int{200, 201, 204, 299} {
		t.Run(fmt.Sprintf("status_%d", code), func(t *testing.T) {
			ts := testServer(map[string]int{
				"/healthz":    code,
				"/health":     404,
				"/ping":       404,
				"/api/health": 404,
			}, nil)
			defer ts.Close()

			d := NewEndpointDiscoverer(ts.Client(), nil)
			strategy, err := d.Discover(context.Background(), "default/myapp", ts.URL)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if strategy.Type != "healthEndpoint" {
				t.Fatalf("expected healthEndpoint for status %d, got %s", code, strategy.Type)
			}
		})
	}
}

func TestNewEndpointDiscoverer_NilLogger(t *testing.T) {
	// Must not panic
	d := NewEndpointDiscoverer(http.DefaultClient, nil)
	if d == nil {
		t.Fatal("expected non-nil discoverer")
	}
}
