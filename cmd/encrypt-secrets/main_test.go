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

	var stderr bytes.Buffer
	code := run([]string{"-in", inFile, "-out", outFile}, &stderr)
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

	var stderr bytes.Buffer
	code := run([]string{"-in", inFile, "-out", outFile}, &stderr)
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

	var stderr bytes.Buffer
	code := run([]string{"-in", filepath.Join(dir, "nonexistent.yaml"), "-out", outFile}, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "cannot read input file") {
		t.Errorf("expected input file error, got: %s", stderr.String())
	}
}

func TestMissingInFlag(t *testing.T) {
	t.Setenv("SECRETS_KEY", "some-key")

	var stderr bytes.Buffer
	code := run([]string{"-out", "output.enc"}, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "-in flag is required") {
		t.Errorf("expected -in flag error, got: %s", stderr.String())
	}
}

func TestMissingOutFlag(t *testing.T) {
	t.Setenv("SECRETS_KEY", "some-key")

	var stderr bytes.Buffer
	code := run([]string{"-in", "input.yaml"}, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "-out flag is required") {
		t.Errorf("expected -out flag error, got: %s", stderr.String())
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

	var stderr bytes.Buffer
	code := run([]string{"-in", inFile, "-out", outFile}, &stderr)
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
