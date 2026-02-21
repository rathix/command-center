package auth

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Token state values surfaced to SSE.
const (
	TokenStateValid      = "valid"
	TokenStateRefreshing = "refreshing"
	TokenStateExpired    = "expired"
	TokenStateError      = "error"
)

// OIDCStatus is a snapshot of provider/token health used for SSE status payloads.
type OIDCStatus struct {
	Connected    bool
	ProviderName string
	TokenState   string
	LastSuccess  *time.Time
}

// cachedToken holds an in-memory OIDC access token and its expiry.
// Never written to disk, logs, or SSE payloads.
type cachedToken struct {
	accessToken string
	expiresAt   time.Time
}

// singleflightCall represents an in-progress token fetch shared across goroutines.
type singleflightCall struct {
	done chan struct{}
	val  *cachedToken
	err  error
}

// singleflightGroup deduplicates concurrent token fetches so only one HTTP
// request is made even when multiple goroutines call GetToken simultaneously.
type singleflightGroup struct {
	mu       sync.Mutex
	inflight *singleflightCall
}

func (g *singleflightGroup) Do(ctx context.Context, fn func() (*cachedToken, error)) (*cachedToken, error) {
	g.mu.Lock()
	if g.inflight != nil {
		call := g.inflight
		g.mu.Unlock()
		select {
		case <-call.done:
			return call.val, call.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	call := &singleflightCall{done: make(chan struct{})}
	g.inflight = call
	g.mu.Unlock()

	call.val, call.err = fn()
	close(call.done)

	g.mu.Lock()
	g.inflight = nil
	g.mu.Unlock()

	return call.val, call.err
}

// OIDCClient acquires and caches OIDC tokens using the client credentials flow.
// It is safe for concurrent use by multiple goroutines.
type OIDCClient struct {
	issuerURL     string
	providerName  string
	clientID      string
	clientSecret  string
	scopes        []string
	tokenEndpoint string // lazily discovered
	httpClient    *http.Client
	mu            sync.Mutex
	cachedToken   *cachedToken
	connected     bool
	tokenState    string
	lastSuccess   *time.Time
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
		providerName: deriveProviderName(issuerURL),
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
		tokenState: TokenStateRefreshing,
		logger:     logger,
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
		c.connected = true
		c.tokenState = TokenStateValid
		c.mu.Unlock()
		return token, nil
	}
	if c.cachedToken != nil {
		if time.Until(c.cachedToken.expiresAt) <= 0 {
			c.connected = false
			c.tokenState = TokenStateExpired
		} else {
			c.connected = true
			c.tokenState = TokenStateRefreshing
		}
	} else {
		c.connected = false
		c.tokenState = TokenStateRefreshing
	}
	c.mu.Unlock()

	// Slow path: fetch with singleflight dedup
	tok, err := c.singleflight.Do(ctx, func() (*cachedToken, error) {
		return c.fetchToken(ctx)
	})
	if err != nil {
		// Waiter cancellation should not mutate global provider status.
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return "", err
		}
		c.mu.Lock()
		c.connected = false
		c.tokenState = TokenStateError
		c.mu.Unlock()
		return "", err
	}

	// Store in cache
	c.mu.Lock()
	c.cachedToken = tok
	c.connected = true
	c.tokenState = TokenStateValid
	lastSuccess := time.Now()
	c.lastSuccess = &lastSuccess
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
	if tokenResp.ExpiresIn <= 0 {
		return nil, fmt.Errorf("oidc token: invalid expires_in value")
	}

	expiresAt := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	c.logger.Info("OIDC token acquired")

	return &cachedToken{
		accessToken: tokenResp.AccessToken,
		expiresAt:   expiresAt,
	}, nil
}

// GetStatus returns a snapshot of current OIDC status for SSE payload generation.
func (c *OIDCClient) GetStatus() *OIDCStatus {
	c.mu.Lock()
	defer c.mu.Unlock()

	tokenState := c.tokenState
	connected := c.connected
	if c.cachedToken != nil && time.Until(c.cachedToken.expiresAt) <= 0 && tokenState != TokenStateRefreshing {
		tokenState = TokenStateExpired
		connected = false
	}

	var lastSuccess *time.Time
	if c.lastSuccess != nil {
		ts := *c.lastSuccess
		lastSuccess = &ts
	}

	return &OIDCStatus{
		Connected:    connected,
		ProviderName: c.providerName,
		TokenState:   tokenState,
		LastSuccess:  lastSuccess,
	}
}

func deriveProviderName(issuerURL string) string {
	u, err := url.Parse(issuerURL)
	if err != nil || u.Hostname() == "" {
		return "OIDC"
	}
	host := strings.ToLower(u.Hostname())
	if strings.Contains(host, "pocketid") {
		return "PocketID"
	}
	return u.Hostname()
}
