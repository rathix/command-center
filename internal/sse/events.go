package sse

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rathix/command-center/internal/state"
)

// StateEventPayload wraps the full service list for the initial "state" event.
type StateEventPayload struct {
	AppVersion string          `json:"appVersion"`
	Services   []state.Service `json:"services"`
}

// DiscoveredEventPayload is the JSON payload for "discovered" and "update" events.
type DiscoveredEventPayload struct {
	Name            string             `json:"name"`
	DisplayName     string             `json:"displayName"`
	Namespace       string             `json:"namespace"`
	URL             string             `json:"url"`
	Status          state.HealthStatus `json:"status"`
	HTTPCode        *int               `json:"httpCode"`
	ResponseTimeMs  *int64             `json:"responseTimeMs"`
	LastChecked     *time.Time         `json:"lastChecked"`
	LastStateChange *time.Time         `json:"lastStateChange"`
	ErrorSnippet    *string            `json:"errorSnippet"`
}

// RemovedEventPayload contains only the identifier fields for a "removed" event.
type RemovedEventPayload struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

func discoveredEventPayloadFromService(svc state.Service) DiscoveredEventPayload {
	return DiscoveredEventPayload{
		Name:            svc.Name,
		DisplayName:     svc.DisplayName,
		Namespace:       svc.Namespace,
		URL:             svc.URL,
		Status:          svc.Status,
		HTTPCode:        svc.HTTPCode,
		ResponseTimeMs:  svc.ResponseTimeMs,
		LastChecked:     svc.LastChecked,
		LastStateChange: svc.LastStateChange,
		ErrorSnippet:    svc.ErrorSnippet,
	}
}

// formatSSEEvent formats an SSE event with the given type and JSON-encoded data.
func formatSSEEvent(eventType string, data interface{}) ([]byte, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal SSE event data: %w", err)
	}
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "event: %s\ndata: %s\n\n", eventType, jsonData)
	return buf.Bytes(), nil
}

// formatKeepalive returns a SSE keepalive comment.
func formatKeepalive() []byte {
	return []byte(":keepalive\n\n")
}
