package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// WebhookOption configures a WebhookAdapter.
type WebhookOption func(*WebhookAdapter)

// WebhookAdapter delivers notifications via HTTP POST to a webhook URL.
type WebhookAdapter struct {
	name   string
	url    string
	client *http.Client
}

// Compile-time interface check.
var _ Adapter = (*WebhookAdapter)(nil)

// NewWebhookAdapter creates a new webhook adapter.
func NewWebhookAdapter(name, url string, opts ...WebhookOption) *WebhookAdapter {
	w := &WebhookAdapter{
		name: name,
		url:  url,
		client: &http.Client{
			Timeout: 10 * 1e9, // 10s
		},
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

// WithHTTPClient sets the HTTP client used by the webhook adapter.
func WithHTTPClient(c *http.Client) WebhookOption {
	return func(w *WebhookAdapter) {
		w.client = c
	}
}

// Name returns the adapter name.
func (w *WebhookAdapter) Name() string { return w.name }

// Send POSTs the notification as JSON to the webhook URL.
func (w *WebhookAdapter) Send(ctx context.Context, n Notification) error {
	body, err := json.Marshal(n)
	if err != nil {
		return fmt.Errorf("webhook %s: marshal: %w", w.name, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("webhook %s: create request: %w", w.name, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook %s: send: %w", w.name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook %s: non-2xx response: %d", w.name, resp.StatusCode)
	}

	return nil
}
