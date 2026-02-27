package logtail

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"nhooyr.io/websocket"
)

// fakePodLogStreamer implements PodLogStreamer for testing.
type fakePodLogStreamer struct {
	mu          sync.Mutex
	streamFunc  func(ctx context.Context, namespace, pod string, opts *corev1.PodLogOptions) (io.ReadCloser, error)
	getPodFunc  func(ctx context.Context, namespace, pod string) (*corev1.Pod, error)
	streamCalls int
}

func (f *fakePodLogStreamer) StreamPodLogs(ctx context.Context, namespace, pod string, opts *corev1.PodLogOptions) (io.ReadCloser, error) {
	f.mu.Lock()
	f.streamCalls++
	f.mu.Unlock()
	if f.streamFunc != nil {
		return f.streamFunc(ctx, namespace, pod, opts)
	}
	return io.NopCloser(strings.NewReader("")), nil
}

func (f *fakePodLogStreamer) GetPod(ctx context.Context, namespace, pod string) (*corev1.Pod, error) {
	if f.getPodFunc != nil {
		return f.getPodFunc(ctx, namespace, pod)
	}
	return nil, nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// dialWS upgrades an httptest.Server to a WebSocket connection.
func dialWS(t *testing.T, srv *httptest.Server, path string) (*websocket.Conn, *http.Response) {
	t.Helper()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + path
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, resp, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	return conn, resp
}

func TestPodLogStreamerInterface(t *testing.T) {
	// Compile-time check that K8sStreamer satisfies PodLogStreamer
	var _ PodLogStreamer = &K8sStreamer{}
}

func TestHandler_StreamsLogLines(t *testing.T) {
	lines := "line1\nline2\nline3\n"
	streamer := &fakePodLogStreamer{
		streamFunc: func(ctx context.Context, namespace, pod string, opts *corev1.PodLogOptions) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(lines)), nil
		},
	}

	handler := NewHandler(streamer, testLogger())
	mux := http.NewServeMux()
	mux.Handle("GET /api/logs/{namespace}/{pod}", handler)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	conn, _ := dialWS(t, srv, "/api/logs/default/my-pod")
	defer conn.CloseNow()

	var received []string
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	for i := 0; i < 3; i++ {
		_, data, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("read %d: %v", i, err)
		}
		received = append(received, string(data))
	}

	if len(received) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(received))
	}
	if received[0] != "line1" || received[1] != "line2" || received[2] != "line3" {
		t.Errorf("unexpected lines: %v", received)
	}
}

func TestHandler_InvalidPod(t *testing.T) {
	streamer := &fakePodLogStreamer{
		streamFunc: func(ctx context.Context, namespace, pod string, opts *corev1.PodLogOptions) (io.ReadCloser, error) {
			return nil, &notFoundError{namespace: namespace, pod: pod}
		},
	}

	handler := NewHandler(streamer, testLogger())
	mux := http.NewServeMux()
	mux.Handle("GET /api/logs/{namespace}/{pod}", handler)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	conn, _ := dialWS(t, srv, "/api/logs/default/no-such-pod")
	defer conn.CloseNow()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Read the error message
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read error message: %v", err)
	}

	var msg map[string]string
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("unmarshal error message: %v", err)
	}
	if msg["type"] != "error" {
		t.Errorf("expected error type, got %q", msg["type"])
	}

	// Connection should be closed with 4404
	_, _, err = conn.Read(ctx)
	if err == nil {
		t.Fatal("expected connection to be closed")
	}
	closeErr := websocket.CloseStatus(err)
	if closeErr != 4404 {
		t.Errorf("expected close code 4404, got %d", closeErr)
	}
}

func TestHandler_PodRestart(t *testing.T) {
	callCount := 0
	var mu sync.Mutex
	streamer := &fakePodLogStreamer{
		streamFunc: func(ctx context.Context, namespace, pod string, opts *corev1.PodLogOptions) (io.ReadCloser, error) {
			mu.Lock()
			callCount++
			n := callCount
			mu.Unlock()
			if n == 1 {
				// First call: return some lines then EOF
				return io.NopCloser(strings.NewReader("before-restart\n")), nil
			}
			// Second call: return lines from restarted pod
			return io.NopCloser(strings.NewReader("after-restart\n")), nil
		},
		getPodFunc: func(ctx context.Context, namespace, pod string) (*corev1.Pod, error) {
			return &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: pod, Namespace: namespace},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
				},
			}, nil
		},
	}

	handler := NewHandler(streamer, testLogger())
	mux := http.NewServeMux()
	mux.Handle("GET /api/logs/{namespace}/{pod}", handler)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	conn, _ := dialWS(t, srv, "/api/logs/default/my-pod")
	defer conn.CloseNow()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var received []string
	// Read: "before-restart", control message, "after-restart"
	for i := 0; i < 3; i++ {
		_, data, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("read %d: %v", i, err)
		}
		received = append(received, string(data))
	}

	if received[0] != "before-restart" {
		t.Errorf("expected 'before-restart', got %q", received[0])
	}

	// Second message should be control: pod-restarted
	var ctrl map[string]string
	if err := json.Unmarshal([]byte(received[1]), &ctrl); err != nil {
		t.Fatalf("unmarshal control: %v", err)
	}
	if ctrl["type"] != "control" || ctrl["event"] != "pod-restarted" {
		t.Errorf("unexpected control message: %v", ctrl)
	}

	if received[2] != "after-restart" {
		t.Errorf("expected 'after-restart', got %q", received[2])
	}
}

func TestHandler_FilterCommand(t *testing.T) {
	// Provide a long stream with delays so filter can be applied mid-stream
	linesCh := make(chan string, 100)
	streamer := &fakePodLogStreamer{
		streamFunc: func(ctx context.Context, namespace, pod string, opts *corev1.PodLogOptions) (io.ReadCloser, error) {
			pr, pw := io.Pipe()
			go func() {
				defer pw.Close()
				for {
					select {
					case <-ctx.Done():
						return
					case line, ok := <-linesCh:
						if !ok {
							return
						}
						pw.Write([]byte(line + "\n"))
					}
				}
			}()
			return pr, nil
		},
	}

	handler := NewHandler(streamer, testLogger())
	mux := http.NewServeMux()
	mux.Handle("GET /api/logs/{namespace}/{pod}", handler)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	conn, _ := dialWS(t, srv, "/api/logs/default/my-pod")
	defer conn.CloseNow()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Send some lines - all should arrive before filter
	linesCh <- "info: normal message"
	linesCh <- "error: something failed"

	_, d1, _ := conn.Read(ctx)
	_, d2, _ := conn.Read(ctx)
	if string(d1) != "info: normal message" {
		t.Errorf("expected unfiltered line 1, got %q", string(d1))
	}
	if string(d2) != "error: something failed" {
		t.Errorf("expected unfiltered line 2, got %q", string(d2))
	}

	// Set filter to "error"
	filterCmd := `{"type":"filter","pattern":"error"}`
	err := conn.Write(ctx, websocket.MessageText, []byte(filterCmd))
	if err != nil {
		t.Fatalf("write filter: %v", err)
	}

	// Give filter time to apply
	time.Sleep(50 * time.Millisecond)

	// Send more lines - only "error" should pass
	linesCh <- "info: another normal"
	linesCh <- "error: another failure"

	_, d3, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read filtered: %v", err)
	}
	if string(d3) != "error: another failure" {
		t.Errorf("expected only error line, got %q", string(d3))
	}

	// Clear filter
	clearCmd := `{"type":"filter","pattern":""}`
	err = conn.Write(ctx, websocket.MessageText, []byte(clearCmd))
	if err != nil {
		t.Fatalf("write clear: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// Send more lines - all should arrive
	linesCh <- "info: cleared filter"
	_, d4, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read after clear: %v", err)
	}
	if string(d4) != "info: cleared filter" {
		t.Errorf("expected cleared line, got %q", string(d4))
	}
}

func TestHandler_FilterRegex(t *testing.T) {
	linesCh := make(chan string, 100)
	streamer := &fakePodLogStreamer{
		streamFunc: func(ctx context.Context, namespace, pod string, opts *corev1.PodLogOptions) (io.ReadCloser, error) {
			pr, pw := io.Pipe()
			go func() {
				defer pw.Close()
				for {
					select {
					case <-ctx.Done():
						return
					case line, ok := <-linesCh:
						if !ok {
							return
						}
						pw.Write([]byte(line + "\n"))
					}
				}
			}()
			return pr, nil
		},
	}

	handler := NewHandler(streamer, testLogger())
	mux := http.NewServeMux()
	mux.Handle("GET /api/logs/{namespace}/{pod}", handler)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	conn, _ := dialWS(t, srv, "/api/logs/default/my-pod")
	defer conn.CloseNow()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Set regex filter
	filterCmd := `{"type":"filter","pattern":"^ERROR|^WARN"}`
	err := conn.Write(ctx, websocket.MessageText, []byte(filterCmd))
	if err != nil {
		t.Fatalf("write filter: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	linesCh <- "INFO: normal"
	linesCh <- "ERROR: bad thing"
	linesCh <- "DEBUG: trace"
	linesCh <- "WARN: careful"

	_, d1, _ := conn.Read(ctx)
	_, d2, _ := conn.Read(ctx)
	if string(d1) != "ERROR: bad thing" {
		t.Errorf("expected ERROR line, got %q", string(d1))
	}
	if string(d2) != "WARN: careful" {
		t.Errorf("expected WARN line, got %q", string(d2))
	}
}

func TestHandler_FilterInvalidRegex(t *testing.T) {
	linesCh := make(chan string, 100)
	streamer := &fakePodLogStreamer{
		streamFunc: func(ctx context.Context, namespace, pod string, opts *corev1.PodLogOptions) (io.ReadCloser, error) {
			pr, pw := io.Pipe()
			go func() {
				defer pw.Close()
				for {
					select {
					case <-ctx.Done():
						return
					case line, ok := <-linesCh:
						if !ok {
							return
						}
						pw.Write([]byte(line + "\n"))
					}
				}
			}()
			return pr, nil
		},
	}

	handler := NewHandler(streamer, testLogger())
	mux := http.NewServeMux()
	mux.Handle("GET /api/logs/{namespace}/{pod}", handler)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	conn, _ := dialWS(t, srv, "/api/logs/default/my-pod")
	defer conn.CloseNow()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Set invalid regex - should fall back to substring
	filterCmd := `{"type":"filter","pattern":"[invalid"}`
	err := conn.Write(ctx, websocket.MessageText, []byte(filterCmd))
	if err != nil {
		t.Fatalf("write filter: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	linesCh <- "has [invalid bracket"
	linesCh <- "no match here"

	_, d1, _ := conn.Read(ctx)
	if string(d1) != "has [invalid bracket" {
		t.Errorf("expected substring match, got %q", string(d1))
	}
}

func TestHandler_MissingPathParams(t *testing.T) {
	streamer := &fakePodLogStreamer{}
	handler := NewHandler(streamer, testLogger())

	// Test with no path params
	req := httptest.NewRequest("GET", "/api/logs//", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing params, got %d", rec.Code)
	}
}

func TestFilterState_Concurrent(t *testing.T) {
	fs := &filterState{}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			fs.setPattern("error")
		}()
		go func() {
			defer wg.Done()
			fs.matches("some line with error")
		}()
	}
	wg.Wait()
}

func TestFilterState_Matches(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		line    string
		want    bool
	}{
		{"empty pattern matches all", "", "anything", true},
		{"substring match", "error", "this has error in it", true},
		{"substring no match", "error", "this is fine", false},
		{"regex match", "^ERR", "ERR: something", true},
		{"regex no match", "^ERR", "not ERR", false},
		{"invalid regex falls back to substring", "[bad", "has [bad in it", true},
		{"invalid regex no substring match", "[bad", "no match", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := &filterState{}
			fs.setPattern(tt.pattern)
			if got := fs.matches(tt.line); got != tt.want {
				t.Errorf("matches(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}

func TestHandler_InternalError(t *testing.T) {
	streamer := &fakePodLogStreamer{
		streamFunc: func(ctx context.Context, namespace, pod string, opts *corev1.PodLogOptions) (io.ReadCloser, error) {
			return nil, &internalError{message: "k8s API failure"}
		},
	}

	handler := NewHandler(streamer, testLogger())
	mux := http.NewServeMux()
	mux.Handle("GET /api/logs/{namespace}/{pod}", handler)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	conn, _ := dialWS(t, srv, "/api/logs/default/my-pod")
	defer conn.CloseNow()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Read the error message
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read error message: %v", err)
	}

	var msg map[string]string
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("unmarshal error message: %v", err)
	}
	if msg["type"] != "error" {
		t.Errorf("expected error type, got %q", msg["type"])
	}

	// Connection should be closed with 4500
	_, _, err = conn.Read(ctx)
	if err == nil {
		t.Fatal("expected connection to be closed")
	}
	closeErr := websocket.CloseStatus(err)
	if closeErr != 4500 {
		t.Errorf("expected close code 4500, got %d", closeErr)
	}
}

// Helper to create a blocking reader from a channel
func chanReader(ctx context.Context, ch <-chan string) io.ReadCloser {
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case line, ok := <-ch:
				if !ok {
					return
				}
				_, _ = pw.Write([]byte(line + "\n"))
			}
		}
	}()
	return pr
}

func TestHandler_OptionsApplied(t *testing.T) {
	streamer := &fakePodLogStreamer{
		streamFunc: func(ctx context.Context, namespace, pod string, opts *corev1.PodLogOptions) (io.ReadCloser, error) {
			if opts.TailLines == nil || *opts.TailLines != 500 {
				t.Errorf("expected TailLines=500, got %v", opts.TailLines)
			}
			return io.NopCloser(strings.NewReader("")), nil
		},
	}

	handler := NewHandler(streamer, testLogger(), WithInitialTailLines(500))
	mux := http.NewServeMux()
	mux.Handle("GET /api/logs/{namespace}/{pod}", handler)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	conn, _ := dialWS(t, srv, "/api/logs/default/my-pod")
	defer conn.CloseNow()

	// Give the handler time to call StreamPodLogs
	time.Sleep(100 * time.Millisecond)
}

func TestHandler_LargeLogLine(t *testing.T) {
	// Test that large lines (up to 1MB) are handled
	largeLine := strings.Repeat("x", 100000) // 100KB line
	streamer := &fakePodLogStreamer{
		streamFunc: func(ctx context.Context, namespace, pod string, opts *corev1.PodLogOptions) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(largeLine + "\n")), nil
		},
	}

	handler := NewHandler(streamer, testLogger())
	mux := http.NewServeMux()
	mux.Handle("GET /api/logs/{namespace}/{pod}", handler)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	conn, _ := dialWS(t, srv, "/api/logs/default/my-pod")
	defer conn.CloseNow()
	conn.SetReadLimit(2 * 1024 * 1024) // 2MB read limit for test client

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read large line: %v", err)
	}
	if len(data) != 100000 {
		t.Errorf("expected 100000 bytes, got %d", len(data))
	}
}

// Ensure the connection correctly writes text frames (not binary).
func TestHandler_TextFrames(t *testing.T) {
	streamer := &fakePodLogStreamer{
		streamFunc: func(ctx context.Context, namespace, pod string, opts *corev1.PodLogOptions) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("hello\n")), nil
		},
	}

	handler := NewHandler(streamer, testLogger())
	mux := http.NewServeMux()
	mux.Handle("GET /api/logs/{namespace}/{pod}", handler)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	conn, _ := dialWS(t, srv, "/api/logs/default/my-pod")
	defer conn.CloseNow()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	msgType, _, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if msgType != websocket.MessageText {
		t.Errorf("expected text frame, got %v", msgType)
	}
}

// Verify unused but exported helper
var _ = bytes.NewReader // keep import
