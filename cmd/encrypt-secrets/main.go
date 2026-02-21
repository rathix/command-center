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
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("encrypt-secrets", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var inFile, outFile string
	fs.StringVar(&inFile, "in", "-", "path to plaintext YAML secrets file ('-' for stdin)")
	fs.StringVar(&outFile, "out", "-", "path to write encrypted output ('-' for stdout)")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	passphrase := os.Getenv("SECRETS_KEY")
	if passphrase == "" {
		fmt.Fprintln(stderr, "error: SECRETS_KEY environment variable is required")
		return 1
	}

	plaintext, err := readInput(inFile, stdin)
	if err != nil {
		fmt.Fprintln(stderr, "error: cannot read input")
		return 1
	}

	encrypted, err := encrypt(plaintext, passphrase)
	if err != nil {
		fmt.Fprintln(stderr, "error: encryption failed")
		return 1
	}

	if err := writeOutput(outFile, encrypted, stdout); err != nil {
		fmt.Fprintln(stderr, "error: cannot write output")
		return 1
	}

	fmt.Fprintln(stderr, "encrypted secrets written successfully")
	return 0
}

func readInput(path string, stdin io.Reader) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(stdin)
	}
	return os.ReadFile(path)
}

func writeOutput(path string, data []byte, stdout io.Writer) error {
	if path == "-" {
		_, err := stdout.Write(data)
		return err
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0400)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	// Enforce restrictive permissions even when the file already existed.
	return os.Chmod(path, 0400)
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
