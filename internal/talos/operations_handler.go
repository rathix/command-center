package talos

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

type operationResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

type upgradeRequest struct {
	TargetVersion string `json:"targetVersion"`
}

type upgradeInfoResponse struct {
	CurrentVersion string `json:"currentVersion"`
	TargetVersion  string `json:"targetVersion"`
}

// NewOperationsHandler returns an http.Handler for POST /api/talos/{node}/reboot.
func NewOperationsHandler(poller *Poller, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if poller == nil {
			writeJSON(w, http.StatusNotFound, operationResponse{
				Success: false,
				Error:   "talos not configured",
			})
			return
		}

		nodeName := r.PathValue("node")
		if nodeName == "" {
			writeJSON(w, http.StatusBadRequest, operationResponse{
				Success: false,
				Error:   "node name required",
			})
			return
		}

		logger.Info("talos operation", "node", nodeName, "op", "reboot")

		if err := poller.client.Reboot(r.Context(), nodeName); err != nil {
			logger.Info("talos operation failed", "node", nodeName, "op", "reboot", "error", err)
			writeJSON(w, http.StatusBadGateway, operationResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}

		logger.Info("talos operation succeeded", "node", nodeName, "op", "reboot")
		writeJSON(w, http.StatusOK, operationResponse{
			Success: true,
			Message: "Reboot initiated for " + nodeName,
		})
	})
}

// NewUpgradeHandler returns an http.Handler for POST /api/talos/{node}/upgrade.
func NewUpgradeHandler(poller *Poller, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if poller == nil {
			writeJSON(w, http.StatusNotFound, operationResponse{
				Success: false,
				Error:   "talos not configured",
			})
			return
		}

		nodeName := r.PathValue("node")
		if nodeName == "" {
			writeJSON(w, http.StatusBadRequest, operationResponse{
				Success: false,
				Error:   "node name required",
			})
			return
		}

		var req upgradeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TargetVersion == "" {
			writeJSON(w, http.StatusBadRequest, operationResponse{
				Success: false,
				Error:   "targetVersion is required in request body",
			})
			return
		}

		logger.Info("talos operation", "node", nodeName, "op", "upgrade", "targetVersion", req.TargetVersion)

		if err := poller.client.Upgrade(r.Context(), nodeName, req.TargetVersion); err != nil {
			logger.Info("talos operation failed", "node", nodeName, "op", "upgrade", "error", err)
			writeJSON(w, http.StatusBadGateway, operationResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}

		logger.Info("talos operation succeeded", "node", nodeName, "op", "upgrade", "targetVersion", req.TargetVersion)
		writeJSON(w, http.StatusOK, operationResponse{
			Success: true,
			Message: "Upgrade initiated for " + nodeName + " to " + req.TargetVersion,
		})
	})
}

// NewUpgradeInfoHandler returns an http.Handler for GET /api/talos/{node}/upgrade-info.
func NewUpgradeInfoHandler(poller *Poller, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if poller == nil {
			writeJSON(w, http.StatusNotFound, operationResponse{
				Success: false,
				Error:   "talos not configured",
			})
			return
		}

		nodeName := r.PathValue("node")
		if nodeName == "" {
			writeJSON(w, http.StatusBadRequest, operationResponse{
				Success: false,
				Error:   "node name required",
			})
			return
		}

		info, err := poller.client.GetUpgradeInfo(r.Context(), nodeName)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, operationResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}

		writeJSON(w, http.StatusOK, upgradeInfoResponse{
			CurrentVersion: info.CurrentVersion,
			TargetVersion:  info.TargetVersion,
		})
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
