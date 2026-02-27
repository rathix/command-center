package gitops

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rathix/command-center/internal/config"
)

type mockRepoClient struct {
	commits []Commit
	err     error
}

func (m *mockRepoClient) ListCommits(_ context.Context, _, _, _ string, _ time.Time, _ int) ([]Commit, error) {
	return m.commits, m.err
}

func (m *mockRepoClient) RevertCommit(_ context.Context, _, _, _, _ string) (*Commit, error) {
	return nil, nil
}

func TestStatusHandler_Configured(t *testing.T) {
	cfg := &config.GitOpsConfig{
		Provider:   "github",
		Repository: "owner/repo",
	}
	handler := StatusHandler(cfg, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/api/gitops/status", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp envelope
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.OK {
		t.Error("expected ok=true")
	}
	data, _ := json.Marshal(resp.Data)
	var status GitOpsStatusResponse
	json.Unmarshal(data, &status)
	if !status.Configured {
		t.Error("expected configured=true")
	}
	if status.Provider != "github" {
		t.Errorf("provider = %q, want %q", status.Provider, "github")
	}
}

func TestStatusHandler_NotConfigured(t *testing.T) {
	handler := StatusHandler(nil, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/api/gitops/status", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp envelope
	json.Unmarshal(w.Body.Bytes(), &resp)
	data, _ := json.Marshal(resp.Data)
	var status GitOpsStatusResponse
	json.Unmarshal(data, &status)
	if status.Configured {
		t.Error("expected configured=false")
	}
}

func TestCommitsHandler_Success(t *testing.T) {
	cfg := &config.GitOpsConfig{
		Provider:   "github",
		Repository: "owner/repo",
		Branch:     "main",
	}
	client := &mockRepoClient{
		commits: []Commit{
			{SHA: "abc123", Message: "test commit", Author: "Kenny", Timestamp: time.Now()},
		},
	}
	handler := CommitsHandler(cfg, client, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/api/gitops/commits", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var resp envelope
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.OK {
		t.Error("expected ok=true")
	}
}

func TestCommitsHandler_NotConfigured(t *testing.T) {
	handler := CommitsHandler(nil, nil, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/api/gitops/commits", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestCommitsHandler_RateLimit(t *testing.T) {
	cfg := &config.GitOpsConfig{
		Provider:   "github",
		Repository: "owner/repo",
		Branch:     "main",
	}
	client := &mockRepoClient{
		err: &GitHubAPIError{Err: errRateLimited, StatusCode: 429, Message: "rate limited"},
	}
	handler := CommitsHandler(cfg, client, slog.Default())

	req := httptest.NewRequest(http.MethodGet, "/api/gitops/commits", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", w.Code)
	}
}

type mockRevertClient struct {
	mockRepoClient
	revertCommit *Commit
	revertErr    error
}

func (m *mockRevertClient) RevertCommit(_ context.Context, _, _, _, _ string) (*Commit, error) {
	return m.revertCommit, m.revertErr
}

func TestRollbackHandler_Success(t *testing.T) {
	cfg := &config.GitOpsConfig{
		Provider:   "github",
		Repository: "owner/repo",
		Branch:     "main",
	}
	client := &mockRevertClient{
		revertCommit: &Commit{SHA: "revert456", Message: "Revert \"bad deploy\""},
	}
	handler := RollbackHandler(cfg, client, slog.Default())

	body := `{"sha":"abc123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/gitops/rollback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", w.Code, w.Body.String())
	}

	var resp envelope
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.OK {
		t.Error("expected ok=true")
	}
}

func TestRollbackHandler_NoConfig(t *testing.T) {
	handler := RollbackHandler(nil, nil, slog.Default())

	body := `{"sha":"abc123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/gitops/rollback", strings.NewReader(body))
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestRollbackHandler_InvalidBody(t *testing.T) {
	cfg := &config.GitOpsConfig{
		Provider:   "github",
		Repository: "owner/repo",
		Branch:     "main",
	}
	client := &mockRevertClient{}
	handler := RollbackHandler(cfg, client, slog.Default())

	// Missing SHA
	body := `{"sha":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/gitops/rollback", strings.NewReader(body))
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestRollbackHandler_RateLimit(t *testing.T) {
	cfg := &config.GitOpsConfig{
		Provider:   "github",
		Repository: "owner/repo",
		Branch:     "main",
	}
	client := &mockRevertClient{
		revertErr: &GitHubAPIError{Err: errRateLimited, StatusCode: 429, Message: "rate limited"},
	}
	handler := RollbackHandler(cfg, client, slog.Default())

	body := `{"sha":"abc123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/gitops/rollback", strings.NewReader(body))
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", w.Code)
	}
}

func TestParseOwnerRepo(t *testing.T) {
	tests := []struct {
		input     string
		wantOwner string
		wantRepo  string
	}{
		{"owner/repo", "owner", "repo"},
		{"my-org/my-repo", "my-org", "my-repo"},
		{"invalid", "", ""},
		{"", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			owner, repo := parseOwnerRepo(tt.input)
			if owner != tt.wantOwner || repo != tt.wantRepo {
				t.Errorf("parseOwnerRepo(%q) = (%q, %q), want (%q, %q)",
					tt.input, owner, repo, tt.wantOwner, tt.wantRepo)
			}
		})
	}
}
