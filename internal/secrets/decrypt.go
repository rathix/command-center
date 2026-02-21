// Package secrets provides encrypted secrets file loading for OIDC credentials.
//
// The encrypted file uses a binary envelope format:
//
//	[0..31]   32-byte Argon2id salt
//	[32..43]  12-byte AES-GCM nonce
//	[44..N]   AES-256-GCM ciphertext + 16-byte auth tag
//
// The passphrase is read from the SECRETS_KEY environment variable, derived
// into a 256-bit AES key via Argon2id (OWASP-recommended parameters), and
// used for AES-256-GCM decryption. After use, passphrase bytes, derived key,
// and plaintext are zeroed from memory.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"
	"log/slog"
	"os"
	"regexp"

	"golang.org/x/crypto/argon2"
	"gopkg.in/yaml.v3"
)

// Argon2id parameters (OWASP recommended). Exported for use by cmd/encrypt-secrets.
const (
	Argon2Time    = 1         // iterations
	Argon2Memory  = 64 * 1024 // 64 MiB
	Argon2Threads = 4         // parallelism
	Argon2KeyLen  = 32        // 256 bits for AES-256
	SaltSize      = 32        // bytes
	NonceSize     = 12        // standard GCM nonce
)

// minFileSize is the minimum valid encrypted file size:
// 32 (salt) + 12 (nonce) + 16 (GCM tag) = 60 bytes.
const minFileSize = SaltSize + NonceSize + 16

// envVarPattern matches ${VAR_NAME} references for substitution.
var envVarPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

// OIDCCredentials holds decrypted OIDC client credentials.
type OIDCCredentials struct {
	ClientID     string
	ClientSecret string
}

// secretsYAML is the internal YAML structure for the decrypted secrets file.
type secretsYAML struct {
	OIDC struct {
		ClientID     string `yaml:"clientId"`
		ClientSecret string `yaml:"clientSecret"`
	} `yaml:"oidc"`
}

// LoadSecrets decrypts and parses an encrypted secrets file. If secretsFile is
// empty, it returns (nil, nil) indicating OIDC is disabled. The decryption key
// is read from the SECRETS_KEY environment variable.
func LoadSecrets(secretsFile string, logger *slog.Logger) (*OIDCCredentials, error) {
	if secretsFile == "" {
		return nil, nil
	}

	// Check file permissions before reading
	info, err := os.Stat(secretsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("load secrets: file not found")
		}
		return nil, fmt.Errorf("load secrets: file not found")
	}

	perm := info.Mode().Perm()
	if perm&0377 != 0 {
		logger.Warn("secrets file has overly permissive permissions")
	}

	// Read the encrypted file
	data, err := os.ReadFile(secretsFile)
	if err != nil {
		return nil, fmt.Errorf("load secrets: file not found")
	}

	// Validate minimum file size
	if len(data) < minFileSize {
		return nil, fmt.Errorf("load secrets: file too small")
	}

	// Slice the envelope
	salt := data[:SaltSize]
	nonce := data[SaltSize : SaltSize+NonceSize]
	ciphertext := data[SaltSize+NonceSize:]

	// Read passphrase from environment
	passphrase := os.Getenv("SECRETS_KEY")
	if passphrase == "" {
		return nil, fmt.Errorf("load secrets: decryption key not provided")
	}

	// Derive key using Argon2id
	passphraseBytes := []byte(passphrase)
	key := argon2.IDKey(passphraseBytes, salt, Argon2Time, Argon2Memory, Argon2Threads, Argon2KeyLen)
	clear(passphraseBytes)

	// Decrypt with AES-256-GCM
	block, err := aes.NewCipher(key)
	if err != nil {
		clear(key)
		return nil, fmt.Errorf("load secrets: decryption failed")
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		clear(key)
		return nil, fmt.Errorf("load secrets: decryption failed")
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	clear(key)
	if err != nil {
		return nil, fmt.Errorf("load secrets: decryption failed")
	}

	// Apply env var substitution before YAML parsing
	substituted := envVarPattern.ReplaceAllStringFunc(string(plaintext), func(match string) string {
		varName := envVarPattern.FindStringSubmatch(match)[1]
		return os.Getenv(varName)
	})
	clear(plaintext)

	// Parse YAML
	var sf secretsYAML
	if err := yaml.Unmarshal([]byte(substituted), &sf); err != nil {
		return nil, fmt.Errorf("load secrets: invalid format")
	}

	// Validate required fields
	if sf.OIDC.ClientID == "" || sf.OIDC.ClientSecret == "" {
		return nil, fmt.Errorf("load secrets: missing required credentials")
	}

	return &OIDCCredentials{
		ClientID:     sf.OIDC.ClientID,
		ClientSecret: sf.OIDC.ClientSecret,
	}, nil
}
