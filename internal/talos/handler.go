package talos

import (
	"encoding/json"
	"net/http"
	"time"
)

type nodesResponse struct {
	Nodes      []NodeHealth `json:"nodes"`
	LastPoll   string       `json:"lastPoll"`
	Error      *string      `json:"error"`
	Stale      bool         `json:"stale"`
	Configured bool         `json:"configured"`
}

type metricsHistoryResponse struct {
	History []NodeMetrics `json:"history"`
}

// NewHandler returns an http.Handler for GET /api/nodes.
// If poller is nil (talos not configured), it returns configured=false.
func NewHandler(poller *Poller) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if poller == nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(nodesResponse{
				Nodes:      nil,
				Configured: false,
			})
			return
		}

		nodes, lastPoll, lastErr := poller.GetNodes()

		resp := nodesResponse{
			Nodes:      nodes,
			Configured: true,
		}

		if !lastPoll.IsZero() {
			resp.LastPoll = lastPoll.Format(time.RFC3339)
		}

		if lastErr != nil {
			errStr := lastErr.Error()
			resp.Error = &errStr
			resp.Stale = true
		} else if !lastPoll.IsZero() && time.Since(lastPoll) > 2*poller.Interval() {
			resp.Stale = true
		}

		if nodes == nil {
			resp.Nodes = []NodeHealth{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
}

// NewMetricsHistoryHandler returns an http.Handler for GET /api/nodes/{name}/metrics.
// It returns the sparkline metrics history for the named node.
func NewMetricsHistoryHandler(poller *Poller) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if poller == nil {
			http.Error(w, `{"error":"talos not configured"}`, http.StatusNotFound)
			return
		}

		nodeName := r.PathValue("name")
		if nodeName == "" {
			http.Error(w, `{"error":"node name required"}`, http.StatusBadRequest)
			return
		}

		history := poller.GetMetricsHistory(nodeName)
		if history == nil {
			history = []NodeMetrics{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(metricsHistoryResponse{
			History: history,
		})
	})
}
