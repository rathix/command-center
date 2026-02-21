package session

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// okHandler is a simple handler that writes 200 OK — used to verify the
// middleware passed the request through.
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
})

func testConfig(t *testing.T) Config {
	t.Helper()
	secret, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret() error = %v", err)
	}
	return Config{
		Secret:   secret,
		Duration: time.Hour,
		Secure:   false, // false for httptest
	}
}

func TestMiddleware_ValidSessionCookie(t *testing.T) {
	cfg := testConfig(t)
	cert := generateTestCert(t)
	fp := CertFingerprint(cert)

	token := CreateToken(cfg.Secret, fp, cfg.Duration)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: cookieName, Value: token})
	w := httptest.NewRecorder()

	Middleware(okHandler, cfg).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestMiddleware_ExpiredCookie_ValidCert(t *testing.T) {
	cfg := testConfig(t)
	cert := generateTestCert(t)
	fp := CertFingerprint(cert)

	// Create an expired token
	token := CreateToken(cfg.Secret, fp, -time.Second)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: cookieName, Value: token})
	r.TLS = &tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{cert},
	}
	w := httptest.NewRecorder()

	Middleware(okHandler, cfg).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	// Should have issued a new cookie
	cookies := w.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == cookieName {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected new session cookie to be set")
	}
}

func TestMiddleware_ExpiredCookie_NoCert(t *testing.T) {
	cfg := testConfig(t)

	token := CreateToken(cfg.Secret, "somefp", -time.Second)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: cookieName, Value: token})
	w := httptest.NewRecorder()

	Middleware(okHandler, cfg).ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestMiddleware_NoCookie_ValidCert(t *testing.T) {
	cfg := testConfig(t)
	cert := generateTestCert(t)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.TLS = &tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{cert},
	}
	w := httptest.NewRecorder()

	Middleware(okHandler, cfg).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	// Should have set a cookie
	cookies := w.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == cookieName {
			found = true
			if !c.HttpOnly {
				t.Error("cookie should be HttpOnly")
			}
			if c.SameSite != http.SameSiteStrictMode {
				t.Errorf("cookie SameSite = %v, want Strict", c.SameSite)
			}
			break
		}
	}
	if !found {
		t.Error("expected session cookie to be set")
	}
}

func TestMiddleware_NoCookie_NoCert(t *testing.T) {
	cfg := testConfig(t)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	Middleware(okHandler, cfg).ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}
	body := w.Body.String()
	if !strings.Contains(body, "authentication required") {
		t.Errorf("body = %q, want authentication required message", body)
	}
}

func TestMiddleware_ValidCookie_MismatchedCert(t *testing.T) {
	cfg := testConfig(t)
	cert1 := generateTestCert(t)
	cert2 := generateTestCert(t)

	// Token signed for cert1
	token := CreateToken(cfg.Secret, CertFingerprint(cert1), cfg.Duration)

	// Request presents cert2
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: cookieName, Value: token})
	r.TLS = &tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{cert2},
	}
	w := httptest.NewRecorder()

	Middleware(okHandler, cfg).ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 (cert fingerprint mismatch)", w.Code)
	}
}

func TestMiddleware_ValidCookie_NoCert(t *testing.T) {
	cfg := testConfig(t)
	cert := generateTestCert(t)
	fp := CertFingerprint(cert)

	token := CreateToken(cfg.Secret, fp, cfg.Duration)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: cookieName, Value: token})
	// No TLS / no cert — this is the normal session flow
	w := httptest.NewRecorder()

	Middleware(okHandler, cfg).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (valid cookie, no cert = normal session)", w.Code)
	}
}

func TestMiddleware_Logout(t *testing.T) {
	cfg := testConfig(t)
	cert := generateTestCert(t)
	fp := CertFingerprint(cert)

	token := CreateToken(cfg.Secret, fp, cfg.Duration)

	r := httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	r.AddCookie(&http.Cookie{Name: cookieName, Value: token})
	w := httptest.NewRecorder()

	Middleware(okHandler, cfg).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"ok":true`) {
		t.Errorf("body = %q, want ok:true", body)
	}
	// Cookie should be cleared
	cookies := w.Result().Cookies()
	for _, c := range cookies {
		if c.Name == cookieName && c.MaxAge != -1 {
			t.Errorf("cookie MaxAge = %d, want -1", c.MaxAge)
		}
	}
}

func TestMiddleware_LogoutWithoutSession(t *testing.T) {
	cfg := testConfig(t)

	r := httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	w := httptest.NewRecorder()

	Middleware(okHandler, cfg).ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 (no session for logout)", w.Code)
	}
}

func TestMiddleware_ConcurrentAccess(t *testing.T) {
	cfg := testConfig(t)
	cert := generateTestCert(t)
	fp := CertFingerprint(cert)
	token := CreateToken(cfg.Secret, fp, cfg.Duration)

	handler := Middleware(okHandler, cfg)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.AddCookie(&http.Cookie{Name: cookieName, Value: token})
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != http.StatusOK {
				t.Errorf("concurrent request: status = %d, want 200", w.Code)
			}
		}()
	}
	wg.Wait()
}

func TestMiddleware_ConcurrentCertAuth(t *testing.T) {
	cfg := testConfig(t)
	cert := generateTestCert(t)
	handler := Middleware(okHandler, cfg)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.TLS = &tls.ConnectionState{
				PeerCertificates: []*x509.Certificate{cert},
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != http.StatusOK {
				t.Errorf("concurrent cert auth: status = %d, want 200", w.Code)
			}
		}()
	}
	wg.Wait()
}

func TestMiddleware_LogoutWrongMethod(t *testing.T) {
	cfg := testConfig(t)
	cert := generateTestCert(t)
	fp := CertFingerprint(cert)
	token := CreateToken(cfg.Secret, fp, cfg.Duration)

	// GET /api/logout should NOT trigger logout — falls through to normal auth
	r := httptest.NewRequest(http.MethodGet, "/api/logout", nil)
	r.AddCookie(&http.Cookie{Name: cookieName, Value: token})
	w := httptest.NewRecorder()

	Middleware(okHandler, cfg).ServeHTTP(w, r)

	// Should pass through to okHandler (valid cookie), not logout
	if w.Code != http.StatusOK {
		t.Errorf("GET /api/logout status = %d, want 200 (not a logout)", w.Code)
	}
	// Cookie should NOT be cleared
	for _, c := range w.Result().Cookies() {
		if c.Name == cookieName && c.MaxAge == -1 {
			t.Error("GET /api/logout should not clear cookie")
		}
	}
}

func TestMiddleware_LogoutMismatchedCert(t *testing.T) {
	cfg := testConfig(t)
	cert1 := generateTestCert(t)
	cert2 := generateTestCert(t)

	// Token for cert1, request presents cert2
	token := CreateToken(cfg.Secret, CertFingerprint(cert1), cfg.Duration)

	r := httptest.NewRequest(http.MethodPost, "/api/logout", nil)
	r.AddCookie(&http.Cookie{Name: cookieName, Value: token})
	r.TLS = &tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{cert2},
	}
	w := httptest.NewRecorder()

	Middleware(okHandler, cfg).ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 (logout with mismatched cert)", w.Code)
	}
}
