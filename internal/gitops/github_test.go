package gitops

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGitHubClient_ListCommits_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/commits" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("sha") != "main" {
			t.Errorf("expected sha=main, got %s", r.URL.Query().Get("sha"))
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q, want %q", got, "Bearer test-token")
		}
		if got := r.Header.Get("Accept"); got != "application/vnd.github+json" {
			t.Errorf("Accept = %q, want %q", got, "application/vnd.github+json")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{
				"sha": "abc1234567890",
				"commit": map[string]interface{}{
					"message": "Update deployment config",
					"author": map[string]interface{}{
						"name": "Kenny",
						"date": "2026-02-27T10:00:00Z",
					},
				},
			},
			{
				"sha": "def4567890123",
				"commit": map[string]interface{}{
					"message": "Fix typo in README",
					"author": map[string]interface{}{
						"name": "Bot",
						"date": "2026-02-27T09:00:00Z",
					},
				},
			},
		})
	}))
	defer ts.Close()

	rl := NewRateLimiter(10, time.Minute)
	client := NewGitHubClient("test-token", slog.Default(), WithBaseURL(ts.URL))
	client.rateLimiter = rl

	commits, err := client.ListCommits(context.Background(), "owner", "repo", "main", time.Time{}, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(commits))
	}
	if commits[0].SHA != "abc1234567890" {
		t.Errorf("SHA = %q, want %q", commits[0].SHA, "abc1234567890")
	}
	if commits[0].Message != "Update deployment config" {
		t.Errorf("Message = %q, want %q", commits[0].Message, "Update deployment config")
	}
	if commits[0].Author != "Kenny" {
		t.Errorf("Author = %q, want %q", commits[0].Author, "Kenny")
	}
	expected := time.Date(2026, 2, 27, 10, 0, 0, 0, time.UTC)
	if !commits[0].Timestamp.Equal(expected) {
		t.Errorf("Timestamp = %v, want %v", commits[0].Timestamp, expected)
	}
}

func TestGitHubClient_ListCommits_RateLimited(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]string{"message": "API rate limit exceeded"})
	}))
	defer ts.Close()

	rl := NewRateLimiter(10, time.Minute)
	client := NewGitHubClient("test-token", slog.Default(), WithBaseURL(ts.URL))
	client.rateLimiter = rl

	_, err := client.ListCommits(context.Background(), "owner", "repo", "main", time.Time{}, 20)
	if err == nil {
		t.Fatal("expected error for rate limit")
	}
	if !IsRateLimitError(err) {
		t.Errorf("expected rate limit error, got: %v", err)
	}
}

func TestGitHubClient_ListCommits_Unauthorized(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"message": "Bad credentials"})
	}))
	defer ts.Close()

	rl := NewRateLimiter(10, time.Minute)
	client := NewGitHubClient("bad-token", slog.Default(), WithBaseURL(ts.URL))
	client.rateLimiter = rl

	_, err := client.ListCommits(context.Background(), "owner", "repo", "main", time.Time{}, 20)
	if err == nil {
		t.Fatal("expected error for unauthorized")
	}
	if !IsAuthError(err) {
		t.Errorf("expected auth error, got: %v", err)
	}
}

func TestGitHubClient_ListCommits_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"message": "Not Found"})
	}))
	defer ts.Close()

	rl := NewRateLimiter(10, time.Minute)
	client := NewGitHubClient("test-token", slog.Default(), WithBaseURL(ts.URL))
	client.rateLimiter = rl

	_, err := client.ListCommits(context.Background(), "owner", "repo", "main", time.Time{}, 20)
	if err == nil {
		t.Fatal("expected error for not found")
	}
	if !IsNotFoundError(err) {
		t.Errorf("expected not found error, got: %v", err)
	}
}

func TestGitHubClient_RevertCommit_Success(t *testing.T) {
	step := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		step++
		w.Header().Set("Content-Type", "application/json")
		switch {
		case step == 1 && r.Method == "GET" && r.URL.Path == "/repos/owner/repo/commits/abc123":
			// Step 1: Get commit details
			json.NewEncoder(w).Encode(map[string]interface{}{
				"sha": "abc123",
				"commit": map[string]interface{}{
					"message": "bad deploy",
				},
				"parents": []map[string]interface{}{
					{"sha": "parent111"},
				},
			})
		case step == 2 && r.Method == "GET" && r.URL.Path == "/repos/owner/repo/git/commits/parent111":
			// Step 2: Get parent commit tree
			json.NewEncoder(w).Encode(map[string]interface{}{
				"sha": "parent111",
				"tree": map[string]interface{}{
					"sha": "tree222",
				},
			})
		case step == 3 && r.Method == "GET" && r.URL.Path == "/repos/owner/repo/git/refs/heads/main":
			// Step 3: Get current HEAD
			json.NewEncoder(w).Encode(map[string]interface{}{
				"object": map[string]interface{}{
					"sha": "head333",
				},
			})
		case step == 4 && r.Method == "POST" && r.URL.Path == "/repos/owner/repo/git/commits":
			// Step 4: Create revert commit
			json.NewEncoder(w).Encode(map[string]interface{}{
				"sha": "revert444",
				"message": "Revert \"bad deploy\"\n\nThis reverts commit abc123.",
			})
		case step == 5 && r.Method == "PATCH" && r.URL.Path == "/repos/owner/repo/git/refs/heads/main":
			// Step 5: Update branch ref
			json.NewEncoder(w).Encode(map[string]interface{}{
				"object": map[string]interface{}{
					"sha": "revert444",
				},
			})
		default:
			t.Errorf("unexpected request step=%d method=%s path=%s", step, r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	rl := NewRateLimiter(10, time.Minute)
	client := NewGitHubClient("test-token", slog.Default(), WithBaseURL(ts.URL))
	client.rateLimiter = rl

	result, err := client.RevertCommit(context.Background(), "owner", "repo", "main", "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.SHA != "revert444" {
		t.Errorf("SHA = %q, want %q", result.SHA, "revert444")
	}
}

func TestGitHubClient_RevertCommit_RateLimited(t *testing.T) {
	// Exhaust rate limiter
	rl := NewRateLimiter(0, time.Minute)
	client := NewGitHubClient("test-token", slog.Default(), WithBaseURL("http://unused"))
	client.rateLimiter = rl

	_, err := client.RevertCommit(context.Background(), "owner", "repo", "main", "abc123")
	if err == nil {
		t.Fatal("expected error for rate limit")
	}
	if !IsRateLimitError(err) {
		t.Errorf("expected rate limit error, got: %v", err)
	}
}

func TestGitHubClient_RevertCommit_Unauthorized(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"message": "Bad credentials"})
	}))
	defer ts.Close()

	rl := NewRateLimiter(10, time.Minute)
	client := NewGitHubClient("bad-token", slog.Default(), WithBaseURL(ts.URL))
	client.rateLimiter = rl

	_, err := client.RevertCommit(context.Background(), "owner", "repo", "main", "abc123")
	if err == nil {
		t.Fatal("expected error for unauthorized")
	}
	if !IsAuthError(err) {
		t.Errorf("expected auth error, got: %v", err)
	}
}

func TestGitHubClient_RevertCommit_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"message": "Not Found"})
	}))
	defer ts.Close()

	rl := NewRateLimiter(10, time.Minute)
	client := NewGitHubClient("test-token", slog.Default(), WithBaseURL(ts.URL))
	client.rateLimiter = rl

	_, err := client.RevertCommit(context.Background(), "owner", "repo", "main", "abc123")
	if err == nil {
		t.Fatal("expected error for not found")
	}
	if !IsNotFoundError(err) {
		t.Errorf("expected not found error, got: %v", err)
	}
}

func TestGitHubClient_RevertCommit_PartialFailure(t *testing.T) {
	step := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		step++
		w.Header().Set("Content-Type", "application/json")
		if step == 1 {
			// Step 1 succeeds
			json.NewEncoder(w).Encode(map[string]interface{}{
				"sha": "abc123",
				"commit": map[string]interface{}{
					"message": "bad deploy",
				},
				"parents": []map[string]interface{}{
					{"sha": "parent111"},
				},
			})
		} else {
			// Step 2 fails
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"message": "Internal Server Error"})
		}
	}))
	defer ts.Close()

	rl := NewRateLimiter(10, time.Minute)
	client := NewGitHubClient("test-token", slog.Default(), WithBaseURL(ts.URL))
	client.rateLimiter = rl

	_, err := client.RevertCommit(context.Background(), "owner", "repo", "main", "abc123")
	if err == nil {
		t.Fatal("expected error for partial failure")
	}
	// Should contain context about which step failed
	errMsg := err.Error()
	if errMsg == "" {
		t.Error("expected non-empty error message")
	}
}

func TestGitHubClient_ListCommits_IncludesAuthHeader(t *testing.T) {
	var gotHeader string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]interface{}{})
	}))
	defer ts.Close()

	rl := NewRateLimiter(10, time.Minute)
	client := NewGitHubClient("my-secret-token", slog.Default(), WithBaseURL(ts.URL))
	client.rateLimiter = rl

	_, err := client.ListCommits(context.Background(), "owner", "repo", "main", time.Time{}, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotHeader != "Bearer my-secret-token" {
		t.Errorf("Authorization header = %q, want %q", gotHeader, "Bearer my-secret-token")
	}
}
