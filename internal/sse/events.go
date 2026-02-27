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
	AppVersion            string          `json:"appVersion"`
	Services              []state.Service `json:"services"`
	K8sConnected          bool            `json:"k8sConnected"`
	K8sLastEvent          *time.Time      `json:"k8sLastEvent"`
	HealthCheckIntervalMs int             `json:"healthCheckIntervalMs"`
	ConfigErrors          []string        `json:"configErrors"`
}

// K8sStatusPayload is the JSON payload for "k8sStatus" events.
type K8sStatusPayload struct {
	K8sConnected bool   `json:"k8sConnected"`
	K8sLastEvent string `json:"k8sLastEvent"`
}

// DiscoveredEventPayload is the JSON payload for "discovered" and "update" events.
type DiscoveredEventPayload struct {
	Name            string             `json:"name"`
	DisplayName     string             `json:"displayName"`
	Namespace       string             `json:"namespace"`
	Group           string             `json:"group"`
	URL             string             `json:"url"`
	Icon            string             `json:"icon,omitempty"`
	Source          string             `json:"source"`
	Status          state.HealthStatus   `json:"status"`
	CompositeStatus state.HealthStatus   `json:"compositeStatus"`
	AuthGuarded     bool                 `json:"authGuarded"`
	HTTPCode        *int                 `json:"httpCode"`
	ResponseTimeMs  *int64             `json:"responseTimeMs"`
	LastChecked     *time.Time         `json:"lastChecked"`
	LastStateChange *time.Time         `json:"lastStateChange"`
	ErrorSnippet    *string            `json:"errorSnippet"`
	ReadyEndpoints  *int                 `json:"readyEndpoints"`
	TotalEndpoints  *int                 `json:"totalEndpoints"`
	PodDiagnostic   *state.PodDiagnostic `json:"podDiagnostic"`
	GitOpsStatus    *state.GitOpsStatus  `json:"gitopsStatus"`
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
		Group:           svc.Group,
		URL:             svc.URL,
		Icon:            svc.Icon,
		Source:          svc.Source,
		Status:          svc.Status,
		CompositeStatus: svc.CompositeStatus,
		AuthGuarded:     svc.AuthGuarded,
		HTTPCode:        svc.HTTPCode,
		ResponseTimeMs:  svc.ResponseTimeMs,
		LastChecked:     svc.LastChecked,
		LastStateChange: svc.LastStateChange,
		ErrorSnippet:    svc.ErrorSnippet,
		ReadyEndpoints:  svc.ReadyEndpoints,
		TotalEndpoints:  svc.TotalEndpoints,
		PodDiagnostic:   svc.PodDiagnostic,
		GitOpsStatus:    svc.GitOpsStatus,
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
