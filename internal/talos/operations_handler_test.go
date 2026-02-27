package talos

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOperationsHandler_RebootSuccess(t *testing.T) {
	rebootCalled := false
	client := &mockClient{
		listNodesFunc: func(ctx context.Context) ([]NodeHealth, error) {
			return []NodeHealth{{Name: "node-01", Health: NodeReady, Role: "worker"}}, nil
		},
		getMetricsFunc: func(ctx context.Context) (map[string]NodeMetrics, error) {
			return nil, nil
		},
		rebootFunc: func(ctx context.Context, nodeName string) error {
			rebootCalled = true
			if nodeName != "node-01" {
				t.Errorf("expected node-01, got %s", nodeName)
			}
			return nil
		},
	}

	p := NewPoller(client, 30*time.Second, testLogger())
	ctx := context.Background()
	p.poll(ctx)

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))

	mux := http.NewServeMux()
	mux.Handle("POST /api/talos/{node}/reboot", NewOperationsHandler(p, logger))

	req := httptest.NewRequest("POST", "/api/talos/node-01/reboot", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !rebootCalled {
		t.Error("expected Reboot to be called on mock client")
	}

	var resp operationResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.Success {
		t.Error("expected success=true")
	}
	if !strings.Contains(resp.Message, "node-01") {
		t.Errorf("expected message to contain node-01, got %q", resp.Message)
	}

	// Verify audit log
	if !strings.Contains(logBuf.String(), "reboot") {
		t.Errorf("expected audit log to contain 'reboot', got: %s", logBuf.String())
	}
}

func TestOperationsHandler_RebootAPIFailure(t *testing.T) {
	client := &mockClient{
		listNodesFunc: func(ctx context.Context) ([]NodeHealth, error) {
			return nil, nil
		},
		getMetricsFunc: func(ctx context.Context) (map[string]NodeMetrics, error) {
			return nil, nil
		},
		rebootFunc: func(ctx context.Context, nodeName string) error {
			return fmt.Errorf("talos API unavailable")
		},
	}

	p := NewPoller(client, 30*time.Second, testLogger())

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))

	mux := http.NewServeMux()
	mux.Handle("POST /api/talos/{node}/reboot", NewOperationsHandler(p, logger))

	req := httptest.NewRequest("POST", "/api/talos/node-01/reboot", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", w.Code)
	}

	var resp operationResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Success {
		t.Error("expected success=false")
	}
	if !strings.Contains(resp.Error, "talos API unavailable") {
		t.Errorf("expected error message, got %q", resp.Error)
	}
}

func TestUpgradeHandler_Success(t *testing.T) {
	upgradeCalled := false
	var capturedVersion string
	client := &mockClient{
		listNodesFunc: func(ctx context.Context) ([]NodeHealth, error) {
			return nil, nil
		},
		getMetricsFunc: func(ctx context.Context) (map[string]NodeMetrics, error) {
			return nil, nil
		},
		upgradeFunc: func(ctx context.Context, nodeName string, targetVersion string) error {
			upgradeCalled = true
			capturedVersion = targetVersion
			return nil
		},
	}

	p := NewPoller(client, 30*time.Second, testLogger())

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))

	mux := http.NewServeMux()
	mux.Handle("POST /api/talos/{node}/upgrade", NewUpgradeHandler(p, logger))

	body := `{"targetVersion":"v1.8.0"}`
	req := httptest.NewRequest("POST", "/api/talos/node-01/upgrade", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !upgradeCalled {
		t.Error("expected Upgrade to be called")
	}
	if capturedVersion != "v1.8.0" {
		t.Errorf("expected version v1.8.0, got %s", capturedVersion)
	}

	// Verify audit log
	if !strings.Contains(logBuf.String(), "upgrade") {
		t.Errorf("expected audit log for upgrade, got: %s", logBuf.String())
	}
}

func TestUpgradeHandler_MissingTargetVersion(t *testing.T) {
	client := &mockClient{
		listNodesFunc: func(ctx context.Context) ([]NodeHealth, error) {
			return nil, nil
		},
		getMetricsFunc: func(ctx context.Context) (map[string]NodeMetrics, error) {
			return nil, nil
		},
	}

	p := NewPoller(client, 30*time.Second, testLogger())

	mux := http.NewServeMux()
	mux.Handle("POST /api/talos/{node}/upgrade", NewUpgradeHandler(p, testLogger()))

	body := `{}`
	req := httptest.NewRequest("POST", "/api/talos/node-01/upgrade", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	var resp operationResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Success {
		t.Error("expected success=false")
	}
}

func TestUpgradeInfoHandler_Success(t *testing.T) {
	client := &mockClient{
		listNodesFunc: func(ctx context.Context) ([]NodeHealth, error) {
			return nil, nil
		},
		getMetricsFunc: func(ctx context.Context) (map[string]NodeMetrics, error) {
			return nil, nil
		},
		getUpgradeInfoFunc: func(ctx context.Context, nodeName string) (*UpgradeInfo, error) {
			return &UpgradeInfo{CurrentVersion: "v1.7.0", TargetVersion: "v1.8.0"}, nil
		},
	}

	p := NewPoller(client, 30*time.Second, testLogger())

	mux := http.NewServeMux()
	mux.Handle("GET /api/talos/{node}/upgrade-info", NewUpgradeInfoHandler(p, testLogger()))

	req := httptest.NewRequest("GET", "/api/talos/node-01/upgrade-info", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp upgradeInfoResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.CurrentVersion != "v1.7.0" {
		t.Errorf("expected currentVersion v1.7.0, got %s", resp.CurrentVersion)
	}
	if resp.TargetVersion != "v1.8.0" {
		t.Errorf("expected targetVersion v1.8.0, got %s", resp.TargetVersion)
	}
}

func TestUpgradeInfoHandler_APIFailure(t *testing.T) {
	client := &mockClient{
		listNodesFunc: func(ctx context.Context) ([]NodeHealth, error) {
			return nil, nil
		},
		getMetricsFunc: func(ctx context.Context) (map[string]NodeMetrics, error) {
			return nil, nil
		},
		getUpgradeInfoFunc: func(ctx context.Context, nodeName string) (*UpgradeInfo, error) {
			return nil, fmt.Errorf("node not found")
		},
	}

	p := NewPoller(client, 30*time.Second, testLogger())

	mux := http.NewServeMux()
	mux.Handle("GET /api/talos/{node}/upgrade-info", NewUpgradeInfoHandler(p, testLogger()))

	req := httptest.NewRequest("GET", "/api/talos/node-01/upgrade-info", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", w.Code)
	}
}

func TestOperationsHandler_NilPollerReturns404(t *testing.T) {
	h := NewOperationsHandler(nil, testLogger())
	req := httptest.NewRequest("POST", "/api/talos/node-01/reboot", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestOperationsHandler_AuditLogOnReboot(t *testing.T) {
	client := &mockClient{
		listNodesFunc: func(ctx context.Context) ([]NodeHealth, error) {
			return nil, nil
		},
		getMetricsFunc: func(ctx context.Context) (map[string]NodeMetrics, error) {
			return nil, nil
		},
		rebootFunc: func(ctx context.Context, nodeName string) error {
			return nil
		},
	}

	p := NewPoller(client, 30*time.Second, testLogger())

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))

	mux := http.NewServeMux()
	mux.Handle("POST /api/talos/{node}/reboot", NewOperationsHandler(p, logger))

	req := httptest.NewRequest("POST", "/api/talos/node-01/reboot", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	logStr := logBuf.String()
	if !strings.Contains(logStr, "node-01") {
		t.Errorf("expected audit log to contain node name, got: %s", logStr)
	}
	if !strings.Contains(logStr, "reboot") {
		t.Errorf("expected audit log to contain operation type, got: %s", logStr)
	}
}

func TestOperationsHandler_AuditLogOnUpgrade(t *testing.T) {
	client := &mockClient{
		listNodesFunc: func(ctx context.Context) ([]NodeHealth, error) {
			return nil, nil
		},
		getMetricsFunc: func(ctx context.Context) (map[string]NodeMetrics, error) {
			return nil, nil
		},
		upgradeFunc: func(ctx context.Context, nodeName string, targetVersion string) error {
			return nil
		},
	}

	p := NewPoller(client, 30*time.Second, testLogger())

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))

	mux := http.NewServeMux()
	mux.Handle("POST /api/talos/{node}/upgrade", NewUpgradeHandler(p, logger))

	body := `{"targetVersion":"v1.8.0"}`
	req := httptest.NewRequest("POST", "/api/talos/node-01/upgrade", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	logStr := logBuf.String()
	if !strings.Contains(logStr, "node-01") {
		t.Errorf("expected audit log to contain node name, got: %s", logStr)
	}
	if !strings.Contains(logStr, "upgrade") {
		t.Errorf("expected audit log to contain operation type, got: %s", logStr)
	}
}
