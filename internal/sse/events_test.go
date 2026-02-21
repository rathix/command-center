package sse

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/rathix/command-center/internal/state"
)

func TestFormatKeepaliveFormat(t *testing.T) {
	got := string(formatKeepalive())
	if got != ":keepalive\n\n" {
		t.Errorf("formatKeepalive() = %q, want %q", got, ":keepalive\n\n")
	}
}

func TestFormatSSEEventFormat(t *testing.T) {
	data, err := formatSSEEvent("test", map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("formatSSEEvent error: %v", err)
	}

	got := string(data)

	// Must start with event line
	if !strings.HasPrefix(got, "event: test\n") {
		t.Errorf("expected 'event: test\\n' prefix, got %q", got)
	}

	// Must contain data line
	if !strings.Contains(got, "data: ") {
		t.Errorf("expected 'data: ' in output, got %q", got)
	}

	// Must end with double newline (SSE event terminator)
	if !strings.HasSuffix(got, "\n\n") {
		t.Errorf("expected trailing '\\n\\n', got %q", got)
	}

	// Data line must be valid JSON
	lines := strings.Split(strings.TrimSpace(got), "\n")
	dataLine := strings.TrimPrefix(lines[1], "data: ")
	var parsed map[string]string
	if err := json.Unmarshal([]byte(dataLine), &parsed); err != nil {
		t.Fatalf("data line is not valid JSON: %v", err)
	}
	if parsed["key"] != "value" {
		t.Errorf("expected key=value, got %q", parsed["key"])
	}
}

func TestFormatSSEEventUnmarshalablePayloadReturnsError(t *testing.T) {
	// Channels cannot be JSON-marshaled
	_, err := formatSSEEvent("test", make(chan int))
	if err == nil {
		t.Error("expected error for unmarshalable payload, got nil")
	}
}

func TestDiscoveredEventPayloadFromServiceAllFields(t *testing.T) {
	now := time.Now()
	code := 200
	respTime := int64(42)
	errSnippet := "timeout"

	svc := state.Service{
		Name:            "web",
		DisplayName:     "Web App",
		Namespace:       "production",
		URL:             "https://web.example.com",
		Status:          state.StatusHealthy,
		HTTPCode:        &code,
		ResponseTimeMs:  &respTime,
		LastChecked:     &now,
		LastStateChange: &now,
		ErrorSnippet:    &errSnippet,
	}

	payload := discoveredEventPayloadFromService(svc)

	if payload.Name != "web" {
		t.Errorf("Name = %q, want %q", payload.Name, "web")
	}
	if payload.DisplayName != "Web App" {
		t.Errorf("DisplayName = %q, want %q", payload.DisplayName, "Web App")
	}
	if payload.Namespace != "production" {
		t.Errorf("Namespace = %q, want %q", payload.Namespace, "production")
	}
	if payload.URL != "https://web.example.com" {
		t.Errorf("URL = %q, want %q", payload.URL, "https://web.example.com")
	}
	if payload.Status != state.StatusHealthy {
		t.Errorf("Status = %q, want %q", payload.Status, state.StatusHealthy)
	}
	if payload.HTTPCode == nil || *payload.HTTPCode != 200 {
		t.Errorf("HTTPCode = %v, want 200", payload.HTTPCode)
	}
	if payload.ResponseTimeMs == nil || *payload.ResponseTimeMs != 42 {
		t.Errorf("ResponseTimeMs = %v, want 42", payload.ResponseTimeMs)
	}
	if payload.ErrorSnippet == nil || *payload.ErrorSnippet != "timeout" {
		t.Errorf("ErrorSnippet = %v, want %q", payload.ErrorSnippet, "timeout")
	}
}

func TestDiscoveredEventPayloadFromServiceNilOptionalFields(t *testing.T) {
	svc := state.Service{
		Name:      "minimal",
		Namespace: "default",
		URL:       "https://minimal.example.com",
		Status:    state.StatusUnknown,
	}

	payload := discoveredEventPayloadFromService(svc)

	if payload.HTTPCode != nil {
		t.Errorf("HTTPCode should be nil, got %v", payload.HTTPCode)
	}
	if payload.ResponseTimeMs != nil {
		t.Errorf("ResponseTimeMs should be nil, got %v", payload.ResponseTimeMs)
	}
	if payload.LastChecked != nil {
		t.Errorf("LastChecked should be nil, got %v", payload.LastChecked)
	}
	if payload.LastStateChange != nil {
		t.Errorf("LastStateChange should be nil, got %v", payload.LastStateChange)
	}
	if payload.ErrorSnippet != nil {
		t.Errorf("ErrorSnippet should be nil, got %v", payload.ErrorSnippet)
	}

	// Verify nil fields serialize to JSON null, not missing
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}
	jsonStr := string(data)
	for _, field := range []string{"httpCode", "responseTimeMs", "lastChecked", "lastStateChange", "errorSnippet"} {
		if !strings.Contains(jsonStr, `"`+field+`":null`) {
			t.Errorf("expected %q:null in JSON, got: %s", field, jsonStr)
		}
	}
}

func TestOIDCStatusJSONCamelCase(t *testing.T) {
	ts := time.Date(2026, 2, 21, 14, 30, 0, 0, time.UTC)
	status := OIDCStatus{
		Connected:    true,
		ProviderName: "PocketID",
		TokenState:   TokenStateValid,
		LastSuccess:  &ts,
	}

	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	jsonStr := string(data)
	for _, field := range []string{"connected", "providerName", "tokenState", "lastSuccess"} {
		if !strings.Contains(jsonStr, `"`+field+`"`) {
			t.Errorf("expected camelCase field %q in JSON, got: %s", field, jsonStr)
		}
	}
}

func TestOIDCStatusJSONRoundTrip(t *testing.T) {
	ts := time.Date(2026, 2, 21, 14, 30, 0, 0, time.UTC)
	original := OIDCStatus{
		Connected:    true,
		ProviderName: "PocketID",
		TokenState:   TokenStateValid,
		LastSuccess:  &ts,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var roundTrip OIDCStatus
	if err := json.Unmarshal(data, &roundTrip); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if roundTrip.Connected != original.Connected {
		t.Errorf("Connected = %v, want %v", roundTrip.Connected, original.Connected)
	}
	if roundTrip.ProviderName != original.ProviderName {
		t.Errorf("ProviderName = %q, want %q", roundTrip.ProviderName, original.ProviderName)
	}
	if roundTrip.TokenState != original.TokenState {
		t.Errorf("TokenState = %q, want %q", roundTrip.TokenState, original.TokenState)
	}
	if roundTrip.LastSuccess == nil || !roundTrip.LastSuccess.Equal(*original.LastSuccess) {
		t.Errorf("LastSuccess = %v, want %v", roundTrip.LastSuccess, original.LastSuccess)
	}
}

func TestStateEventPayloadOIDCStatusOmittedWhenNil(t *testing.T) {
	payload := StateEventPayload{
		AppVersion:   "v1.0.0",
		Services:     []state.Service{},
		ConfigErrors: []string{},
		OIDCStatus:   nil,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	jsonStr := string(data)
	if strings.Contains(jsonStr, `"oidcStatus"`) {
		t.Errorf("expected oidcStatus to be omitted when nil, got: %s", jsonStr)
	}
}

func TestServiceAuthMethodOmittedWhenEmpty(t *testing.T) {
	svc := state.Service{
		Name:       "svc",
		Namespace:  "default",
		URL:        "https://svc.example.com",
		Status:     state.StatusHealthy,
		AuthMethod: "",
	}

	data, err := json.Marshal(svc)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	jsonStr := string(data)
	if strings.Contains(jsonStr, `"authMethod"`) {
		t.Errorf("expected authMethod to be omitted when empty, got: %s", jsonStr)
	}
}

func TestServiceAuthMethodPresentWhenSet(t *testing.T) {
	svc := state.Service{
		Name:       "svc",
		Namespace:  "default",
		URL:        "https://svc.example.com",
		Status:     state.StatusHealthy,
		AuthMethod: "oidc",
	}

	data, err := json.Marshal(svc)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	jsonStr := string(data)
	if !strings.Contains(jsonStr, `"authMethod":"oidc"`) {
		t.Errorf("expected authMethod:oidc in JSON, got: %s", jsonStr)
	}
}
