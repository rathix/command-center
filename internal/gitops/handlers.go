package gitops

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/rathix/command-center/internal/config"
)

// envelope wraps API responses in {ok, data} or {ok, error} format.
type envelope struct {
	OK    bool        `json:"ok"`
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
}

// GitOpsStatusResponse is the data payload for GET /api/gitops/status.
type GitOpsStatusResponse struct {
	Configured bool   `json:"configured"`
	Provider   string `json:"provider,omitempty"`
	Repository string `json:"repository,omitempty"`
}

// CommitsResponse is the data payload for GET /api/gitops/commits.
type CommitsResponse struct {
	Commits []Commit `json:"commits"`
}

// StatusHandler returns a handler for GET /api/gitops/status.
func StatusHandler(cfg *config.GitOpsConfig, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if cfg == nil {
			json.NewEncoder(w).Encode(envelope{
				OK:   true,
				Data: GitOpsStatusResponse{Configured: false},
			})
			return
		}

		json.NewEncoder(w).Encode(envelope{
			OK: true,
			Data: GitOpsStatusResponse{
				Configured: true,
				Provider:   cfg.Provider,
				Repository: cfg.Repository,
			},
		})
	}
}

// CommitsHandler returns a handler for GET /api/gitops/commits.
func CommitsHandler(cfg *config.GitOpsConfig, client RepositoryClient, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if cfg == nil || client == nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(envelope{
				OK:    false,
				Error: "GitOps is not configured",
			})
			return
		}

		// Parse owner/repo from config
		owner, repo := parseOwnerRepo(cfg.Repository)
		if owner == "" || repo == "" {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(envelope{
				OK:    false,
				Error: "invalid repository format in config",
			})
			return
		}

		// Lookback window: 24 hours of commits
		since := time.Now().Add(-24 * time.Hour)
		commits, err := client.ListCommits(r.Context(), owner, repo, cfg.Branch, since, 20)
		if err != nil {
			if IsRateLimitError(err) {
				w.WriteHeader(http.StatusTooManyRequests)
				json.NewEncoder(w).Encode(envelope{
					OK:    false,
					Error: "GitHub API rate limited. Try again later.",
				})
				return
			}
			logger.Warn("failed to list commits", "error", err)
			w.WriteHeader(http.StatusBadGateway)
			json.NewEncoder(w).Encode(envelope{
				OK:    false,
				Error: "failed to fetch commits from GitHub",
			})
			return
		}

		json.NewEncoder(w).Encode(envelope{
			OK:   true,
			Data: CommitsResponse{Commits: commits},
		})
	}
}

// RollbackRequest is the request body for POST /api/gitops/rollback.
type RollbackRequest struct {
	SHA string `json:"sha"`
}

// RollbackResponse is the data payload for a successful rollback.
type RollbackResponse struct {
	RevertSHA string `json:"revertSha"`
	Message   string `json:"message"`
}

// RollbackHandler returns a handler for POST /api/gitops/rollback.
func RollbackHandler(cfg *config.GitOpsConfig, client RepositoryClient, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if cfg == nil || client == nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(envelope{
				OK:    false,
				Error: "GitOps is not configured",
			})
			return
		}

		var req RollbackRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(envelope{
				OK:    false,
				Error: "invalid request body",
			})
			return
		}

		if req.SHA == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(envelope{
				OK:    false,
				Error: "sha is required",
			})
			return
		}

		owner, repo := parseOwnerRepo(cfg.Repository)
		if owner == "" || repo == "" {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(envelope{
				OK:    false,
				Error: "invalid repository format in config",
			})
			return
		}

		commit, err := client.RevertCommit(r.Context(), owner, repo, cfg.Branch, req.SHA)
		if err != nil {
			if IsRateLimitError(err) {
				w.WriteHeader(http.StatusTooManyRequests)
				json.NewEncoder(w).Encode(envelope{
					OK:    false,
					Error: "GitHub API rate limited. Try again in a few seconds.",
				})
				return
			}
			if IsAuthError(err) {
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(envelope{
					OK:    false,
					Error: "GitHub token is invalid or expired. Check GITHUB_TOKEN environment variable.",
				})
				return
			}
			if IsNotFoundError(err) {
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(envelope{
					OK:    false,
					Error: fmt.Sprintf("Commit %s not found in %s/%s.", req.SHA, owner, repo),
				})
				return
			}
			logger.Warn("rollback failed", "error", err, "sha", req.SHA)
			w.WriteHeader(http.StatusBadGateway)
			json.NewEncoder(w).Encode(envelope{
				OK:    false,
				Error: err.Error(),
			})
			return
		}

		json.NewEncoder(w).Encode(envelope{
			OK: true,
			Data: RollbackResponse{
				RevertSHA: commit.SHA,
				Message:   commit.Message,
			},
		})
	}
}

// parseOwnerRepo splits "owner/repo" into owner and repo parts.
func parseOwnerRepo(repository string) (string, string) {
	for i, c := range repository {
		if c == '/' {
			return repository[:i], repository[i+1:]
		}
	}
	return "", ""
}
