package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// newMockOIDCProvider creates a test HTTP server that simulates an OIDC provider.
// It serves a discovery document at /.well-known/openid-configuration and a
// token endpoint at /oauth/token. The tokenHandler can be nil for default behavior.
func newMockOIDCProvider(t *testing.T, tokenHandler http.HandlerFunc) *httptest.Server {
	t.Helper()
	var srv *httptest.Server

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"issuer":         srv.URL,
			"token_endpoint": srv.URL + "/oauth/token",
		})
	})

	if tokenHandler != nil {
		mux.HandleFunc("/oauth/token", tokenHandler)
	} else {
		mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"access_token": "mock-access-token",
				"token_type":   "Bearer",
				"expires_in":   3600,
			})
		})
	}

	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// Test 8.1: Successful token acquisition via mock OIDC provider
func TestGetToken_Success(t *testing.T) {
	srv := newMockOIDCProvider(t, nil)

	client := NewOIDCClient(srv.URL, "test-id", "test-secret", []string{"openid"}, nil)

	token, err := client.GetToken(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "mock-access-token" {
		t.Errorf("token = %q, want %q", token, "mock-access-token")
	}
}

// Test 8.2: Token caching — first call fetches, second returns cached
func TestGetToken_Caching(t *testing.T) {
	var fetchCount atomic.Int32

	srv := newMockOIDCProvider(t, func(w http.ResponseWriter, r *http.Request) {
		fetchCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "cached-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	})

	client := NewOIDCClient(srv.URL, "test-id", "test-secret", nil, nil)

	// First call — should fetch
	token1, err := client.GetToken(context.Background())
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}

	// Second call — should return cached
	token2, err := client.GetToken(context.Background())
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}

	if token1 != "cached-token" || token2 != "cached-token" {
		t.Errorf("tokens = %q, %q, want %q", token1, token2, "cached-token")
	}

	if count := fetchCount.Load(); count != 1 {
		t.Errorf("fetch count = %d, want 1 (cached second call)", count)
	}
}

// Test 8.3: Proactive refresh — token with <30s remaining triggers refetch
func TestGetToken_ProactiveRefresh(t *testing.T) {
	var fetchCount atomic.Int32

	srv := newMockOIDCProvider(t, func(w http.ResponseWriter, r *http.Request) {
		count := fetchCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": fmt.Sprintf("token-%d", count),
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	})

	client := NewOIDCClient(srv.URL, "test-id", "test-secret", nil, nil)

	// First fetch
	token1, err := client.GetToken(context.Background())
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}
	if token1 != "token-1" {
		t.Errorf("first token = %q, want %q", token1, "token-1")
	}

	// Manually set the cached token to expire in 20 seconds (below 30s threshold)
	client.mu.Lock()
	client.cachedToken = &cachedToken{
		accessToken: "about-to-expire",
		expiresAt:   time.Now().Add(20 * time.Second),
	}
	client.mu.Unlock()

	// This should trigger a refetch since token is within 30s threshold
	token2, err := client.GetToken(context.Background())
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}
	if token2 != "token-2" {
		t.Errorf("second token = %q, want %q (should have refreshed)", token2, "token-2")
	}

	if count := fetchCount.Load(); count != 2 {
		t.Errorf("fetch count = %d, want 2", count)
	}
}

// Test 8.4: Non-expired token — token with >30s remaining, no refetch
func TestGetToken_NonExpiredNoCacheMiss(t *testing.T) {
	var fetchCount atomic.Int32

	srv := newMockOIDCProvider(t, func(w http.ResponseWriter, r *http.Request) {
		fetchCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "valid-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	})

	client := NewOIDCClient(srv.URL, "test-id", "test-secret", nil, nil)

	// Manually set a valid cached token with plenty of time remaining
	client.mu.Lock()
	client.cachedToken = &cachedToken{
		accessToken: "still-valid",
		expiresAt:   time.Now().Add(60 * time.Second),
	}
	client.mu.Unlock()

	token, err := client.GetToken(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "still-valid" {
		t.Errorf("token = %q, want %q", token, "still-valid")
	}

	if count := fetchCount.Load(); count != 0 {
		t.Errorf("fetch count = %d, want 0 (should use cache)", count)
	}
}

// Test 8.5: Concurrent access — singleflight dedup
func TestGetToken_ConcurrentSingleflight(t *testing.T) {
	var fetchCount atomic.Int32

	srv := newMockOIDCProvider(t, func(w http.ResponseWriter, r *http.Request) {
		fetchCount.Add(1)
		// Small delay to ensure goroutines overlap
		time.Sleep(50 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "shared-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	})

	client := NewOIDCClient(srv.URL, "test-id", "test-secret", nil, nil)

	var wg sync.WaitGroup
	errors := make([]error, 10)
	tokens := make([]string, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			token, err := client.GetToken(context.Background())
			tokens[idx] = token
			errors[idx] = err
		}(i)
	}
	wg.Wait()

	for i, err := range errors {
		if err != nil {
			t.Errorf("goroutine %d error: %v", i, err)
		}
	}
	for i, tok := range tokens {
		if tok != "shared-token" {
			t.Errorf("goroutine %d token = %q, want %q", i, tok, "shared-token")
		}
	}

	// Singleflight: only 1 actual token fetch despite 10 concurrent callers
	if count := fetchCount.Load(); count != 1 {
		t.Errorf("fetch count = %d, want 1 (singleflight dedup)", count)
	}
}

// Test 8.6: Provider unreachable — timeout within 5 seconds
func TestGetToken_Timeout(t *testing.T) {
	// Create a listener then close it immediately — connection refused
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	unreachableURL := srv.URL
	srv.Close()

	client := NewOIDCClient(unreachableURL, "test-id", "test-secret", nil, nil)

	start := time.Now()
	_, err := client.GetToken(context.Background())
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error for unreachable provider")
	}

	// Should fail quickly (connection refused), well within 5s timeout
	if elapsed > 6*time.Second {
		t.Errorf("timeout took %v, expected well under 5s", elapsed)
	}
}

// Test 8.7: Error response (400/500) — clean error, no cached stale data
func TestGetToken_ErrorResponse(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"400 Bad Request", http.StatusBadRequest},
		{"500 Internal Server Error", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var srv *httptest.Server
			mux := http.NewServeMux()
			mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
				json.NewEncoder(w).Encode(map[string]string{
					"issuer":         srv.URL,
					"token_endpoint": srv.URL + "/oauth/token",
				})
			})
			mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(`{"error":"invalid_client"}`))
			})
			srv = httptest.NewServer(mux)
			t.Cleanup(srv.Close)

			client := NewOIDCClient(srv.URL, "test-id", "test-secret", nil, nil)

			_, err := client.GetToken(context.Background())
			if err == nil {
				t.Fatal("expected error for error response")
			}

			// Verify no stale data cached
			client.mu.Lock()
			cached := client.cachedToken
			client.mu.Unlock()
			if cached != nil {
				t.Error("expected nil cached token after error")
			}
		})
	}
}

// Test 8.8: Discovery endpoint failure — non-JSON response
func TestGetToken_DiscoveryFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("not json"))
	}))
	t.Cleanup(srv.Close)

	client := NewOIDCClient(srv.URL, "test-id", "test-secret", nil, nil)

	_, err := client.GetToken(context.Background())
	if err == nil {
		t.Fatal("expected error for non-JSON discovery")
	}
	if !strings.Contains(err.Error(), "oidc discovery") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "oidc discovery")
	}
}

// Test 8.8b: Discovery returns empty token_endpoint
func TestGetToken_DiscoveryMissingTokenEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{
			"issuer": "https://mock-issuer",
		})
	}))
	t.Cleanup(srv.Close)

	client := NewOIDCClient(srv.URL, "test-id", "test-secret", nil, nil)

	_, err := client.GetToken(context.Background())
	if err == nil {
		t.Fatal("expected error for missing token_endpoint")
	}
	if !strings.Contains(err.Error(), "token_endpoint not found") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "token_endpoint not found")
	}
}

// Test 8.9: Token value never in error messages
func TestGetToken_TokenNeverInErrors(t *testing.T) {
	secretToken := "super-secret-token-12345"

	srv := newMockOIDCProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": secretToken,
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	})

	client := NewOIDCClient(srv.URL, "test-id", "test-secret", nil, nil)

	// Get a valid token first
	token, err := client.GetToken(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != secretToken {
		t.Fatalf("token = %q, want %q", token, secretToken)
	}

	// Now force an error by poisoning the token endpoint
	client.mu.Lock()
	client.cachedToken = nil
	client.mu.Unlock()
	client.tokenEndpoint = "http://invalid.invalid/token"

	_, err = client.GetToken(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}

	errStr := err.Error()
	if strings.Contains(errStr, secretToken) {
		t.Errorf("error message contains token value: %q", errStr)
	}
	if strings.Contains(errStr, "test-id") {
		t.Errorf("error message contains client ID: %q", errStr)
	}
	if strings.Contains(errStr, "test-secret") {
		t.Errorf("error message contains client secret: %q", errStr)
	}
}

// Test: Error not cached — failed fetch allows retry on next call
func TestGetToken_ErrorNotCached(t *testing.T) {
	callCount := atomic.Int32{}

	var srv *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{
			"issuer":         srv.URL,
			"token_endpoint": srv.URL + "/oauth/token",
		})
	})
	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		count := callCount.Add(1)
		if count == 1 {
			// First call fails
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"server_error"}`))
			return
		}
		// Second call succeeds
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "retry-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := NewOIDCClient(srv.URL, "test-id", "test-secret", nil, nil)

	// First call fails
	_, err := client.GetToken(context.Background())
	if err == nil {
		t.Fatal("expected error on first call")
	}

	// Second call should retry and succeed (error not cached)
	token, err := client.GetToken(context.Background())
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}
	if token != "retry-token" {
		t.Errorf("token = %q, want %q", token, "retry-token")
	}
}

// Test: Scopes are sent in token request
func TestGetToken_ScopesSent(t *testing.T) {
	var receivedScope string

	srv := newMockOIDCProvider(t, func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		receivedScope = r.FormValue("scope")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "scoped-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	})

	client := NewOIDCClient(srv.URL, "test-id", "test-secret", []string{"openid", "profile"}, nil)

	_, err := client.GetToken(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedScope != "openid profile" {
		t.Errorf("scope = %q, want %q", receivedScope, "openid profile")
	}
}

// Test: Grant type is client_credentials
func TestGetToken_GrantType(t *testing.T) {
	var receivedGrantType string

	srv := newMockOIDCProvider(t, func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		receivedGrantType = r.FormValue("grant_type")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	})

	client := NewOIDCClient(srv.URL, "test-id", "test-secret", nil, nil)

	_, err := client.GetToken(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedGrantType != "client_credentials" {
		t.Errorf("grant_type = %q, want %q", receivedGrantType, "client_credentials")
	}
}

// Test: Discovery endpoint is only called once (cached for lifetime)
func TestGetToken_DiscoveryCached(t *testing.T) {
	var discoveryCount atomic.Int32

	var srv *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		discoveryCount.Add(1)
		json.NewEncoder(w).Encode(map[string]string{
			"issuer":         srv.URL,
			"token_endpoint": srv.URL + "/oauth/token",
		})
	})
	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := NewOIDCClient(srv.URL, "test-id", "test-secret", nil, nil)

	// First call discovers endpoint
	_, err := client.GetToken(context.Background())
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}

	// Force cache miss to trigger another fetch (but NOT rediscovery)
	client.mu.Lock()
	client.cachedToken = nil
	client.mu.Unlock()

	_, err = client.GetToken(context.Background())
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}

	if count := discoveryCount.Load(); count != 1 {
		t.Errorf("discovery count = %d, want 1 (should be cached)", count)
	}
}

// Test: Discovery returns non-200 status
func TestGetToken_DiscoveryNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	client := NewOIDCClient(srv.URL, "test-id", "test-secret", nil, nil)

	_, err := client.GetToken(context.Background())
	if err == nil {
		t.Fatal("expected error for non-200 discovery")
	}
	if !strings.Contains(err.Error(), "oidc discovery") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "oidc discovery")
	}
}

// Test: Context cancellation
func TestGetToken_ContextCancelled(t *testing.T) {
	// Server that blocks until request context is done
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)

	client := NewOIDCClient(srv.URL, "test-id", "test-secret", nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := client.GetToken(ctx)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

// Test: Empty access_token in response
func TestGetToken_EmptyAccessToken(t *testing.T) {
	srv := newMockOIDCProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	})

	client := NewOIDCClient(srv.URL, "test-id", "test-secret", nil, nil)

	_, err := client.GetToken(context.Background())
	if err == nil {
		t.Fatal("expected error for empty access_token")
	}
	if !strings.Contains(err.Error(), "empty access_token") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "empty access_token")
	}
}
