package main

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rathix/command-center/internal/secrets"
)

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	inFile := filepath.Join(dir, "secrets.yaml")
	outFile := filepath.Join(dir, "secrets.enc")

	yamlContent := `oidc:
  clientId: "round-trip-client"
  clientSecret: "round-trip-secret"
`
	if err := os.WriteFile(inFile, []byte(yamlContent), 0600); err != nil {
		t.Fatalf("failed to write input: %v", err)
	}

	passphrase := "test-roundtrip-key"
	t.Setenv("SECRETS_KEY", passphrase)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"-in", inFile, "-out", outFile}, bytes.NewReader(nil), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, stderr.String())
	}

	if !strings.Contains(stderr.String(), "encrypted secrets written successfully") {
		t.Errorf("expected success message, got: %s", stderr.String())
	}

	// Decrypt with LoadSecrets to verify round-trip
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))
	creds, err := secrets.LoadSecrets(outFile, logger)
	if err != nil {
		t.Fatalf("LoadSecrets failed: %v", err)
	}
	if creds == nil {
		t.Fatal("expected credentials, got nil")
	}
	if creds.ClientID != "round-trip-client" {
		t.Errorf("expected ClientID=round-trip-client, got %q", creds.ClientID)
	}
	if creds.ClientSecret != "round-trip-secret" {
		t.Errorf("expected ClientSecret=round-trip-secret, got %q", creds.ClientSecret)
	}
}

func TestMissingSecretsKey(t *testing.T) {
	dir := t.TempDir()
	inFile := filepath.Join(dir, "secrets.yaml")
	outFile := filepath.Join(dir, "secrets.enc")

	if err := os.WriteFile(inFile, []byte("test"), 0600); err != nil {
		t.Fatalf("failed to write input: %v", err)
	}

	t.Setenv("SECRETS_KEY", "")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"-in", inFile, "-out", outFile}, bytes.NewReader(nil), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "SECRETS_KEY environment variable is required") {
		t.Errorf("expected SECRETS_KEY error, got: %s", stderr.String())
	}
}

func TestMissingInputFile(t *testing.T) {
	dir := t.TempDir()
	outFile := filepath.Join(dir, "secrets.enc")

	t.Setenv("SECRETS_KEY", "some-key")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run(
		[]string{"-in", filepath.Join(dir, "nonexistent.yaml"), "-out", outFile},
		bytes.NewReader(nil),
		&stdout,
		&stderr,
	)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "cannot read input") {
		t.Errorf("expected input file error, got: %s", stderr.String())
	}
}

func TestDefaultStdinStdout(t *testing.T) {
	input := "oidc:\n  clientId: stdin-client\n  clientSecret: stdin-secret\n"
	t.Setenv("SECRETS_KEY", "stdin-key")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{}, strings.NewReader(input), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, stderr.String())
	}

	if stdout.Len() == 0 {
		t.Fatal("expected encrypted bytes on stdout")
	}

	// Decrypt to verify stdout payload is a valid encrypted envelope.
	dir := t.TempDir()
	outFile := filepath.Join(dir, "stdout.enc")
	if err := os.WriteFile(outFile, stdout.Bytes(), 0400); err != nil {
		t.Fatalf("failed to persist stdout bytes: %v", err)
	}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))
	creds, err := secrets.LoadSecrets(outFile, logger)
	if err != nil {
		t.Fatalf("LoadSecrets failed: %v", err)
	}
	if creds == nil {
		t.Fatal("expected credentials, got nil")
	}
	if creds.ClientID != "stdin-client" {
		t.Errorf("expected ClientID=stdin-client, got %q", creds.ClientID)
	}
	if creds.ClientSecret != "stdin-secret" {
		t.Errorf("expected ClientSecret=stdin-secret, got %q", creds.ClientSecret)
	}

	if !strings.Contains(stderr.String(), "encrypted secrets written successfully") {
		t.Errorf("expected success message, got: %s", stderr.String())
	}
}

func TestOutputFilePermissions(t *testing.T) {
	dir := t.TempDir()
	inFile := filepath.Join(dir, "secrets.yaml")
	outFile := filepath.Join(dir, "secrets.enc")

	if err := os.WriteFile(inFile, []byte("oidc:\n  clientId: a\n  clientSecret: b\n"), 0600); err != nil {
		t.Fatalf("failed to write input: %v", err)
	}

	t.Setenv("SECRETS_KEY", "test-key")

	// Pre-create with permissive mode to verify chmod enforcement.
	if err := os.WriteFile(outFile, []byte("old"), 0644); err != nil {
		t.Fatalf("failed to pre-create output file: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"-in", inFile, "-out", outFile}, bytes.NewReader(nil), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, stderr.String())
	}

	info, err := os.Stat(outFile)
	if err != nil {
		t.Fatalf("failed to stat output: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0400 {
		t.Errorf("expected permissions 0400, got %04o", perm)
	}
}
