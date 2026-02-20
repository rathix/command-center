package certs

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// CAResult holds the paths and in-memory objects from CA generation.
type CAResult struct {
	CACertPath string
	CAKeyPath  string
	CACert     *x509.Certificate
	CAKey      *ecdsa.PrivateKey
}

// ServerCertResult holds the paths to the generated server certificate and key.
type ServerCertResult struct {
	ServerCertPath string
	ServerKeyPath  string
}

// ClientCertResult holds the paths to the generated client certificate and key.
type ClientCertResult struct {
	ClientCertPath string
	ClientKeyPath  string
}

// GenerateCA creates a self-signed CA certificate and writes the cert to certsDir/ca.crt.
// The CA key is kept in memory (returned in CAResult) for signing child certificates.
func GenerateCA(certsDir string) (*CAResult, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating CA key: %w", err)
	}

	serialNumber, err := randomSerial()
	if err != nil {
		return nil, err
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   "command-center-ca",
			Organization: []string{"Command Center"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("creating CA certificate: %w", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, fmt.Errorf("parsing CA certificate: %w", err)
	}

	certPath := filepath.Join(certsDir, "ca.crt")
	if err := writePEMFile(certPath, "CERTIFICATE", certDER); err != nil {
		return nil, fmt.Errorf("writing CA cert: %w", err)
	}

	keyPath := filepath.Join(certsDir, "ca.key")
	if err := writeKeyFile(keyPath, key); err != nil {
		return nil, fmt.Errorf("writing CA key: %w", err)
	}

	return &CAResult{
		CACertPath: certPath,
		CAKeyPath:  keyPath,
		CACert:     cert,
		CAKey:      key,
	}, nil
}

// GenerateServerCert creates a server certificate signed by the CA.
// SANs include localhost and 127.0.0.1. Validity is 1 year.
func GenerateServerCert(certsDir string, ca *CAResult) (*ServerCertResult, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating server key: %w", err)
	}

	serialNumber, err := randomSerial()
	if err != nil {
		return nil, err
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   "command-center-server",
			Organization: []string{"Command Center"},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:    []string{"localhost", "command-center"},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, ca.CACert, &key.PublicKey, ca.CAKey)
	if err != nil {
		return nil, fmt.Errorf("creating server certificate: %w", err)
	}

	certPath := filepath.Join(certsDir, "server.crt")
	keyPath := filepath.Join(certsDir, "server.key")

	if err := writePEMFile(certPath, "CERTIFICATE", certDER); err != nil {
		return nil, fmt.Errorf("writing server cert: %w", err)
	}
	if err := writeKeyFile(keyPath, key); err != nil {
		return nil, fmt.Errorf("writing server key: %w", err)
	}

	return &ServerCertResult{
		ServerCertPath: certPath,
		ServerKeyPath:  keyPath,
	}, nil
}

// GenerateClientCert creates a client certificate signed by the CA.
// CN is "command-center-client". Validity is 1 year.
func GenerateClientCert(certsDir string, ca *CAResult) (*ClientCertResult, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating client key: %w", err)
	}

	serialNumber, err := randomSerial()
	if err != nil {
		return nil, err
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   "command-center-client",
			Organization: []string{"Command Center"},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, ca.CACert, &key.PublicKey, ca.CAKey)
	if err != nil {
		return nil, fmt.Errorf("creating client certificate: %w", err)
	}

	certPath := filepath.Join(certsDir, "client.crt")
	keyPath := filepath.Join(certsDir, "client.key")

	if err := writePEMFile(certPath, "CERTIFICATE", certDER); err != nil {
		return nil, fmt.Errorf("writing client cert: %w", err)
	}
	if err := writeKeyFile(keyPath, key); err != nil {
		return nil, fmt.Errorf("writing client key: %w", err)
	}

	return &ClientCertResult{
		ClientCertPath: certPath,
		ClientKeyPath:  keyPath,
	}, nil
}

// CertsConfig holds configuration for certificate loading/generation.
type CertsConfig struct {
	DataDir      string // Base directory for auto-generated cert storage (certs written to DataDir/certs/)
	CustomCACert string // Custom CA cert path (if set, all three custom paths must be set)
	CustomCert   string // Custom server cert path
	CustomKey    string // Custom server key path
}

// TLSAssets holds the resolved paths to all TLS certificate files.
type TLSAssets struct {
	CACertPath     string
	ServerCertPath string
	ServerKeyPath  string
	ClientCertPath string
	ClientKeyPath  string
	WasGenerated   bool
	// GenerationReason clarifies whether assets were reused, generated fresh, or leaf certs were renewed.
	GenerationReason string
}

const (
	generationReasonCustom         = "custom"
	generationReasonReused         = "reused"
	generationReasonFullGeneration = "full-generation"
	generationReasonLeafRenewal    = "leaf-renewal"
)

// LoadOrGenerateCerts resolves TLS certificates based on configuration:
//   - If all custom paths are set, validates and uses them
//   - If certs exist in DataDir/certs/, reuses them
//   - Otherwise, generates new self-signed CA + server + client certs
func LoadOrGenerateCerts(cfg CertsConfig) (*TLSAssets, error) {
	// Check if custom paths are configured
	hasCustom := cfg.CustomCACert != "" || cfg.CustomCert != "" || cfg.CustomKey != ""
	allCustom := cfg.CustomCACert != "" && cfg.CustomCert != "" && cfg.CustomKey != ""

	if hasCustom && !allCustom {
		return nil, fmt.Errorf("partial custom cert config: all three of --tls-ca-cert, --tls-cert, and --tls-key must be set")
	}

	if allCustom {
		// Validate custom cert files exist and are readable (open + close, not just stat)
		for _, path := range []string{cfg.CustomCACert, cfg.CustomCert, cfg.CustomKey} {
			f, err := os.Open(path)
			if err != nil {
				return nil, fmt.Errorf("custom cert file not readable: %w", err)
			}
			f.Close()
		}
		return &TLSAssets{
			CACertPath:       cfg.CustomCACert,
			ServerCertPath:   cfg.CustomCert,
			ServerKeyPath:    cfg.CustomKey,
			WasGenerated:     false,
			GenerationReason: generationReasonCustom,
		}, nil
	}

	// Check for existing auto-generated certs
	certsDir := filepath.Join(cfg.DataDir, "certs")
	caCertPath := filepath.Join(certsDir, "ca.crt")
	caKeyPath := filepath.Join(certsDir, "ca.key")
	serverCertPath := filepath.Join(certsDir, "server.crt")
	serverKeyPath := filepath.Join(certsDir, "server.key")
	clientCertPath := filepath.Join(certsDir, "client.crt")
	clientKeyPath := filepath.Join(certsDir, "client.key")

	allExist := true
	for _, path := range []string{caCertPath, caKeyPath, serverCertPath, serverKeyPath, clientCertPath, clientKeyPath} {
		if _, err := os.Stat(path); err != nil {
			allExist = false
			break
		}
	}

	if allExist {
		// Check if certs are still valid (not expired)
		serverExpired, err := certExpired(serverCertPath)
		if err != nil {
			return nil, fmt.Errorf("checking server certificate expiration: %w", err)
		}
		clientExpired, err := certExpired(clientCertPath)
		if err != nil {
			return nil, fmt.Errorf("checking client certificate expiration: %w", err)
		}
		if !serverExpired && !clientExpired {
			return &TLSAssets{
				CACertPath:       caCertPath,
				ServerCertPath:   serverCertPath,
				ServerKeyPath:    serverKeyPath,
				ClientCertPath:   clientCertPath,
				ClientKeyPath:    clientKeyPath,
				WasGenerated:     false,
				GenerationReason: generationReasonReused,
			}, nil
		}
	}

	// Generate new certs with lock file to prevent race conditions
	if err := os.MkdirAll(certsDir, 0o700); err != nil {
		return nil, fmt.Errorf("creating certs directory: %w", err)
	}

	unlock, err := acquireLock(certsDir)
	if err != nil {
		return nil, fmt.Errorf("acquiring cert generation lock: %w", err)
	}
	defer unlock()

	// Re-check after acquiring lock — another process may have generated certs
	allExistNow := true
	for _, path := range []string{caCertPath, caKeyPath, serverCertPath, serverKeyPath, clientCertPath, clientKeyPath} {
		if _, err := os.Stat(path); err != nil {
			allExistNow = false
			break
		}
	}
	if allExistNow {
		serverExpired, err := certExpired(serverCertPath)
		if err != nil {
			return nil, fmt.Errorf("checking server certificate expiration: %w", err)
		}
		clientExpired, err := certExpired(clientCertPath)
		if err != nil {
			return nil, fmt.Errorf("checking client certificate expiration: %w", err)
		}
		if !serverExpired && !clientExpired {
			return &TLSAssets{
				CACertPath:       caCertPath,
				ServerCertPath:   serverCertPath,
				ServerKeyPath:    serverKeyPath,
				ClientCertPath:   clientCertPath,
				ClientKeyPath:    clientKeyPath,
				WasGenerated:     false,
				GenerationReason: generationReasonReused,
			}, nil
		}

		// Existing CA is still valid trust material; renew only expired leaf certificates.
		caResult, err := loadCAFromDisk(caCertPath, caKeyPath)
		if err == nil {
			if serverExpired {
				if _, err := GenerateServerCert(certsDir, caResult); err != nil {
					return nil, fmt.Errorf("renewing server cert: %w", err)
				}
			}
			if clientExpired {
				if _, err := GenerateClientCert(certsDir, caResult); err != nil {
					return nil, fmt.Errorf("renewing client cert: %w", err)
				}
			}
			return &TLSAssets{
				CACertPath:       caCertPath,
				ServerCertPath:   serverCertPath,
				ServerKeyPath:    serverKeyPath,
				ClientCertPath:   clientCertPath,
				ClientKeyPath:    clientKeyPath,
				WasGenerated:     true,
				GenerationReason: generationReasonLeafRenewal,
			}, nil
		}
	}

	caResult, err := GenerateCA(certsDir)
	if err != nil {
		return nil, fmt.Errorf("generating CA: %w", err)
	}

	serverResult, err := GenerateServerCert(certsDir, caResult)
	if err != nil {
		return nil, fmt.Errorf("generating server cert: %w", err)
	}

	clientResult, err := GenerateClientCert(certsDir, caResult)
	if err != nil {
		return nil, fmt.Errorf("generating client cert: %w", err)
	}

	return &TLSAssets{
		CACertPath:       caResult.CACertPath,
		ServerCertPath:   serverResult.ServerCertPath,
		ServerKeyPath:    serverResult.ServerKeyPath,
		ClientCertPath:   clientResult.ClientCertPath,
		ClientKeyPath:    clientResult.ClientKeyPath,
		WasGenerated:     true,
		GenerationReason: generationReasonFullGeneration,
	}, nil
}

// NewTLSConfig builds a *tls.Config configured for mTLS with TLS 1.3 minimum.
func NewTLSConfig(caCertPath, serverCertPath, serverKeyPath string) (*tls.Config, error) {
	caCertPEM, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, fmt.Errorf("reading CA cert: %w", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCertPEM) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	serverCert, err := tls.LoadX509KeyPair(serverCertPath, serverKeyPath)
	if err != nil {
		return nil, fmt.Errorf("loading server key pair: %w", err)
	}

	return &tls.Config{
		MinVersion:   tls.VersionTLS13,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caCertPool,
		Certificates: []tls.Certificate{serverCert},
	}, nil
}

// staleLockAge is the maximum age of a lock file before it is considered stale
// and eligible for cleanup (e.g., left behind by a crashed process).
const staleLockAge = 5 * time.Minute

// acquireLock creates an exclusive lock file in certsDir to prevent concurrent
// certificate generation by multiple processes. Returns an unlock function.
// If a lock file older than staleLockAge is found, it is removed as stale.
func acquireLock(certsDir string) (func(), error) {
	lockPath := filepath.Join(certsDir, ".lock")
	for i := 0; i < 10; i++ {
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			f.Close()
			return func() { os.Remove(lockPath) }, nil
		}
		if !os.IsExist(err) {
			return nil, fmt.Errorf("creating lock file: %w", err)
		}
		// Check if the existing lock is stale
		if info, statErr := os.Stat(lockPath); statErr == nil {
			if time.Since(info.ModTime()) > staleLockAge {
				os.Remove(lockPath)
				continue // retry immediately after cleaning stale lock
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return nil, fmt.Errorf("could not acquire cert generation lock at %s after 5s", lockPath)
}

// certExpired checks whether the PEM-encoded certificate at path has expired.
func certExpired(certPath string) (bool, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return false, fmt.Errorf("reading cert file: %w", err)
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return false, fmt.Errorf("no PEM data in %s", certPath)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false, fmt.Errorf("parsing certificate: %w", err)
	}
	return time.Now().After(cert.NotAfter), nil
}

// loadCAFromDisk loads a previously generated CA cert and key pair.
func loadCAFromDisk(caCertPath, caKeyPath string) (*CAResult, error) {
	caCertPEM, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, fmt.Errorf("reading CA certificate: %w", err)
	}
	caCertBlock, _ := pem.Decode(caCertPEM)
	if caCertBlock == nil {
		return nil, fmt.Errorf("no PEM data in CA certificate %s", caCertPath)
	}
	caCert, err := x509.ParseCertificate(caCertBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing CA certificate: %w", err)
	}

	caKeyPEM, err := os.ReadFile(caKeyPath)
	if err != nil {
		return nil, fmt.Errorf("reading CA key: %w", err)
	}
	caKeyBlock, _ := pem.Decode(caKeyPEM)
	if caKeyBlock == nil {
		return nil, fmt.Errorf("no PEM data in CA key %s", caKeyPath)
	}
	caKey, err := x509.ParseECPrivateKey(caKeyBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing CA key: %w", err)
	}

	return &CAResult{
		CACertPath: caCertPath,
		CAKeyPath:  caKeyPath,
		CACert:     caCert,
		CAKey:      caKey,
	}, nil
}

func randomSerial() (*big.Int, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("generating serial number: %w", err)
	}
	return serial, nil
}

func writePEMFile(path, blockType string, data []byte) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return pem.Encode(f, &pem.Block{Type: blockType, Bytes: data})
}

func writeKeyFile(path string, key *ecdsa.PrivateKey) error {
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return fmt.Errorf("marshaling EC private key: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	return pem.Encode(f, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
}
