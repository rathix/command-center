package session

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"strings"
	"testing"
	"time"
)

func TestGenerateSecret(t *testing.T) {
	s1, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret() error = %v", err)
	}
	if len(s1) != 32 {
		t.Errorf("secret length = %d, want 32", len(s1))
	}

	s2, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret() second call error = %v", err)
	}
	if string(s1) == string(s2) {
		t.Error("two GenerateSecret calls produced identical secrets")
	}
}

func TestCreateAndVerifyToken(t *testing.T) {
	secret, _ := GenerateSecret()
	fp := "abc123def456"

	token := CreateToken(secret, fp, time.Hour)

	got, err := VerifyToken(secret, token)
	if err != nil {
		t.Fatalf("VerifyToken() error = %v", err)
	}
	if got != fp {
		t.Errorf("fingerprint = %q, want %q", got, fp)
	}
}

func TestVerifyTokenExpired(t *testing.T) {
	secret, _ := GenerateSecret()
	token := CreateToken(secret, "fp", -time.Second)

	_, err := VerifyToken(secret, token)
	if err != ErrTokenExpired {
		t.Errorf("VerifyToken() error = %v, want ErrTokenExpired", err)
	}
}

func TestVerifyTokenTampered(t *testing.T) {
	secret, _ := GenerateSecret()
	token := CreateToken(secret, "fp", time.Hour)

	// Flip a character in the middle of the token
	runes := []rune(token)
	mid := len(runes) / 2
	if runes[mid] == 'A' {
		runes[mid] = 'B'
	} else {
		runes[mid] = 'A'
	}
	tampered := string(runes)

	_, err := VerifyToken(secret, tampered)
	if err == nil {
		t.Fatal("VerifyToken() should reject tampered token")
	}
	if err != ErrTokenTampered && err != ErrTokenMalformed {
		t.Errorf("VerifyToken() error = %v, want ErrTokenTampered or ErrTokenMalformed", err)
	}
}

func TestVerifyTokenMalformed(t *testing.T) {
	secret, _ := GenerateSecret()
	cases := []struct {
		name  string
		token string
	}{
		{"empty", ""},
		{"garbage", "not-base64-!!!"},
		{"partial", "abc"},
		{"no-pipes", "aGVsbG8"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := VerifyToken(secret, tc.token)
			if err == nil {
				t.Error("VerifyToken() should reject malformed token")
			}
		})
	}
}

func TestVerifyTokenOversized(t *testing.T) {
	secret, _ := GenerateSecret()
	oversized := strings.Repeat("A", 513)

	_, err := VerifyToken(secret, oversized)
	if err != ErrTokenOversized {
		t.Errorf("VerifyToken() error = %v, want ErrTokenOversized", err)
	}
}

func TestCertFingerprint(t *testing.T) {
	cert := generateTestCert(t)

	fp1 := CertFingerprint(cert)
	fp2 := CertFingerprint(cert)

	if fp1 != fp2 {
		t.Error("CertFingerprint should be deterministic")
	}
	if len(fp1) != 64 { // SHA-256 hex = 64 chars
		t.Errorf("fingerprint length = %d, want 64", len(fp1))
	}
}

func TestVerifyTokenWrongSecret(t *testing.T) {
	secret1, _ := GenerateSecret()
	secret2, _ := GenerateSecret()

	token := CreateToken(secret1, "fp", time.Hour)
	_, err := VerifyToken(secret2, token)
	if err != ErrTokenTampered {
		t.Errorf("VerifyToken() with wrong secret: error = %v, want ErrTokenTampered", err)
	}
}

func TestCreateTokenPanicsOnDelimiterInFingerprint(t *testing.T) {
	secret, _ := GenerateSecret()
	defer func() {
		if r := recover(); r == nil {
			t.Error("CreateToken should panic when fingerprint contains '|'")
		}
	}()
	CreateToken(secret, "abc|def", time.Hour)
}

// generateTestCert creates a self-signed x509 certificate for testing.
func generateTestCert(t *testing.T) *x509.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test-client"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("creating certificate: %v", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("parsing certificate: %v", err)
	}
	return cert
}
