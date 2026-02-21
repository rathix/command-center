// Command encrypt-secrets encrypts a plaintext YAML secrets file using
// Argon2id key derivation and AES-256-GCM authenticated encryption.
//
// Usage:
//
//	SECRETS_KEY=my-passphrase encrypt-secrets -in secrets.yaml -out secrets.enc
//
// The passphrase is read from the SECRETS_KEY environment variable.
// The output file is created with 0400 (owner read-only) permissions.
package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/rathix/command-center/internal/secrets"
	"golang.org/x/crypto/argon2"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

func run(args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("encrypt-secrets", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var inFile, outFile string
	fs.StringVar(&inFile, "in", "", "path to plaintext YAML secrets file")
	fs.StringVar(&outFile, "out", "", "path to write encrypted output")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	if inFile == "" {
		fmt.Fprintln(stderr, "error: -in flag is required")
		return 1
	}
	if outFile == "" {
		fmt.Fprintln(stderr, "error: -out flag is required")
		return 1
	}

	passphrase := os.Getenv("SECRETS_KEY")
	if passphrase == "" {
		fmt.Fprintln(stderr, "error: SECRETS_KEY environment variable is required")
		return 1
	}

	plaintext, err := os.ReadFile(inFile)
	if err != nil {
		fmt.Fprintln(stderr, "error: cannot read input file")
		return 1
	}

	encrypted, err := encrypt(plaintext, passphrase)
	if err != nil {
		fmt.Fprintln(stderr, "error: encryption failed")
		return 1
	}

	if err := os.WriteFile(outFile, encrypted, 0400); err != nil {
		fmt.Fprintln(stderr, "error: cannot write output file")
		return 1
	}

	fmt.Fprintln(stderr, "encrypted secrets written successfully")
	return 0
}

func encrypt(plaintext []byte, passphrase string) ([]byte, error) {
	salt := make([]byte, secrets.SaltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("generate salt: %w", err)
	}

	nonce := make([]byte, secrets.NonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	passphraseBytes := []byte(passphrase)
	key := argon2.IDKey(passphraseBytes, salt, secrets.Argon2Time, secrets.Argon2Memory, secrets.Argon2Threads, secrets.Argon2KeyLen)
	clear(passphraseBytes)

	block, err := aes.NewCipher(key)
	if err != nil {
		clear(key)
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		clear(key)
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	clear(key)

	// Build envelope: salt || nonce || ciphertext+tag
	envelope := make([]byte, 0, secrets.SaltSize+secrets.NonceSize+len(ciphertext))
	envelope = append(envelope, salt...)
	envelope = append(envelope, nonce...)
	envelope = append(envelope, ciphertext...)

	return envelope, nil
}
