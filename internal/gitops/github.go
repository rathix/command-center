package gitops

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

const defaultGitHubBaseURL = "https://api.github.com"

// Sentinel error types for GitHub API errors.
var (
	errRateLimited  = errors.New("GitHub API rate limited")
	errUnauthorized = errors.New("GitHub token is invalid or expired")
	errNotFound     = errors.New("resource not found")
	errNetwork      = errors.New("unable to reach GitHub API")
)

// GitHubAPIError wraps a GitHub API error with context.
type GitHubAPIError struct {
	Err        error
	StatusCode int
	Message    string
}

func (e *GitHubAPIError) Error() string {
	return fmt.Sprintf("%v: %s (HTTP %d)", e.Err, e.Message, e.StatusCode)
}

func (e *GitHubAPIError) Unwrap() error {
	return e.Err
}

// IsRateLimitError checks if the error is a rate limit error.
func IsRateLimitError(err error) bool {
	return errors.Is(err, errRateLimited)
}

// IsAuthError checks if the error is an authentication error.
func IsAuthError(err error) bool {
	return errors.Is(err, errUnauthorized)
}

// IsNotFoundError checks if the error is a not-found error.
func IsNotFoundError(err error) bool {
	return errors.Is(err, errNotFound)
}

// Commit represents a Git commit from the GitHub API.
type Commit struct {
	SHA       string    `json:"sha"`
	Message   string    `json:"message"`
	Author    string    `json:"author"`
	Timestamp time.Time `json:"timestamp"`
}

// RepositoryClient defines the interface for interacting with a Git repository.
type RepositoryClient interface {
	ListCommits(ctx context.Context, owner, repo, branch string, since time.Time, limit int) ([]Commit, error)
	RevertCommit(ctx context.Context, owner, repo, branch, sha string) (*Commit, error)
}

// GitHubClient implements RepositoryClient using the GitHub REST API.
type GitHubClient struct {
	httpClient  *http.Client
	token       string
	logger      *slog.Logger
	rateLimiter *RateLimiter
	baseURL     string
}

// GitHubOption configures a GitHubClient.
type GitHubOption func(*GitHubClient)

// WithBaseURL sets a custom base URL (for testing).
func WithBaseURL(url string) GitHubOption {
	return func(c *GitHubClient) {
		c.baseURL = url
	}
}

// NewGitHubClient creates a new GitHubClient.
func NewGitHubClient(token string, logger *slog.Logger, opts ...GitHubOption) *GitHubClient {
	c := &GitHubClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		token:       token,
		logger:      logger,
		rateLimiter: NewRateLimiter(10, time.Minute),
		baseURL:     defaultGitHubBaseURL,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// ListCommits lists commits from a GitHub repository.
func (c *GitHubClient) ListCommits(ctx context.Context, owner, repo, branch string, since time.Time, limit int) ([]Commit, error) {
	if !c.rateLimiter.Allow() {
		return nil, &GitHubAPIError{
			Err:        errRateLimited,
			StatusCode: 429,
			Message:    "GitHub API rate limited. Try again later.",
		}
	}

	url := fmt.Sprintf("%s/repos/%s/%s/commits?sha=%s&per_page=%d",
		c.baseURL, owner, repo, branch, limit)
	if !since.IsZero() {
		url += "&since=" + since.Format(time.RFC3339)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &GitHubAPIError{
			Err:     errNetwork,
			Message: "Check network connectivity",
		}
	}
	defer resp.Body.Close()

	if err := c.checkResponse(resp); err != nil {
		return nil, err
	}

	var ghCommits []struct {
		SHA    string `json:"sha"`
		Commit struct {
			Message string `json:"message"`
			Author  struct {
				Name string `json:"name"`
				Date string `json:"date"`
			} `json:"author"`
		} `json:"commit"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ghCommits); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	commits := make([]Commit, 0, len(ghCommits))
	for _, gc := range ghCommits {
		ts, _ := time.Parse(time.RFC3339, gc.Commit.Author.Date)
		commits = append(commits, Commit{
			SHA:       gc.SHA,
			Message:   gc.Commit.Message,
			Author:    gc.Commit.Author.Name,
			Timestamp: ts,
		})
	}

	return commits, nil
}

// RevertCommit creates a revert of the specified commit by:
// 1. Getting the commit details and parent SHA
// 2. Getting the parent commit's tree
// 3. Getting the current HEAD of the branch
// 4. Creating a new commit with the parent's tree on top of HEAD
// 5. Updating the branch ref to point to the new commit
func (c *GitHubClient) RevertCommit(ctx context.Context, owner, repo, branch, sha string) (*Commit, error) {
	if !c.rateLimiter.Allow() {
		return nil, &GitHubAPIError{
			Err:        errRateLimited,
			StatusCode: 429,
			Message:    "GitHub API rate limited. Try again later.",
		}
	}

	c.logger.Info("gitops rollback initiated", "sha", sha, "repository", owner+"/"+repo, "branch", branch)

	// Step 1: Get commit details and parent SHA
	commitData, err := c.apiGet(ctx, fmt.Sprintf("%s/repos/%s/%s/commits/%s", c.baseURL, owner, repo, sha))
	if err != nil {
		return nil, fmt.Errorf("step 1 (get commit): %w", err)
	}

	parents, ok := commitData["parents"].([]interface{})
	if !ok || len(parents) == 0 {
		return nil, fmt.Errorf("step 1: commit %s has no parents", sha)
	}
	parentMap, ok := parents[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("step 1: invalid parent format")
	}
	parentSHA, ok := parentMap["sha"].(string)
	if !ok {
		return nil, fmt.Errorf("step 1: parent SHA not found")
	}

	commitInfo, _ := commitData["commit"].(map[string]interface{})
	originalMessage := ""
	if commitInfo != nil {
		originalMessage, _ = commitInfo["message"].(string)
	}

	// Step 2: Get parent commit tree
	if !c.rateLimiter.Allow() {
		return nil, &GitHubAPIError{Err: errRateLimited, StatusCode: 429, Message: "rate limited during rollback"}
	}
	parentCommit, err := c.apiGet(ctx, fmt.Sprintf("%s/repos/%s/%s/git/commits/%s", c.baseURL, owner, repo, parentSHA))
	if err != nil {
		return nil, fmt.Errorf("step 2 (get parent tree): %w", err)
	}

	treeMap, ok := parentCommit["tree"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("step 2: tree not found in parent commit")
	}
	treeSHA, ok := treeMap["sha"].(string)
	if !ok {
		return nil, fmt.Errorf("step 2: tree SHA not found")
	}

	// Step 3: Get current HEAD of branch
	if !c.rateLimiter.Allow() {
		return nil, &GitHubAPIError{Err: errRateLimited, StatusCode: 429, Message: "rate limited during rollback"}
	}
	refData, err := c.apiGet(ctx, fmt.Sprintf("%s/repos/%s/%s/git/refs/heads/%s", c.baseURL, owner, repo, branch))
	if err != nil {
		return nil, fmt.Errorf("step 3 (get branch HEAD): %w", err)
	}

	refObj, ok := refData["object"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("step 3: ref object not found")
	}
	headSHA, ok := refObj["sha"].(string)
	if !ok {
		return nil, fmt.Errorf("step 3: HEAD SHA not found")
	}

	// Step 4: Create revert commit
	if !c.rateLimiter.Allow() {
		return nil, &GitHubAPIError{Err: errRateLimited, StatusCode: 429, Message: "rate limited during rollback"}
	}
	revertMessage := fmt.Sprintf("Revert %q\n\nThis reverts commit %s.", originalMessage, sha)
	createBody := map[string]interface{}{
		"message": revertMessage,
		"tree":    treeSHA,
		"parents": []string{headSHA},
	}
	newCommit, err := c.apiPost(ctx, fmt.Sprintf("%s/repos/%s/%s/git/commits", c.baseURL, owner, repo), createBody)
	if err != nil {
		return nil, fmt.Errorf("step 4 (create revert commit): %w", err)
	}

	newCommitSHA, ok := newCommit["sha"].(string)
	if !ok {
		return nil, fmt.Errorf("step 4: new commit SHA not found")
	}

	// Step 5: Update branch ref
	if !c.rateLimiter.Allow() {
		return nil, &GitHubAPIError{Err: errRateLimited, StatusCode: 429, Message: "rate limited during rollback"}
	}
	_, err = c.apiPatch(ctx, fmt.Sprintf("%s/repos/%s/%s/git/refs/heads/%s", c.baseURL, owner, repo, branch), map[string]interface{}{
		"sha": newCommitSHA,
	})
	if err != nil {
		return nil, fmt.Errorf("step 5 (update branch ref): %w", err)
	}

	c.logger.Info("gitops rollback executed", "sha", sha, "revertSha", newCommitSHA, "repository", owner+"/"+repo, "branch", branch)

	return &Commit{
		SHA:     newCommitSHA,
		Message: revertMessage,
	}, nil
}

func (c *GitHubClient) apiGet(ctx context.Context, url string) (map[string]interface{}, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &GitHubAPIError{Err: errNetwork, Message: "Check network connectivity"}
	}
	defer resp.Body.Close()

	if err := c.checkResponse(resp); err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return result, nil
}

func (c *GitHubClient) apiPost(ctx context.Context, url string, body interface{}) (map[string]interface{}, error) {
	return c.apiMutate(ctx, http.MethodPost, url, body)
}

func (c *GitHubClient) apiPatch(ctx context.Context, url string, body interface{}) (map[string]interface{}, error) {
	return c.apiMutate(ctx, http.MethodPatch, url, body)
}

func (c *GitHubClient) apiMutate(ctx context.Context, method, url string, body interface{}) (map[string]interface{}, error) {
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	c.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &GitHubAPIError{Err: errNetwork, Message: "Check network connectivity"}
	}
	defer resp.Body.Close()

	if err := c.checkResponse(resp); err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return result, nil
}

func (c *GitHubClient) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
}

func (c *GitHubClient) checkResponse(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	var body struct {
		Message string `json:"message"`
	}
	json.NewDecoder(resp.Body).Decode(&body)

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return &GitHubAPIError{
			Err:        errUnauthorized,
			StatusCode: resp.StatusCode,
			Message:    "Check GITHUB_TOKEN environment variable",
		}
	case http.StatusForbidden, http.StatusTooManyRequests:
		return &GitHubAPIError{
			Err:        errRateLimited,
			StatusCode: resp.StatusCode,
			Message:    body.Message,
		}
	case http.StatusNotFound:
		return &GitHubAPIError{
			Err:        errNotFound,
			StatusCode: resp.StatusCode,
			Message:    body.Message,
		}
	default:
		return &GitHubAPIError{
			Err:        fmt.Errorf("GitHub API error (HTTP %d)", resp.StatusCode),
			StatusCode: resp.StatusCode,
			Message:    body.Message,
		}
	}
}
