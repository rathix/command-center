package auth

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// cachedToken holds an in-memory OIDC access token and its expiry.
// Never written to disk, logs, or SSE payloads.
type cachedToken struct {
	accessToken string
	expiresAt   time.Time
}

// singleflightCall represents an in-progress token fetch shared across goroutines.
type singleflightCall struct {
	wg  sync.WaitGroup
	val *cachedToken
	err error
}

// singleflightGroup deduplicates concurrent token fetches so only one HTTP
// request is made even when multiple goroutines call GetToken simultaneously.
type singleflightGroup struct {
	mu       sync.Mutex
	inflight *singleflightCall
}

func (g *singleflightGroup) Do(fn func() (*cachedToken, error)) (*cachedToken, error) {
	g.mu.Lock()
	if g.inflight != nil {
		call := g.inflight
		g.mu.Unlock()
		call.wg.Wait()
		return call.val, call.err
	}
	call := &singleflightCall{}
	call.wg.Add(1)
	g.inflight = call
	g.mu.Unlock()

	call.val, call.err = fn()
	call.wg.Done()

	g.mu.Lock()
	g.inflight = nil
	g.mu.Unlock()

	return call.val, call.err
}

// OIDCClient acquires and caches OIDC tokens using the client credentials flow.
// It is safe for concurrent use by multiple goroutines.
type OIDCClient struct {
	issuerURL     string
	clientID      string
	clientSecret  string
	scopes        []string
	tokenEndpoint string // lazily discovered
	httpClient    *http.Client
	mu            sync.Mutex
	cachedToken   *cachedToken
	logger        *slog.Logger
	singleflight  singleflightGroup
}

// NewOIDCClient creates an OIDCClient. The token endpoint is discovered lazily
// on the first GetToken call. If logger is nil, a no-op logger is used.
func NewOIDCClient(issuerURL, clientID, clientSecret string, scopes []string, logger *slog.Logger) *OIDCClient {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &OIDCClient{
		issuerURL:    issuerURL,
		clientID:     clientID,
		clientSecret: clientSecret,
		scopes:       scopes,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		},
		logger: logger,
	}
}

// refreshThreshold is the window before token expiry when proactive refresh occurs.
const refreshThreshold = 30 * time.Second

// GetToken returns a valid OIDC access token, fetching or refreshing as needed.
// Concurrent calls are deduplicated via singleflight — only one HTTP request is made.
// Errors are never cached; a failed fetch returns the error and the next call retries.
func (c *OIDCClient) GetToken(ctx context.Context) (string, error) {
	// Fast path: check cache under mutex
	c.mu.Lock()
	if c.cachedToken != nil && time.Until(c.cachedToken.expiresAt) > refreshThreshold {
		token := c.cachedToken.accessToken
		c.mu.Unlock()
		return token, nil
	}
	c.mu.Unlock()

	// Slow path: fetch with singleflight dedup
	tok, err := c.singleflight.Do(func() (*cachedToken, error) {
		return c.fetchToken(ctx)
	})
	if err != nil {
		return "", err
	}

	// Store in cache
	c.mu.Lock()
	c.cachedToken = tok
	c.mu.Unlock()

	return tok.accessToken, nil
}

// discoverTokenEndpoint fetches the OIDC discovery document and extracts the token_endpoint.
func (c *OIDCClient) discoverTokenEndpoint(ctx context.Context) error {
	discoveryURL := strings.TrimRight(c.issuerURL, "/") + "/.well-known/openid-configuration"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return fmt.Errorf("oidc discovery: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("oidc discovery: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("oidc discovery: unexpected status %d", resp.StatusCode)
	}

	var doc struct {
		TokenEndpoint string `json:"token_endpoint"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return fmt.Errorf("oidc discovery: %w", err)
	}
	if doc.TokenEndpoint == "" {
		return fmt.Errorf("oidc discovery: token_endpoint not found in response")
	}

	c.tokenEndpoint = doc.TokenEndpoint
	return nil
}

// fetchToken performs a client_credentials grant to obtain a new access token.
// It calls discoverTokenEndpoint lazily if the endpoint is not yet known.
func (c *OIDCClient) fetchToken(ctx context.Context) (*cachedToken, error) {
	if c.tokenEndpoint == "" {
		if err := c.discoverTokenEndpoint(ctx); err != nil {
			c.logger.Warn("OIDC token acquisition failed", "error", err)
			return nil, err
		}
	}

	formData := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
	}
	if len(c.scopes) > 0 {
		formData.Set("scope", strings.Join(c.scopes, " "))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenEndpoint, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("oidc token: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Warn("OIDC token acquisition failed", "error", err)
		return nil, fmt.Errorf("oidc token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.logger.Warn("OIDC token acquisition failed", "error", fmt.Sprintf("status %d", resp.StatusCode))
		return nil, fmt.Errorf("oidc token: unexpected status %d", resp.StatusCode)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("oidc token: %w", err)
	}
	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("oidc token: empty access_token in response")
	}

	expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	c.logger.Info("OIDC token acquired")

	return &cachedToken{
		accessToken: tokenResp.AccessToken,
		expiresAt:   expiresAt,
	}, nil
}
