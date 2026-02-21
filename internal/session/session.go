package session

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	// secretSize is the number of random bytes for the HMAC key.
	secretSize = 32

	// maxTokenBytes is the maximum base64url-encoded token length accepted.
	// Tokens exceeding this are rejected before any decoding or HMAC processing.
	maxTokenBytes = 512
)

var (
	ErrTokenExpired   = errors.New("session token expired")
	ErrTokenTampered  = errors.New("session token HMAC invalid")
	ErrTokenMalformed = errors.New("session token malformed")
	ErrTokenOversized = errors.New("session token exceeds size limit")
)

// GenerateSecret returns a cryptographically random 32-byte HMAC key.
func GenerateSecret() ([]byte, error) {
	secret := make([]byte, secretSize)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("generating session secret: %w", err)
	}
	return secret, nil
}

// CreateToken builds an HMAC-SHA256 signed session token.
// The token encodes the cert fingerprint, issue time, and expiry as:
//
//	base64url(fingerprint|issuedAt|expiresAt|hmac)
//
// certFingerprint must not contain the '|' delimiter. The only intended
// source is CertFingerprint(), which returns hex-encoded SHA-256 (safe).
func CreateToken(secret []byte, certFingerprint string, duration time.Duration) string {
	if strings.ContainsRune(certFingerprint, '|') {
		panic("session: certFingerprint contains delimiter '|'")
	}

	now := time.Now()
	payload := certFingerprint + "|" + strconv.FormatInt(now.Unix(), 10) + "|" + strconv.FormatInt(now.Add(duration).Unix(), 10)

	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	sig := mac.Sum(nil)

	raw := payload + "|" + base64.RawURLEncoding.EncodeToString(sig)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// VerifyToken decodes and verifies an HMAC-SHA256 signed session token.
// Returns the cert fingerprint on success.
func VerifyToken(secret []byte, token string) (string, error) {
	if len(token) > maxTokenBytes {
		return "", ErrTokenOversized
	}

	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return "", ErrTokenMalformed
	}

	parts := strings.Split(string(raw), "|")
	if len(parts) != 4 {
		return "", ErrTokenMalformed
	}

	fingerprint := parts[0]
	issuedAtStr := parts[1]
	expiresAtStr := parts[2]
	sigB64 := parts[3]

	// Validate timestamp fields are numeric
	if _, err := strconv.ParseInt(issuedAtStr, 10, 64); err != nil {
		return "", ErrTokenMalformed
	}
	expiresAt, err := strconv.ParseInt(expiresAtStr, 10, 64)
	if err != nil {
		return "", ErrTokenMalformed
	}

	// Verify HMAC before checking expiry (constant-time comparison)
	payload := fingerprint + "|" + issuedAtStr + "|" + expiresAtStr
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	expectedSig := mac.Sum(nil)

	sig, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		return "", ErrTokenTampered
	}
	if !hmac.Equal(sig, expectedSig) {
		return "", ErrTokenTampered
	}

	// Check expiry
	if time.Now().Unix() > expiresAt {
		return "", ErrTokenExpired
	}

	return fingerprint, nil
}

// CertFingerprint returns the hex-encoded SHA-256 hash of a certificate's raw DER bytes.
func CertFingerprint(cert *x509.Certificate) string {
	sum := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(sum[:])
}
