package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// === Task 1: Configuration Loading Tests (AC #1, #2, #3, #4) ===

func TestConfigDefaults(t *testing.T) {
	// AC #1: Default values when no flags or env vars are provided
	cfg, err := loadConfig([]string{})
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("os.UserHomeDir() error = %v", err)
	}
	expectedKubeconfig := filepath.Join(home, ".kube", "config")

	checks := []struct {
		name string
		got  string
		want string
	}{
		{"ListenAddr", cfg.ListenAddr, ":8443"},
		{"Kubeconfig", cfg.Kubeconfig, expectedKubeconfig},
		{"DataDir", cfg.DataDir, "/data"},
		{"LogFormat", cfg.LogFormat, "json"},
		{"TLSCACert", cfg.TLSCACert, ""},
		{"TLSCert", cfg.TLSCert, ""},
		{"TLSKey", cfg.TLSKey, ""},
		{"ConfigFile", cfg.ConfigFile, ""},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}

	if cfg.HealthInterval != 30*time.Second {
		t.Errorf("HealthInterval = %v, want %v", cfg.HealthInterval, 30*time.Second)
	}
}

func TestConfigCLIFlag(t *testing.T) {
	// AC #2: CLI flag --listen-addr :9443 makes server listen on port 9443
	cfg, err := loadConfig([]string{"--listen-addr", ":9443"})
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if cfg.ListenAddr != ":9443" {
		t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, ":9443")
	}
}

func TestConfigEnvVarFallback(t *testing.T) {
	// AC #3: Env var LISTEN_ADDR=:9443 is used when no CLI flag overrides it
	t.Setenv("LISTEN_ADDR", ":9443")
	cfg, err := loadConfig([]string{})
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if cfg.ListenAddr != ":9443" {
		t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, ":9443")
	}
}

func TestConfigFlagPrecedenceOverEnv(t *testing.T) {
	// AC #4: CLI flag takes precedence over env var
	t.Setenv("LISTEN_ADDR", ":9443")
	cfg, err := loadConfig([]string{"--listen-addr", ":7777"})
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if cfg.ListenAddr != ":7777" {
		t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, ":7777")
	}
}

func TestConfigEnvVarAllParameters(t *testing.T) {
	// Verify env var fallback works for all supported parameters
	t.Setenv("LISTEN_ADDR", ":9999")
	t.Setenv("KUBECONFIG", "/custom/kubeconfig")
	t.Setenv("HEALTH_INTERVAL", "60s")
	t.Setenv("DATA_DIR", "/custom/data")
	t.Setenv("LOG_FORMAT", "text")
	t.Setenv("TLS_CA_CERT", "/ca.crt")
	t.Setenv("TLS_CERT", "/server.crt")
	t.Setenv("TLS_KEY", "/server.key")
	t.Setenv("CONFIG_FILE", "/custom/config.yaml")

	cfg, err := loadConfig([]string{})
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}

	checks := []struct {
		name string
		got  string
		want string
	}{
		{"ListenAddr", cfg.ListenAddr, ":9999"},
		{"Kubeconfig", cfg.Kubeconfig, "/custom/kubeconfig"},
		{"DataDir", cfg.DataDir, "/custom/data"},
		{"LogFormat", cfg.LogFormat, "text"},
		{"TLSCACert", cfg.TLSCACert, "/ca.crt"},
		{"TLSCert", cfg.TLSCert, "/server.crt"},
		{"TLSKey", cfg.TLSKey, "/server.key"},
		{"ConfigFile", cfg.ConfigFile, "/custom/config.yaml"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}

	if cfg.HealthInterval != 60*time.Second {
		t.Errorf("HealthInterval = %v, want %v", cfg.HealthInterval, 60*time.Second)
	}
}

func TestConfigHistoryFileDefaultUsesDataDir(t *testing.T) {
	cfg, err := loadConfig([]string{"--data-dir", "/custom/data"})
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	want := filepath.Join("/custom/data", "history.jsonl")
	if cfg.HistoryFile != want {
		t.Fatalf("HistoryFile = %q, want %q", cfg.HistoryFile, want)
	}
}

func TestConfigHistoryFileFlag(t *testing.T) {
	cfg, err := loadConfig([]string{"--history-file", "/tmp/custom-history.jsonl"})
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if cfg.HistoryFile != "/tmp/custom-history.jsonl" {
		t.Fatalf("HistoryFile = %q, want %q", cfg.HistoryFile, "/tmp/custom-history.jsonl")
	}
}

func TestConfigHistoryFileEnvFallback(t *testing.T) {
	t.Setenv("HISTORY_FILE", "/env/history.jsonl")
	cfg, err := loadConfig([]string{})
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if cfg.HistoryFile != "/env/history.jsonl" {
		t.Fatalf("HistoryFile = %q, want %q", cfg.HistoryFile, "/env/history.jsonl")
	}
}

func TestConfigHistoryFileFlagPrecedenceOverEnv(t *testing.T) {
	t.Setenv("HISTORY_FILE", "/env/history.jsonl")
	cfg, err := loadConfig([]string{"--history-file", "/flag/history.jsonl"})
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if cfg.HistoryFile != "/flag/history.jsonl" {
		t.Fatalf("HistoryFile = %q, want %q", cfg.HistoryFile, "/flag/history.jsonl")
	}
}

func TestConfigInvalidHealthInterval(t *testing.T) {
	// Invalid health interval should return an error
	t.Setenv("HEALTH_INTERVAL", "not-a-duration")
	_, err := loadConfig([]string{})
	if err == nil {
		t.Fatal("loadConfig() with invalid health interval should return error")
	}
	if !strings.Contains(err.Error(), "invalid health interval") {
		t.Errorf("error should mention invalid health interval, got: %v", err)
	}
}

func TestConfigNonPositiveHealthIntervalReturnsError(t *testing.T) {
	_, err := loadConfig([]string{"--health-interval", "0s"})
	if err == nil {
		t.Fatal("loadConfig() with non-positive health interval should return error")
	}
	if !strings.Contains(err.Error(), "at least 1s") {
		t.Fatalf("error should mention at least 1s, got: %v", err)
	}
}

func TestConfigInvalidFlagReturnsError(t *testing.T) {
	// Unknown flags should return an error
	_, err := loadConfig([]string{"--unknown-flag", "value"})
	if err == nil {
		t.Error("loadConfig() with unknown flag should return error, got nil")
	}
}

// === Task 2: Structured Logging Tests (AC #6, #7) ===

func TestSetupLoggerJSON(t *testing.T) {
	// AC #6: --log-format json produces structured JSON output
	var buf bytes.Buffer
	logger := setupLoggerWithWriter("json", &buf)
	logger.Info("test message", "key", "value")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("JSON log output is not valid JSON: %v\nOutput: %s", err, buf.String())
	}
	if msg, ok := entry["msg"].(string); !ok || msg != "test message" {
		t.Errorf("msg = %v, want %q", entry["msg"], "test message")
	}
}

func TestSetupLoggerText(t *testing.T) {
	// AC #7: --log-format text produces human-readable text
	var buf bytes.Buffer
	logger := setupLoggerWithWriter("text", &buf)
	logger.Info("test message", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Errorf("text log output should contain message, got: %s", output)
	}
	// Text format should NOT be valid JSON
	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err == nil {
		t.Error("text log output should not be valid JSON")
	}
}

// === Task 3: Graceful Shutdown Tests (AC #5) ===

func getFreeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("skipping network-bound test: cannot bind loopback socket: %v", err)
	}
	addr := l.Addr().String()
	l.Close()
	return addr
}

func TestGracefulShutdownViaContext(t *testing.T) {
	// AC #5: SIGTERM/SIGINT causes graceful shutdown
	// We test via context cancellation since signal.NotifyContext wires signals to context
	ctx, cancel := context.WithCancel(context.Background())

	addr := getFreeAddr(t)
	cfg, err := loadConfig([]string{"--listen-addr", addr})
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	cfg.Dev = true // dev mode uses plain HTTP, no TLS setup needed
	cfg.HistoryFile = filepath.Join(t.TempDir(), "history.jsonl")

	runErr := make(chan error, 1)
	go func() {
		runErr <- run(ctx, cfg)
	}()

	// Give the server time to start
	time.Sleep(200 * time.Millisecond)

	// Cancel context (simulates SIGTERM/SIGINT via signal.NotifyContext)
	cancel()

	select {
	case err := <-runErr:
		if err != nil {
			t.Errorf("run() after shutdown returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run() did not shut down within 5 seconds")
	}
}

// === Task 4: Dev Mode Plain HTTP Test ===

func TestDevModeUsesPlainHTTP(t *testing.T) {
	// Dev mode should use plain HTTP, not HTTPS/mTLS
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	addr := getFreeAddr(t)
	cfg, err := loadConfig([]string{"--listen-addr", addr})
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	cfg.Dev = true
	cfg.HistoryFile = filepath.Join(t.TempDir(), "history.jsonl")

	go func() {
		_ = run(ctx, cfg)
	}()

	// Give the server time to start
	time.Sleep(200 * time.Millisecond)

	// Plain HTTP request should succeed — if server were using TLS, this would fail
	resp, err := http.Get("http://" + addr + "/")
	if err != nil {
		t.Fatalf("HTTP GET failed: %v (server may be using TLS instead of plain HTTP)", err)
	}
	resp.Body.Close()
	// 502 is expected since Vite dev server isn't running, but HTTP layer worked
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("expected 502 (Vite not running), got %d", resp.StatusCode)
	}
}

func TestSetupLoggerReturnsCorrectHandlerType(t *testing.T) {
	cases := []struct {
		format string
		isJSON bool
	}{
		{"json", true},
		{"text", false},
	}
	for _, tc := range cases {
		t.Run(tc.format, func(t *testing.T) {
			logger := setupLogger(tc.format)
			_, ok := logger.Handler().(*slog.JSONHandler)
			if ok != tc.isJSON {
				t.Errorf("setupLogger(%q): JSONHandler = %v, want %v", tc.format, ok, tc.isJSON)
			}
		})
	}
}

func TestConfigInvalidLogFormatReturnsError(t *testing.T) {
	_, err := loadConfig([]string{"--log-format", "invalid"})
	if err == nil {
		t.Error("loadConfig() with invalid log format should return error")
	}
	if !strings.Contains(err.Error(), "unsupported log format") {
		t.Errorf("error should mention unsupported log format, got: %v", err)
	}
}

func TestConfigEmptyLogFormatReturnsError(t *testing.T) {
	_, err := loadConfig([]string{"--log-format", ""})
	if err == nil {
		t.Error("loadConfig() with empty log format should return error")
	}
}

func TestConfigKubeconfigExpandsHomePath(t *testing.T) {
	// Default kubeconfig should be an absolute path, not ~
	cfg, err := loadConfig([]string{})
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if strings.HasPrefix(cfg.Kubeconfig, "~") {
		t.Errorf("Kubeconfig default should be expanded, got %q", cfg.Kubeconfig)
	}
	if !filepath.IsAbs(cfg.Kubeconfig) {
		t.Errorf("Kubeconfig default should be absolute path, got %q", cfg.Kubeconfig)
	}
}

func TestGetEnvBoolParsesBoolValues(t *testing.T) {
	cases := []struct {
		value string
		want  bool
	}{
		{"true", true},
		{"TRUE", true},
		{"True", true},
		{"1", true},
		{"false", false},
		{"FALSE", false},
		{"False", false},
		{"0", false},
	}
	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			t.Setenv("TEST_BOOL", tc.value)
			got := getEnvBool("TEST_BOOL", false)
			if got != tc.want {
				t.Errorf("getEnvBool(%q) = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}

func TestGetEnvBoolInvalidFallsBackToDefault(t *testing.T) {
	t.Setenv("TEST_BOOL", "not-a-bool")
	got := getEnvBool("TEST_BOOL", true)
	if !got {
		t.Error("getEnvBool with invalid value should return fallback (true)")
	}
}

// === SSE Endpoint Integration Tests ===

func TestSSEEndpointReturnsEventStream(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	addr := getFreeAddr(t)
	cfg, err := loadConfig([]string{"--listen-addr", addr})
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	cfg.Dev = true
	cfg.HistoryFile = filepath.Join(t.TempDir(), "history.jsonl")

	go func() {
		_ = run(ctx, cfg)
	}()

	// Give the server time to start
	time.Sleep(300 * time.Millisecond)

	resp, err := http.Get("http://" + addr + "/api/events")
	if err != nil {
		t.Fatalf("GET /api/events failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "text/event-stream" {
		t.Errorf("expected Content-Type 'text/event-stream', got %q", ct)
	}
}

// === Story 2.1: K8s Watcher Integration Tests ===

func TestRunWithInvalidKubeconfigContinuesServing(t *testing.T) {
	// When kubeconfig is invalid, run() should warn but continue serving
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	addr := getFreeAddr(t)
	cfg, err := loadConfig([]string{"--listen-addr", addr, "--kubeconfig", "/nonexistent/kubeconfig"})
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	cfg.Dev = true
	cfg.HistoryFile = filepath.Join(t.TempDir(), "history.jsonl")

	runErr := make(chan error, 1)
	go func() {
		runErr <- run(ctx, cfg)
	}()

	// Give the server time to start
	time.Sleep(300 * time.Millisecond)

	// Server should still be running despite invalid kubeconfig
	resp, err := http.Get("http://" + addr + "/")
	if err != nil {
		t.Fatalf("server not running after invalid kubeconfig: %v", err)
	}
	resp.Body.Close()

	cancel()

	select {
	case err := <-runErr:
		if err != nil {
			t.Errorf("run() returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run() did not shut down within 5 seconds after cancel")
	}
}

// === Session Duration Config Tests ===

func TestConfigSessionDurationDefault(t *testing.T) {
	cfg, err := loadConfig([]string{})
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if cfg.SessionDuration != 24*time.Hour {
		t.Errorf("SessionDuration = %v, want 24h", cfg.SessionDuration)
	}
}

func TestConfigSessionDurationFlag(t *testing.T) {
	cfg, err := loadConfig([]string{"--session-duration", "12h"})
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if cfg.SessionDuration != 12*time.Hour {
		t.Errorf("SessionDuration = %v, want 12h", cfg.SessionDuration)
	}
}

func TestConfigSessionDurationEnv(t *testing.T) {
	t.Setenv("SESSION_DURATION", "48h")
	cfg, err := loadConfig([]string{})
	if err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	}
	if cfg.SessionDuration != 48*time.Hour {
		t.Errorf("SessionDuration = %v, want 48h", cfg.SessionDuration)
	}
}

func TestConfigSessionDurationTooShort(t *testing.T) {
	_, err := loadConfig([]string{"--session-duration", "30s"})
	if err == nil {
		t.Fatal("loadConfig() with 30s session duration should return error")
	}
	if !strings.Contains(err.Error(), "at least 1m") {
		t.Errorf("error should mention at least 1m, got: %v", err)
	}
}

func TestConfigSessionDurationTooLong(t *testing.T) {
	_, err := loadConfig([]string{"--session-duration", "8760h"})
	if err == nil {
		t.Fatal("loadConfig() with 8760h session duration should return error")
	}
	if !strings.Contains(err.Error(), "at most 720h") {
		t.Errorf("error should mention at most 720h, got: %v", err)
	}
}

func TestConfigSessionDurationNegative(t *testing.T) {
	_, err := loadConfig([]string{"--session-duration", "-1h"})
	if err == nil {
		t.Fatal("loadConfig() with negative session duration should return error")
	}
	if !strings.Contains(err.Error(), "positive") {
		t.Errorf("error should mention positive, got: %v", err)
	}
}

func TestConfigSessionDurationZero(t *testing.T) {
	_, err := loadConfig([]string{"--session-duration", "0s"})
	if err == nil {
		t.Fatal("loadConfig() with zero session duration should return error")
	}
	if !strings.Contains(err.Error(), "positive") {
		t.Errorf("error should mention positive, got: %v", err)
	}
}
