package secrets

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/argon2"
)

// testEncrypt creates a valid encrypted file using the same Argon2id + AES-256-GCM
// logic as the production code. Used for round-trip testing.
func testEncrypt(t *testing.T, plaintext []byte, passphrase string) []byte {
	t.Helper()

	salt := make([]byte, SaltSize)
	if _, err := rand.Read(salt); err != nil {
		t.Fatalf("failed to generate salt: %v", err)
	}

	nonce := make([]byte, NonceSize)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatalf("failed to generate nonce: %v", err)
	}

	key := argon2.IDKey([]byte(passphrase), salt, Argon2Time, Argon2Memory, Argon2Threads, Argon2KeyLen)

	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("failed to create cipher: %v", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("failed to create GCM: %v", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	// Build envelope: salt || nonce || ciphertext+tag
	envelope := make([]byte, 0, SaltSize+NonceSize+len(ciphertext))
	envelope = append(envelope, salt...)
	envelope = append(envelope, nonce...)
	envelope = append(envelope, ciphertext...)

	return envelope
}

// writeEncryptedFile writes encrypted content to a temp file with specified permissions.
func writeEncryptedFile(t *testing.T, data []byte, perm os.FileMode) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "secrets.enc")
	if err := os.WriteFile(path, data, perm); err != nil {
		t.Fatalf("failed to write encrypted file: %v", err)
	}
	return path
}

// testLogger returns a logger that writes to a buffer for assertion.
func testLogger(t *testing.T) (*slog.Logger, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	return slog.New(handler), &buf
}

func TestLoadSecrets_EmptyPath(t *testing.T) {
	t.Parallel()
	logger, _ := testLogger(t)
	creds, err := LoadSecrets("", logger)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if creds != nil {
		t.Fatalf("expected nil credentials, got: %+v", creds)
	}
}

func TestLoadSecrets_ValidRoundTrip(t *testing.T) {
	passphrase := "test-passphrase-for-round-trip"
	yamlContent := `oidc:
  clientId: "my-client"
  clientSecret: "my-secret"
`
	encrypted := testEncrypt(t, []byte(yamlContent), passphrase)
	path := writeEncryptedFile(t, encrypted, 0400)

	t.Setenv("SECRETS_KEY", passphrase)

	logger, _ := testLogger(t)
	creds, err := LoadSecrets(path, logger)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if creds == nil {
		t.Fatal("expected credentials, got nil")
	}
	if creds.ClientID != "my-client" {
		t.Errorf("expected ClientID=my-client, got %q", creds.ClientID)
	}
	if creds.ClientSecret != "my-secret" {
		t.Errorf("expected ClientSecret=my-secret, got %q", creds.ClientSecret)
	}
}

func TestLoadSecrets_WrongKey(t *testing.T) {
	passphrase := "correct-passphrase"
	yamlContent := `oidc:
  clientId: "my-client"
  clientSecret: "my-secret"
`
	encrypted := testEncrypt(t, []byte(yamlContent), passphrase)
	path := writeEncryptedFile(t, encrypted, 0400)

	t.Setenv("SECRETS_KEY", "wrong-passphrase")

	logger, _ := testLogger(t)
	creds, err := LoadSecrets(path, logger)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if creds != nil {
		t.Fatalf("expected nil credentials, got: %+v", creds)
	}
	if err.Error() != "load secrets: decryption failed" {
		t.Errorf("expected generic error, got: %v", err)
	}
	// Verify no key material leaks in error
	errMsg := err.Error()
	if strings.Contains(errMsg, "wrong-passphrase") || strings.Contains(errMsg, "correct-passphrase") {
		t.Errorf("error contains passphrase: %v", err)
	}
}

func TestLoadSecrets_MissingKey(t *testing.T) {
	passphrase := "some-passphrase"
	yamlContent := `oidc:
  clientId: "x"
  clientSecret: "y"
`
	encrypted := testEncrypt(t, []byte(yamlContent), passphrase)
	path := writeEncryptedFile(t, encrypted, 0400)

	// Ensure SECRETS_KEY is not set
	t.Setenv("SECRETS_KEY", "")

	logger, _ := testLogger(t)
	creds, err := LoadSecrets(path, logger)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if creds != nil {
		t.Fatalf("expected nil credentials, got: %+v", creds)
	}
	if err.Error() != "load secrets: decryption key not provided" {
		t.Errorf("expected generic error, got: %v", err)
	}
	// Must not mention SECRETS_KEY env var name
	if strings.Contains(err.Error(), "SECRETS_KEY") {
		t.Errorf("error mentions env var name: %v", err)
	}
}

func TestLoadSecrets_FileTooSmall(t *testing.T) {
	// Write a file with less than 60 bytes
	path := writeEncryptedFile(t, make([]byte, 59), 0400)

	t.Setenv("SECRETS_KEY", "any-key")

	logger, _ := testLogger(t)
	creds, err := LoadSecrets(path, logger)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if creds != nil {
		t.Fatalf("expected nil credentials, got: %+v", creds)
	}
	if err.Error() != "load secrets: file too small" {
		t.Errorf("expected 'file too small' error, got: %v", err)
	}
}

func TestLoadSecrets_CorruptedCiphertext(t *testing.T) {
	passphrase := "test-passphrase"
	yamlContent := `oidc:
  clientId: "a"
  clientSecret: "b"
`
	encrypted := testEncrypt(t, []byte(yamlContent), passphrase)

	// Corrupt the ciphertext portion (after salt+nonce)
	encrypted[SaltSize+NonceSize+5] ^= 0xFF

	path := writeEncryptedFile(t, encrypted, 0400)
	t.Setenv("SECRETS_KEY", passphrase)

	logger, _ := testLogger(t)
	creds, err := LoadSecrets(path, logger)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if creds != nil {
		t.Fatalf("expected nil credentials, got: %+v", creds)
	}
	if err.Error() != "load secrets: decryption failed" {
		t.Errorf("expected generic error, got: %v", err)
	}
}

func TestLoadSecrets_TruncatedCiphertext(t *testing.T) {
	passphrase := "test-passphrase"
	yamlContent := `oidc:
  clientId: "a"
  clientSecret: "b"
`
	encrypted := testEncrypt(t, []byte(yamlContent), passphrase)

	// Truncate: keep salt+nonce but only partial ciphertext (still >= 60 total)
	truncated := encrypted[:minFileSize]
	path := writeEncryptedFile(t, truncated, 0400)
	t.Setenv("SECRETS_KEY", passphrase)

	logger, _ := testLogger(t)
	creds, err := LoadSecrets(path, logger)
	if err == nil {
		t.Fatal("expected error for truncated file, got nil")
	}
	if creds != nil {
		t.Fatalf("expected nil credentials, got: %+v", creds)
	}
	if err.Error() != "load secrets: decryption failed" {
		t.Errorf("expected generic error, got: %v", err)
	}
}

func TestLoadSecrets_PermissionsNoWarning(t *testing.T) {
	passphrase := "test-passphrase"
	yamlContent := `oidc:
  clientId: "x"
  clientSecret: "y"
`
	encrypted := testEncrypt(t, []byte(yamlContent), passphrase)
	path := writeEncryptedFile(t, encrypted, 0400)

	t.Setenv("SECRETS_KEY", passphrase)

	logger, logBuf := testLogger(t)
	_, err := LoadSecrets(path, logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	logOutput := logBuf.String()
	if strings.Contains(logOutput, "overly permissive") {
		t.Errorf("expected no permission warning for 0400, got log output: %s", logOutput)
	}
}

func TestLoadSecrets_PermissionsWarning(t *testing.T) {
	passphrase := "test-passphrase"
	yamlContent := `oidc:
  clientId: "x"
  clientSecret: "y"
`
	encrypted := testEncrypt(t, []byte(yamlContent), passphrase)
	path := writeEncryptedFile(t, encrypted, 0644)

	t.Setenv("SECRETS_KEY", passphrase)

	logger, logBuf := testLogger(t)
	_, err := LoadSecrets(path, logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "overly permissive") {
		t.Errorf("expected permission warning for 0644, got log output: %q", logOutput)
	}
	// Verify no file path in the warning
	if strings.Contains(logOutput, path) {
		t.Errorf("permission warning contains file path: %s", logOutput)
	}
}

func TestLoadSecrets_EnvVarSubstitution(t *testing.T) {
	passphrase := "test-passphrase"
	yamlContent := `oidc:
  clientId: "${TEST_OIDC_CID}"
  clientSecret: "${TEST_OIDC_SEC}"
`
	encrypted := testEncrypt(t, []byte(yamlContent), passphrase)
	path := writeEncryptedFile(t, encrypted, 0400)

	t.Setenv("SECRETS_KEY", passphrase)
	t.Setenv("TEST_OIDC_CID", "env-client-id")
	t.Setenv("TEST_OIDC_SEC", "env-client-secret")

	logger, _ := testLogger(t)
	creds, err := LoadSecrets(path, logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.ClientID != "env-client-id" {
		t.Errorf("expected ClientID=env-client-id, got %q", creds.ClientID)
	}
	if creds.ClientSecret != "env-client-secret" {
		t.Errorf("expected ClientSecret=env-client-secret, got %q", creds.ClientSecret)
	}
}

func TestLoadSecrets_UnsetEnvVarSubstitutesEmpty(t *testing.T) {
	passphrase := "test-passphrase"
	yamlContent := `oidc:
  clientId: "static-id"
  clientSecret: "${UNSET_VAR_XXYZ}"
`
	encrypted := testEncrypt(t, []byte(yamlContent), passphrase)
	path := writeEncryptedFile(t, encrypted, 0400)

	t.Setenv("SECRETS_KEY", passphrase)
	// Do NOT set UNSET_VAR_XXYZ

	logger, _ := testLogger(t)
	creds, err := LoadSecrets(path, logger)
	if err == nil {
		t.Fatal("expected error for missing credentials, got nil")
	}
	if creds != nil {
		t.Fatalf("expected nil credentials, got: %+v", creds)
	}
	if err.Error() != "load secrets: missing required credentials" {
		t.Errorf("expected missing credentials error, got: %v", err)
	}
}

func TestLoadSecrets_MissingRequiredFields(t *testing.T) {
	passphrase := "test-passphrase"

	tests := []struct {
		name string
		yaml string
	}{
		{
			name: "empty clientId",
			yaml: "oidc:\n  clientId: \"\"\n  clientSecret: \"secret\"\n",
		},
		{
			name: "empty clientSecret",
			yaml: "oidc:\n  clientId: \"id\"\n  clientSecret: \"\"\n",
		},
		{
			name: "both empty",
			yaml: "oidc:\n  clientId: \"\"\n  clientSecret: \"\"\n",
		},
		{
			name: "missing oidc section",
			yaml: "other:\n  key: value\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			encrypted := testEncrypt(t, []byte(tc.yaml), passphrase)
			path := writeEncryptedFile(t, encrypted, 0400)

			t.Setenv("SECRETS_KEY", passphrase)

			logger, _ := testLogger(t)
			creds, err := LoadSecrets(path, logger)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if creds != nil {
				t.Fatalf("expected nil credentials, got: %+v", creds)
			}
			if err.Error() != "load secrets: missing required credentials" {
				t.Errorf("expected missing credentials error, got: %v", err)
			}
		})
	}
}

func TestLoadSecrets_InvalidYAML(t *testing.T) {
	passphrase := "test-passphrase"
	invalidYAML := `{{{not valid yaml`

	encrypted := testEncrypt(t, []byte(invalidYAML), passphrase)
	path := writeEncryptedFile(t, encrypted, 0400)

	t.Setenv("SECRETS_KEY", passphrase)

	logger, _ := testLogger(t)
	creds, err := LoadSecrets(path, logger)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if creds != nil {
		t.Fatalf("expected nil credentials, got: %+v", creds)
	}
	if err.Error() != "load secrets: invalid format" {
		t.Errorf("expected invalid format error, got: %v", err)
	}
	// Ensure no plaintext content leaks
	if strings.Contains(err.Error(), "not valid yaml") {
		t.Error("error contains plaintext content")
	}
}

func TestLoadSecrets_EnvVarSpecialChars(t *testing.T) {
	passphrase := "test-passphrase"
	yamlContent := `oidc:
  clientId: "${TEST_SPECIAL_ID}"
  clientSecret: "${TEST_SPECIAL_SEC}"
`
	encrypted := testEncrypt(t, []byte(yamlContent), passphrase)
	path := writeEncryptedFile(t, encrypted, 0400)

	t.Setenv("SECRETS_KEY", passphrase)
	t.Setenv("TEST_SPECIAL_ID", "client-with-special=chars&more")
	t.Setenv("TEST_SPECIAL_SEC", "secret/with+special$chars!")

	logger, _ := testLogger(t)
	creds, err := LoadSecrets(path, logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.ClientID != "client-with-special=chars&more" {
		t.Errorf("unexpected ClientID: %q", creds.ClientID)
	}
	if creds.ClientSecret != "secret/with+special$chars!" {
		t.Errorf("unexpected ClientSecret: %q", creds.ClientSecret)
	}
}

func TestLoadSecrets_FileNotFound(t *testing.T) {
	t.Setenv("SECRETS_KEY", "any-key")

	logger, _ := testLogger(t)
	creds, err := LoadSecrets("/nonexistent/path/to/secrets.enc", logger)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if creds != nil {
		t.Fatalf("expected nil credentials, got: %+v", creds)
	}
	if err.Error() != "load secrets: file not found" {
		t.Errorf("expected file not found error, got: %v", err)
	}
	// Ensure file path not in error
	if strings.Contains(err.Error(), "/nonexistent") {
		t.Error("error contains file path")
	}
}

func TestLoadSecrets_ErrorMessagesNeverLeakSensitiveData(t *testing.T) {
	// Table-driven test: every error path must have a clean error message
	passphrase := "super-secret-passphrase-12345"
	filePath := "/tmp/test-secrets-file.enc"

	forbidden := []string{
		passphrase,
		filePath,
		"SECRETS_KEY",
		"super-secret",
	}

	tests := []struct {
		name    string
		setup   func(t *testing.T) string
		wantErr string
	}{
		{
			name: "file not found",
			setup: func(t *testing.T) string {
				t.Setenv("SECRETS_KEY", passphrase)
				return filepath.Join(t.TempDir(), "nonexistent.enc")
			},
			wantErr: "load secrets: file not found",
		},
		{
			name: "file too small",
			setup: func(t *testing.T) string {
				t.Setenv("SECRETS_KEY", passphrase)
				return writeEncryptedFile(t, make([]byte, 30), 0400)
			},
			wantErr: "load secrets: file too small",
		},
		{
			name: "missing key",
			setup: func(t *testing.T) string {
				t.Setenv("SECRETS_KEY", "")
				yaml := "oidc:\n  clientId: a\n  clientSecret: b\n"
				enc := testEncrypt(t, []byte(yaml), passphrase)
				return writeEncryptedFile(t, enc, 0400)
			},
			wantErr: "load secrets: decryption key not provided",
		},
		{
			name: "wrong key",
			setup: func(t *testing.T) string {
				t.Setenv("SECRETS_KEY", "wrong-key")
				yaml := "oidc:\n  clientId: a\n  clientSecret: b\n"
				enc := testEncrypt(t, []byte(yaml), passphrase)
				return writeEncryptedFile(t, enc, 0400)
			},
			wantErr: "load secrets: decryption failed",
		},
		{
			name: "invalid yaml",
			setup: func(t *testing.T) string {
				t.Setenv("SECRETS_KEY", passphrase)
				enc := testEncrypt(t, []byte("{{{bad"), passphrase)
				return writeEncryptedFile(t, enc, 0400)
			},
			wantErr: "load secrets: invalid format",
		},
		{
			name: "missing credentials",
			setup: func(t *testing.T) string {
				t.Setenv("SECRETS_KEY", passphrase)
				enc := testEncrypt(t, []byte("oidc:\n  clientId: \"\"\n  clientSecret: \"\"\n"), passphrase)
				return writeEncryptedFile(t, enc, 0400)
			},
			wantErr: "load secrets: missing required credentials",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := tc.setup(t)
			logger, _ := testLogger(t)
			_, err := LoadSecrets(path, logger)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if err.Error() != tc.wantErr {
				t.Errorf("expected %q, got %q", tc.wantErr, err.Error())
			}
			for _, f := range forbidden {
				if strings.Contains(err.Error(), f) {
					t.Errorf("error contains forbidden string %q: %v", f, err)
				}
			}
		})
	}
}

func TestLoadSecrets_Permissions600Warns(t *testing.T) {
	passphrase := "test-passphrase"
	yamlContent := `oidc:
  clientId: "x"
  clientSecret: "y"
`
	encrypted := testEncrypt(t, []byte(yamlContent), passphrase)
	path := writeEncryptedFile(t, encrypted, 0600)

	t.Setenv("SECRETS_KEY", passphrase)

	logger, logBuf := testLogger(t)
	_, err := LoadSecrets(path, logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "overly permissive") {
		t.Errorf("expected permission warning for 0600, got: %q", logOutput)
	}
}
