package sse

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"log/slog"

	"github.com/rathix/command-center/internal/state"
)

// mockStateSource implements StateSource for testing.
type mockStateSource struct {
	services []state.Service
	eventCh  chan state.Event
}

func newMockSource(services []state.Service) *mockStateSource {
	return &mockStateSource{
		services: services,
		eventCh:  make(chan state.Event, 64),
	}
}

func (m *mockStateSource) All() []state.Service {
	return m.services
}

func (m *mockStateSource) Subscribe() <-chan state.Event {
	return m.eventCh
}

// parseSSEEvents reads SSE events from a response body string.
// Returns a slice of (eventType, data) pairs.
func parseSSEEvents(body string) []struct{ eventType, data string } {
	var events []struct{ eventType, data string }
	scanner := bufio.NewScanner(strings.NewReader(body))
	var currentType, currentData string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: ") {
			currentType = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			currentData = strings.TrimPrefix(line, "data: ")
		} else if line == "" && currentType != "" {
			events = append(events, struct{ eventType, data string }{currentType, currentData})
			currentType = ""
			currentData = ""
		}
	}
	return events
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestBrokerInitialStateEvent(t *testing.T) {
	services := []state.Service{
		{Name: "web", Namespace: "default", URL: "https://web.example.com", Status: "unknown"},
		{Name: "api", Namespace: "default", URL: "https://api.example.com", Status: "healthy"},
	}
	source := newMockSource(services)
	broker := NewBroker(source, discardLogger(), "v1.0.0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go broker.Run(ctx)

	// Use httptest server for a real HTTP connection
	ts := httptest.NewServer(broker)
	defer ts.Close()

	resp, err := http.Get(ts.URL)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("expected Content-Type text/event-stream, got %q", ct)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("expected Cache-Control no-cache, got %q", cc)
	}

	// Read the initial state event
	scanner := bufio.NewScanner(resp.Body)
	var eventType, data string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			data = strings.TrimPrefix(line, "data: ")
		} else if line == "" && eventType != "" {
			break
		}
	}

	if eventType != "state" {
		t.Errorf("expected event type 'state', got %q", eventType)
	}

	var payload StateEventPayload
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		t.Fatalf("failed to unmarshal state payload: %v", err)
	}
	if len(payload.Services) != 2 {
		t.Errorf("expected 2 services in initial state, got %d", len(payload.Services))
	}
	if payload.AppVersion != "v1.0.0" {
		t.Errorf("expected appVersion 'v1.0.0', got %q", payload.AppVersion)
	}
}

func TestBrokerDiscoveredBroadcast(t *testing.T) {
	source := newMockSource(nil)
	broker := NewBroker(source, discardLogger(), "v1.0.0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go broker.Run(ctx)

	ts := httptest.NewServer(broker)
	defer ts.Close()

	resp, err := http.Get(ts.URL)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer resp.Body.Close()

	// Drain initial state event
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		if scanner.Text() == "" {
			break // End of initial state event
		}
	}

	// Send a discovered event
	source.eventCh <- state.Event{
		Type:    state.EventDiscovered,
		Service: state.Service{Name: "new-svc", Namespace: "prod", URL: "https://new.example.com", Status: "unknown"},
	}

	// Read the discovered event
	var eventType, data string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			data = strings.TrimPrefix(line, "data: ")
		} else if line == "" && eventType != "" {
			break
		}
	}

	if eventType != "discovered" {
		t.Errorf("expected event type 'discovered', got %q", eventType)
	}

	var payload DiscoveredEventPayload
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		t.Fatalf("failed to unmarshal discovered payload: %v", err)
	}
	if payload.Name != "new-svc" {
		t.Errorf("expected name 'new-svc', got %q", payload.Name)
	}
}

func TestBrokerRemovedBroadcast(t *testing.T) {
	source := newMockSource(nil)
	broker := NewBroker(source, discardLogger(), "v1.0.0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go broker.Run(ctx)

	ts := httptest.NewServer(broker)
	defer ts.Close()

	resp, err := http.Get(ts.URL)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer resp.Body.Close()

	// Drain initial state event
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		if scanner.Text() == "" {
			break
		}
	}

	// Send a removed event
	source.eventCh <- state.Event{
		Type:      state.EventRemoved,
		Namespace: "prod",
		Name:      "old-svc",
	}

	// Read the removed event
	var eventType, data string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			data = strings.TrimPrefix(line, "data: ")
		} else if line == "" && eventType != "" {
			break
		}
	}

	if eventType != "removed" {
		t.Errorf("expected event type 'removed', got %q", eventType)
	}

	var payload RemovedEventPayload
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		t.Fatalf("failed to unmarshal removed payload: %v", err)
	}
	if payload.Name != "old-svc" {
		t.Errorf("expected name 'old-svc', got %q", payload.Name)
	}
	if payload.Namespace != "prod" {
		t.Errorf("expected namespace 'prod', got %q", payload.Namespace)
	}
}

func TestBrokerKeepaliveTiming(t *testing.T) {
	source := newMockSource(nil)
	broker := newBrokerWithKeepalive(source, discardLogger(), "v1.0.0", 40*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go broker.Run(ctx)

	ts := httptest.NewServer(broker)
	defer ts.Close()

	resp, err := http.Get(ts.URL)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer resp.Body.Close()

	// Drain initial state event.
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		if scanner.Text() == "" {
			break
		}
	}

	gotKeepalive := make(chan bool, 1)
	go func() {
		for scanner.Scan() {
			if scanner.Text() == ":keepalive" {
				gotKeepalive <- true
				return
			}
		}
		gotKeepalive <- false
	}()

	select {
	case ok := <-gotKeepalive:
		if !ok {
			t.Fatal("stream ended before keepalive")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for keepalive")
	}
}

func TestBrokerClientDisconnectCleanup(t *testing.T) {
	source := newMockSource(nil)
	broker := NewBroker(source, discardLogger(), "v1.0.0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go broker.Run(ctx)

	ts := httptest.NewServer(broker)
	defer ts.Close()

	// Connect a client
	resp, err := http.Get(ts.URL)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	// Wait for client registration
	time.Sleep(50 * time.Millisecond)

	broker.mu.Lock()
	clientsBefore := len(broker.clients)
	broker.mu.Unlock()

	if clientsBefore != 1 {
		t.Errorf("expected 1 client, got %d", clientsBefore)
	}

	// Disconnect client
	resp.Body.Close()

	// Wait for cleanup
	time.Sleep(100 * time.Millisecond)

	broker.mu.Lock()
	clientsAfter := len(broker.clients)
	broker.mu.Unlock()

	if clientsAfter != 0 {
		t.Errorf("expected 0 clients after disconnect, got %d", clientsAfter)
	}
}

func TestBrokerMultiClientBroadcast(t *testing.T) {
	source := newMockSource(nil)
	broker := NewBroker(source, discardLogger(), "v1.0.0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go broker.Run(ctx)

	ts := httptest.NewServer(broker)
	defer ts.Close()

	const numClients = 3
	responses := make([]*http.Response, numClients)
	scanners := make([]*bufio.Scanner, numClients)

	// Connect multiple clients
	for i := range numClients {
		resp, err := http.Get(ts.URL)
		if err != nil {
			t.Fatalf("client %d: failed to connect: %v", i, err)
		}
		defer resp.Body.Close()
		responses[i] = resp
		scanners[i] = bufio.NewScanner(resp.Body)

		// Drain initial state event
		for scanners[i].Scan() {
			if scanners[i].Text() == "" {
				break
			}
		}
	}

	// Send a discovered event
	source.eventCh <- state.Event{
		Type:    state.EventDiscovered,
		Service: state.Service{Name: "broadcast-svc", Namespace: "default", URL: "https://b.example.com", Status: "unknown"},
	}

	// All clients should receive it
	var wg sync.WaitGroup
	wg.Add(numClients)
	for i := range numClients {
		go func(idx int) {
			defer wg.Done()
			var eventType string
			for scanners[idx].Scan() {
				line := scanners[idx].Text()
				if strings.HasPrefix(line, "event: ") {
					eventType = strings.TrimPrefix(line, "event: ")
				} else if line == "" && eventType != "" {
					break
				}
			}
			if eventType != "discovered" {
				t.Errorf("client %d: expected 'discovered', got %q", idx, eventType)
			}
		}(i)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All clients received
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for all clients to receive broadcast")
	}
}

func TestBrokerGracefulShutdown(t *testing.T) {
	source := newMockSource(nil)
	broker := NewBroker(source, discardLogger(), "v1.0.0")

	ctx, cancel := context.WithCancel(context.Background())
	go broker.Run(ctx)

	ts := httptest.NewServer(broker)
	defer ts.Close()

	// Connect a client
	resp, err := http.Get(ts.URL)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer resp.Body.Close()

	// Wait for client to register
	time.Sleep(50 * time.Millisecond)

	// Cancel context to trigger shutdown
	cancel()

	// The client connection should end (read should return)
	done := make(chan struct{})
	go func() {
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			// drain remaining data
		}
		close(done)
	}()

	select {
	case <-done:
		// Shutdown completed
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for graceful shutdown")
	}

	broker.mu.Lock()
	remainingClients := len(broker.clients)
	broker.mu.Unlock()

	if remainingClients != 0 {
		t.Errorf("expected 0 clients after shutdown, got %d", remainingClients)
	}
}

func TestBrokerSourceChannelClosedStopsBroker(t *testing.T) {
	source := newMockSource(nil)
	broker := NewBroker(source, discardLogger(), "v1.0.0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		broker.Run(ctx)
		close(done)
	}()

	ts := httptest.NewServer(broker)
	defer ts.Close()

	resp, err := http.Get(ts.URL)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer resp.Body.Close()

	time.Sleep(50 * time.Millisecond)
	close(source.eventCh)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("broker did not stop after source channel closed")
	}

	broker.mu.Lock()
	remainingClients := len(broker.clients)
	broker.mu.Unlock()

	if remainingClients != 0 {
		t.Errorf("expected 0 clients after source close, got %d", remainingClients)
	}
}

func TestFormatSSEEvent(t *testing.T) {
	data, err := formatSSEEvent("state", StateEventPayload{
		AppVersion: "v1.2.3",
		Services: []state.Service{
			{Name: "web", Namespace: "default", URL: "https://web.example.com", Status: "unknown"},
		},
	})
	if err != nil {
		t.Fatalf("formatSSEEvent error: %v", err)
	}

	got := string(data)
	if !strings.HasPrefix(got, "event: state\n") {
		t.Errorf("expected event line, got %q", got)
	}
	if !strings.Contains(got, "data: ") {
		t.Errorf("expected data line, got %q", got)
	}
	if !strings.HasSuffix(got, "\n\n") {
		t.Errorf("expected trailing blank line, got %q", got)
	}

	// Verify JSON payload
	lines := strings.Split(strings.TrimSpace(got), "\n")
	dataLine := strings.TrimPrefix(lines[1], "data: ")
	var payload StateEventPayload
	if err := json.Unmarshal([]byte(dataLine), &payload); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(payload.Services) != 1 {
		t.Errorf("expected 1 service, got %d", len(payload.Services))
	}
	if payload.Services[0].Name != "web" {
		t.Errorf("expected name 'web', got %q", payload.Services[0].Name)
	}
}

func TestFormatSSEEventRemovedPayload(t *testing.T) {
	data, err := formatSSEEvent("removed", RemovedEventPayload{
		Name:      "old-svc",
		Namespace: "default",
	})
	if err != nil {
		t.Fatalf("formatSSEEvent error: %v", err)
	}

	got := string(data)
	if !strings.Contains(got, "event: removed\n") {
		t.Errorf("expected 'event: removed', got %q", got)
	}

	lines := strings.Split(strings.TrimSpace(got), "\n")
	dataLine := strings.TrimPrefix(lines[1], "data: ")
	var payload RemovedEventPayload
	if err := json.Unmarshal([]byte(dataLine), &payload); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if payload.Name != "old-svc" {
		t.Errorf("expected name 'old-svc', got %q", payload.Name)
	}
	if payload.Namespace != "default" {
		t.Errorf("expected namespace 'default', got %q", payload.Namespace)
	}
}

func TestStateEventPayloadCamelCaseJSON(t *testing.T) {
	now := time.Now()
	code := 200
	respTime := int64(42)
	errSnippet := "timeout"

	payload := StateEventPayload{
		Services: []state.Service{
			{
				Name:            "web",
				Namespace:       "default",
				URL:             "https://web.example.com",
				Status:          "unknown",
				HTTPCode:        &code,
				ResponseTimeMs:  &respTime,
				LastChecked:     &now,
				LastStateChange: &now,
				ErrorSnippet:    &errSnippet,
			},
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	jsonStr := string(data)
	// Verify camelCase field names from state.Service json tags
	for _, field := range []string{"name", "displayName", "namespace", "url", "status", "httpCode", "responseTimeMs", "lastChecked", "lastStateChange", "errorSnippet"} {
		if !strings.Contains(jsonStr, `"`+field+`"`) {
			t.Errorf("expected camelCase field %q in JSON, got: %s", field, jsonStr)
		}
	}
}

func TestDiscoveredEventPayloadCamelCaseJSON(t *testing.T) {
	now := time.Now()
	code := 200
	respTime := int64(42)
	errSnippet := "timeout"

	payload := discoveredEventPayloadFromService(state.Service{
		Name:            "web",
		DisplayName:     "web",
		Namespace:       "default",
		URL:             "https://web.example.com",
		Status:          "unknown",
		HTTPCode:        &code,
		ResponseTimeMs:  &respTime,
		LastChecked:     &now,
		LastStateChange: &now,
		ErrorSnippet:    &errSnippet,
	})

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	jsonStr := string(data)
	for _, field := range []string{"name", "displayName", "namespace", "url", "status", "httpCode", "responseTimeMs", "lastChecked", "lastStateChange", "errorSnippet"} {
		if !strings.Contains(jsonStr, `"`+field+`"`) {
			t.Errorf("expected camelCase field %q in JSON, got: %s", field, jsonStr)
		}
	}
}

func TestRemovedEventPayloadCamelCaseJSON(t *testing.T) {
	payload := RemovedEventPayload{
		Name:      "svc",
		Namespace: "ns",
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	jsonStr := string(data)
	if !strings.Contains(jsonStr, `"name"`) {
		t.Errorf("expected 'name' field in JSON: %s", jsonStr)
	}
	if !strings.Contains(jsonStr, `"namespace"`) {
		t.Errorf("expected 'namespace' field in JSON: %s", jsonStr)
	}
}
