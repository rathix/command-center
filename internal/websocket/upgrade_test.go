package websocket

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckMTLS_ValidCert(t *testing.T) {
	r := &http.Request{
		TLS: &tls.ConnectionState{
			PeerCertificates: []*x509.Certificate{{}},
		},
	}
	if !CheckMTLS(r) {
		t.Error("expected CheckMTLS to return true for request with client cert")
	}
}

func TestCheckMTLS_NilTLS(t *testing.T) {
	r := &http.Request{}
	if CheckMTLS(r) {
		t.Error("expected CheckMTLS to return false for request without TLS")
	}
}

func TestCheckMTLS_NoPeerCerts(t *testing.T) {
	r := &http.Request{
		TLS: &tls.ConnectionState{
			PeerCertificates: []*x509.Certificate{},
		},
	}
	if CheckMTLS(r) {
		t.Error("expected CheckMTLS to return false for request with no peer certificates")
	}
}

func TestRejectNoMTLS(t *testing.T) {
	w := httptest.NewRecorder()
	RejectNoMTLS(w)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	body := w.Body.String()
	if body == "" {
		t.Error("expected non-empty response body")
	}
}
