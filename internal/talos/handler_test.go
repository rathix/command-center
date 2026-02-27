package talos

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHandler_NilPollerReturnsNotConfigured(t *testing.T) {
	h := NewHandler(nil)
	req := httptest.NewRequest("GET", "/api/nodes", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp nodesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Configured {
		t.Error("expected configured=false")
	}
	if resp.Nodes != nil {
		t.Errorf("expected null nodes, got %v", resp.Nodes)
	}
}

func TestHandler_ReturnsNodesWhenConfigured(t *testing.T) {
	now := time.Now()
	nodes := []NodeHealth{
		{Name: "node-01", Health: NodeReady, Role: "controlplane", LastSeen: now},
		{Name: "node-02", Health: NodeNotReady, Role: "worker", LastSeen: now},
	}

	client := &mockClient{
		listNodesFunc: func(ctx context.Context) ([]NodeHealth, error) {
			return nodes, nil
		},
		getMetricsFunc: func(ctx context.Context) (map[string]NodeMetrics, error) {
			return nil, nil
		},
	}

	p := NewPoller(client, 30*time.Second, testLogger())
	ctx := context.Background()
	p.poll(ctx)

	h := NewHandler(p)
	req := httptest.NewRequest("GET", "/api/nodes", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp nodesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Configured {
		t.Error("expected configured=true")
	}
	if len(resp.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(resp.Nodes))
	}
	if resp.Stale {
		t.Error("expected stale=false")
	}
	if resp.Error != nil {
		t.Errorf("expected no error, got %v", *resp.Error)
	}
}

func TestHandler_StaleWhenLastErrorSet(t *testing.T) {
	callCount := 0
	client := &mockClient{
		listNodesFunc: func(ctx context.Context) ([]NodeHealth, error) {
			callCount++
			if callCount == 1 {
				return []NodeHealth{{Name: "node-01", Health: NodeReady, Role: "worker"}}, nil
			}
			return nil, fmt.Errorf("connection refused")
		},
		getMetricsFunc: func(ctx context.Context) (map[string]NodeMetrics, error) {
			return nil, nil
		},
	}

	p := NewPoller(client, 30*time.Second, testLogger())
	ctx := context.Background()
	p.poll(ctx) // success
	p.poll(ctx) // failure

	h := NewHandler(p)
	req := httptest.NewRequest("GET", "/api/nodes", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	var resp nodesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Stale {
		t.Error("expected stale=true when lastError is set")
	}
	if resp.Error == nil {
		t.Error("expected error field to be set")
	}
}

func TestHandler_ReturnsEmptyArrayWhenNoNodesYet(t *testing.T) {
	client := &mockClient{
		listNodesFunc: func(ctx context.Context) ([]NodeHealth, error) {
			return nil, nil
		},
		getMetricsFunc: func(ctx context.Context) (map[string]NodeMetrics, error) {
			return nil, nil
		},
	}

	p := NewPoller(client, 30*time.Second, testLogger())
	ctx := context.Background()
	p.poll(ctx)

	h := NewHandler(p)
	req := httptest.NewRequest("GET", "/api/nodes", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	var resp nodesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Nodes == nil {
		t.Error("expected non-nil empty array")
	}
	if len(resp.Nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(resp.Nodes))
	}
}

func TestHandler_IncludesMetricsInResponse(t *testing.T) {
	now := time.Now()
	nodes := []NodeHealth{
		{Name: "node-01", Health: NodeReady, Role: "worker", LastSeen: now},
	}

	client := &mockClient{
		listNodesFunc: func(ctx context.Context) ([]NodeHealth, error) {
			return nodes, nil
		},
		getMetricsFunc: func(ctx context.Context) (map[string]NodeMetrics, error) {
			return map[string]NodeMetrics{
				"node-01": {CPUPercent: 42.5, MemoryPercent: 60.0, DiskPercent: 30.0},
			}, nil
		},
	}

	p := NewPoller(client, 30*time.Second, testLogger())
	ctx := context.Background()
	p.poll(ctx)

	h := NewHandler(p)
	req := httptest.NewRequest("GET", "/api/nodes", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	var resp nodesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(resp.Nodes))
	}
	if resp.Nodes[0].Metrics == nil {
		t.Fatal("expected metrics on node")
	}
	if resp.Nodes[0].Metrics.CPUPercent != 42.5 {
		t.Errorf("expected CPU 42.5, got %f", resp.Nodes[0].Metrics.CPUPercent)
	}
}

func TestMetricsHistoryHandler_ReturnsHistory(t *testing.T) {
	now := time.Now()
	nodes := []NodeHealth{
		{Name: "node-01", Health: NodeReady, Role: "worker", LastSeen: now},
	}

	client := &mockClient{
		listNodesFunc: func(ctx context.Context) ([]NodeHealth, error) {
			return nodes, nil
		},
		getMetricsFunc: func(ctx context.Context) (map[string]NodeMetrics, error) {
			return map[string]NodeMetrics{
				"node-01": {CPUPercent: 42.5, MemoryPercent: 60.0, DiskPercent: 30.0},
			}, nil
		},
	}

	p := NewPoller(client, 30*time.Second, testLogger())
	ctx := context.Background()
	p.poll(ctx)
	p.poll(ctx)

	mux := http.NewServeMux()
	mux.Handle("GET /api/nodes/{name}/metrics", NewMetricsHistoryHandler(p))

	req := httptest.NewRequest("GET", "/api/nodes/node-01/metrics", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp metricsHistoryResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.History) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(resp.History))
	}
}

func TestMetricsHistoryHandler_NilPollerReturns404(t *testing.T) {
	h := NewMetricsHistoryHandler(nil)
	req := httptest.NewRequest("GET", "/api/nodes/node-01/metrics", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}
