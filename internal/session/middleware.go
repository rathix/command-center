package session

import (
	"log/slog"
	"net/http"
	"time"
)

const cookieName = "__Host-session"

// Config holds session middleware configuration.
type Config struct {
	Secret   []byte
	Duration time.Duration
	Secure   bool // false only for httptest; true in production
}

// Middleware returns an http.Handler that authenticates requests via session
// cookie or mTLS client certificate. On successful mTLS, it issues a session
// cookie so subsequent requests skip the certificate prompt.
func Middleware(next http.Handler, cfg Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Logout endpoint — must have a valid session to reach here,
		// but we check it before the main auth flow so the cookie clear
		// is always processed.
		if r.Method == http.MethodPost && r.URL.Path == "/api/logout" {
			handleLogout(w, r, cfg)
			return
		}

		// Step 1: Check for existing session cookie.
		if cookie, err := r.Cookie(cookieName); err == nil {
			fingerprint, verifyErr := VerifyToken(cfg.Secret, cookie.Value)
			if verifyErr == nil {
				// Valid token — check cert fingerprint if a cert is presented.
				if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
					certFP := CertFingerprint(r.TLS.PeerCertificates[0])
					if certFP != fingerprint {
						slog.Warn("Session auth: cert fingerprint mismatch", "path", r.URL.Path)
						unauthorized(w)
						return
					}
				}
				// Valid session (with or without cert).
				next.ServeHTTP(w, r)
				return
			}
			slog.Info("Session auth: invalid cookie", "path", r.URL.Path, "reason", verifyErr.Error())
			// Cookie invalid/expired — fall through to cert check.
		}

		// Step 2: No valid cookie — try mTLS client certificate.
		if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
			cert := r.TLS.PeerCertificates[0]
			fp := CertFingerprint(cert)
			token := CreateToken(cfg.Secret, fp, cfg.Duration)
			http.SetCookie(w, &http.Cookie{
				Name:     cookieName,
				Value:    token,
				Path:     "/",
				HttpOnly: true,
				Secure:   cfg.Secure,
				SameSite: http.SameSiteStrictMode,
				MaxAge:   int(cfg.Duration.Seconds()),
			})
			next.ServeHTTP(w, r)
			return
		}

		// Step 3: No cookie, no cert — reject.
		slog.Warn("Session auth: no cookie and no client cert", "path", r.URL.Path)
		unauthorized(w)
	})
}

func handleLogout(w http.ResponseWriter, r *http.Request, cfg Config) {
	// Verify the caller has a valid session before clearing it.
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		unauthorized(w)
		return
	}
	fingerprint, err := VerifyToken(cfg.Secret, cookie.Value)
	if err != nil {
		unauthorized(w)
		return
	}

	// Check cert fingerprint mismatch if a cert is presented (same as main auth flow).
	if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
		certFP := CertFingerprint(r.TLS.PeerCertificates[0])
		if certFP != fingerprint {
			slog.Warn("Session auth: logout cert fingerprint mismatch", "path", r.URL.Path)
			unauthorized(w)
			return
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   cfg.Secure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"authentication required"}`))
}
