package certs

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func loadCertFromFile(t *testing.T, path string) *x509.Certificate {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read cert file %s: %v", path, err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		t.Fatalf("failed to decode PEM from %s", path)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse certificate from %s: %v", path, err)
	}
	return cert
}

func loadKeyFromFile(t *testing.T, path string) *ecdsa.PrivateKey {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read key file %s: %v", path, err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		t.Fatalf("failed to decode PEM from %s", path)
	}
	key, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse EC private key from %s: %v", path, err)
	}
	return key
}

func TestGenerateCA(t *testing.T) {
	dir := t.TempDir()

	result, err := GenerateCA(dir)
	if err != nil {
		t.Fatalf("GenerateCA returned error: %v", err)
	}

	if result.CACertPath != filepath.Join(dir, "ca.crt") {
		t.Errorf("expected CACertPath %s, got %s", filepath.Join(dir, "ca.crt"), result.CACertPath)
	}

	// Verify CA cert properties
	cert := loadCertFromFile(t, result.CACertPath)

	if !cert.IsCA {
		t.Error("CA certificate should have IsCA=true")
	}

	if cert.KeyUsage&x509.KeyUsageCertSign == 0 {
		t.Error("CA certificate should have KeyUsageCertSign")
	}

	if cert.KeyUsage&x509.KeyUsageCRLSign == 0 {
		t.Error("CA certificate should have KeyUsageCRLSign")
	}

	// Verify key algorithm is ECDSA P-256
	if cert.PublicKeyAlgorithm != x509.ECDSA {
		t.Errorf("expected ECDSA public key, got %v", cert.PublicKeyAlgorithm)
	}
	pubKey, ok := cert.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		t.Fatal("expected *ecdsa.PublicKey")
	}
	if pubKey.Curve != elliptic.P256() {
		t.Error("expected P-256 curve")
	}

	// Verify self-signed
	if err := cert.CheckSignatureFrom(cert); err != nil {
		t.Errorf("CA cert should be self-signed: %v", err)
	}

	// Verify PEM block type for cert
	data, _ := os.ReadFile(result.CACertPath)
	block, _ := pem.Decode(data)
	if block.Type != "CERTIFICATE" {
		t.Errorf("expected PEM type CERTIFICATE, got %s", block.Type)
	}
}

func TestGenerateServerCert(t *testing.T) {
	dir := t.TempDir()

	caResult, err := GenerateCA(dir)
	if err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}

	result, err := GenerateServerCert(dir, caResult)
	if err != nil {
		t.Fatalf("GenerateServerCert returned error: %v", err)
	}

	if result.ServerCertPath != filepath.Join(dir, "server.crt") {
		t.Errorf("expected ServerCertPath %s, got %s", filepath.Join(dir, "server.crt"), result.ServerCertPath)
	}
	if result.ServerKeyPath != filepath.Join(dir, "server.key") {
		t.Errorf("expected ServerKeyPath %s, got %s", filepath.Join(dir, "server.key"), result.ServerKeyPath)
	}

	cert := loadCertFromFile(t, result.ServerCertPath)

	// Verify signed by CA
	caCert := loadCertFromFile(t, caResult.CACertPath)
	if err := cert.CheckSignatureFrom(caCert); err != nil {
		t.Errorf("server cert should be signed by CA: %v", err)
	}

	// Verify ExtKeyUsage
	foundServerAuth := false
	for _, usage := range cert.ExtKeyUsage {
		if usage == x509.ExtKeyUsageServerAuth {
			foundServerAuth = true
		}
	}
	if !foundServerAuth {
		t.Error("server cert should have ExtKeyUsageServerAuth")
	}

	// Verify SANs
	foundLocalhost := false
	for _, name := range cert.DNSNames {
		if name == "localhost" {
			foundLocalhost = true
		}
	}
	if !foundLocalhost {
		t.Error("server cert should have localhost in SANs")
	}

	foundLoopback := false
	for _, ip := range cert.IPAddresses {
		if ip.String() == "127.0.0.1" {
			foundLoopback = true
		}
	}
	if !foundLoopback {
		t.Error("server cert should have 127.0.0.1 in SANs")
	}

	// Verify command-center service name in SANs for Docker/K8s
	foundCommandCenter := false
	for _, name := range cert.DNSNames {
		if name == "command-center" {
			foundCommandCenter = true
		}
	}
	if !foundCommandCenter {
		t.Error("server cert should have 'command-center' in SANs for Docker/K8s compatibility")
	}

	// Verify not a CA
	if cert.IsCA {
		t.Error("server cert should not be a CA")
	}

	// Verify ECDSA P-256
	if cert.PublicKeyAlgorithm != x509.ECDSA {
		t.Errorf("expected ECDSA public key, got %v", cert.PublicKeyAlgorithm)
	}
}

func TestGenerateClientCert(t *testing.T) {
	dir := t.TempDir()

	caResult, err := GenerateCA(dir)
	if err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}

	result, err := GenerateClientCert(dir, caResult)
	if err != nil {
		t.Fatalf("GenerateClientCert returned error: %v", err)
	}

	if result.ClientCertPath != filepath.Join(dir, "client.crt") {
		t.Errorf("expected ClientCertPath %s, got %s", filepath.Join(dir, "client.crt"), result.ClientCertPath)
	}
	if result.ClientKeyPath != filepath.Join(dir, "client.key") {
		t.Errorf("expected ClientKeyPath %s, got %s", filepath.Join(dir, "client.key"), result.ClientKeyPath)
	}

	cert := loadCertFromFile(t, result.ClientCertPath)

	// Verify signed by CA
	caCert := loadCertFromFile(t, caResult.CACertPath)
	if err := cert.CheckSignatureFrom(caCert); err != nil {
		t.Errorf("client cert should be signed by CA: %v", err)
	}

	// Verify ExtKeyUsage
	foundClientAuth := false
	for _, usage := range cert.ExtKeyUsage {
		if usage == x509.ExtKeyUsageClientAuth {
			foundClientAuth = true
		}
	}
	if !foundClientAuth {
		t.Error("client cert should have ExtKeyUsageClientAuth")
	}

	// Verify CN
	if cert.Subject.CommonName != "command-center-client" {
		t.Errorf("expected CN 'command-center-client', got %q", cert.Subject.CommonName)
	}

	// Verify not a CA
	if cert.IsCA {
		t.Error("client cert should not be a CA")
	}

	// Verify ECDSA P-256
	if cert.PublicKeyAlgorithm != x509.ECDSA {
		t.Errorf("expected ECDSA public key, got %v", cert.PublicKeyAlgorithm)
	}
}

func TestGenerateCAWritesKeyFile(t *testing.T) {
	dir := t.TempDir()

	result, err := GenerateCA(dir)
	if err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}

	// Verify ca.crt exists and is valid PEM
	data, err := os.ReadFile(filepath.Join(dir, "ca.crt"))
	if err != nil {
		t.Fatalf("ca.crt should exist: %v", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		t.Fatal("ca.crt should contain valid PEM")
	}
	if block.Type != "CERTIFICATE" {
		t.Errorf("expected CERTIFICATE PEM type, got %s", block.Type)
	}

	// Verify ca.key exists and is valid EC PRIVATE KEY PEM
	keyPath := filepath.Join(dir, "ca.key")
	if result.CAKeyPath != keyPath {
		t.Errorf("expected CAKeyPath %s, got %s", keyPath, result.CAKeyPath)
	}
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("ca.key should exist: %v", err)
	}
	keyBlock, _ := pem.Decode(keyData)
	if keyBlock == nil {
		t.Fatal("ca.key should contain valid PEM")
	}
	if keyBlock.Type != "EC PRIVATE KEY" {
		t.Errorf("expected 'EC PRIVATE KEY' PEM type, got %s", keyBlock.Type)
	}

	// Verify the key can be parsed
	_ = loadKeyFromFile(t, keyPath)
}

func TestServerKeyFileHasCorrectPEMType(t *testing.T) {
	dir := t.TempDir()

	caResult, err := GenerateCA(dir)
	if err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}

	result, err := GenerateServerCert(dir, caResult)
	if err != nil {
		t.Fatalf("GenerateServerCert: %v", err)
	}

	data, err := os.ReadFile(result.ServerKeyPath)
	if err != nil {
		t.Fatalf("server.key should exist: %v", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		t.Fatal("server.key should contain valid PEM")
	}
	if block.Type != "EC PRIVATE KEY" {
		t.Errorf("expected 'EC PRIVATE KEY' PEM type, got %s", block.Type)
	}

	// Verify it's a valid ECDSA key
	_ = loadKeyFromFile(t, result.ServerKeyPath)
}

// --- Task 2: LoadOrGenerateCerts tests ---

func TestLoadOrGenerateCertsFirstRunGenerates(t *testing.T) {
	dir := t.TempDir()

	cfg := CertsConfig{DataDir: dir}
	assets, err := LoadOrGenerateCerts(cfg)
	if err != nil {
		t.Fatalf("LoadOrGenerateCerts: %v", err)
	}

	if !assets.WasGenerated {
		t.Error("first run should set WasGenerated=true")
	}

	// Verify all files exist
	for _, path := range []string{assets.CACertPath, assets.ServerCertPath, assets.ServerKeyPath, assets.ClientCertPath, assets.ClientKeyPath} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file to exist: %s", path)
		}
	}
}

func TestLoadOrGenerateCertsSecondRunReuses(t *testing.T) {
	dir := t.TempDir()

	cfg := CertsConfig{DataDir: dir}
	assets1, err := LoadOrGenerateCerts(cfg)
	if err != nil {
		t.Fatalf("first LoadOrGenerateCerts: %v", err)
	}

	// Get modification time of ca.crt after first run
	stat1, err := os.Stat(assets1.CACertPath)
	if err != nil {
		t.Fatalf("stat ca.crt: %v", err)
	}

	// Second run should reuse
	assets2, err := LoadOrGenerateCerts(cfg)
	if err != nil {
		t.Fatalf("second LoadOrGenerateCerts: %v", err)
	}

	if assets2.WasGenerated {
		t.Error("second run should set WasGenerated=false (reusing existing certs)")
	}

	// File should not have been modified
	stat2, err := os.Stat(assets2.CACertPath)
	if err != nil {
		t.Fatalf("stat ca.crt: %v", err)
	}
	if !stat1.ModTime().Equal(stat2.ModTime()) {
		t.Error("ca.crt should not have been regenerated")
	}
}

func TestLoadOrGenerateCertsCustomPaths(t *testing.T) {
	dir := t.TempDir()

	// Generate certs first to create valid cert files for custom paths
	cfg := CertsConfig{DataDir: dir}
	assets, err := LoadOrGenerateCerts(cfg)
	if err != nil {
		t.Fatalf("LoadOrGenerateCerts: %v", err)
	}

	// Use the generated certs as "custom" certs
	customDir := t.TempDir()
	customCfg := CertsConfig{
		DataDir:      customDir,
		CustomCACert: assets.CACertPath,
		CustomCert:   assets.ServerCertPath,
		CustomKey:    assets.ServerKeyPath,
	}

	customAssets, err := LoadOrGenerateCerts(customCfg)
	if err != nil {
		t.Fatalf("LoadOrGenerateCerts with custom: %v", err)
	}

	if customAssets.WasGenerated {
		t.Error("custom paths should set WasGenerated=false")
	}

	if customAssets.CACertPath != assets.CACertPath {
		t.Errorf("expected custom CA path %s, got %s", assets.CACertPath, customAssets.CACertPath)
	}
}

func TestLoadOrGenerateCertsPartialCustomPathsError(t *testing.T) {
	dir := t.TempDir()

	// Only set one custom path — should error
	cfg := CertsConfig{
		DataDir:    dir,
		CustomCert: "/some/cert.pem",
	}

	_, err := LoadOrGenerateCerts(cfg)
	if err == nil {
		t.Error("expected error when only some custom paths are set")
	}
}

func TestLoadOrGenerateCertsCustomPathNotFoundError(t *testing.T) {
	dir := t.TempDir()

	cfg := CertsConfig{
		DataDir:      dir,
		CustomCACert: "/nonexistent/ca.crt",
		CustomCert:   "/nonexistent/server.crt",
		CustomKey:    "/nonexistent/server.key",
	}

	_, err := LoadOrGenerateCerts(cfg)
	if err == nil {
		t.Error("expected error when custom cert files don't exist")
	}
}

func TestClientKeyFileHasCorrectPEMType(t *testing.T) {
	dir := t.TempDir()

	caResult, err := GenerateCA(dir)
	if err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}

	result, err := GenerateClientCert(dir, caResult)
	if err != nil {
		t.Fatalf("GenerateClientCert: %v", err)
	}

	data, err := os.ReadFile(result.ClientKeyPath)
	if err != nil {
		t.Fatalf("client.key should exist: %v", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		t.Fatal("client.key should contain valid PEM")
	}
	if block.Type != "EC PRIVATE KEY" {
		t.Errorf("expected 'EC PRIVATE KEY' PEM type, got %s", block.Type)
	}

	_ = loadKeyFromFile(t, result.ClientKeyPath)
}

func TestPrivateKeyFilesHaveRestrictivePermissions(t *testing.T) {
	dir := t.TempDir()

	cfg := CertsConfig{DataDir: dir}
	assets, err := LoadOrGenerateCerts(cfg)
	if err != nil {
		t.Fatalf("LoadOrGenerateCerts: %v", err)
	}

	keyFiles := []string{
		filepath.Join(dir, "certs", "ca.key"),
		assets.ServerKeyPath,
		assets.ClientKeyPath,
	}

	for _, keyFile := range keyFiles {
		info, err := os.Stat(keyFile)
		if err != nil {
			t.Fatalf("stat %s: %v", keyFile, err)
		}
		perm := info.Mode().Perm()
		if perm != 0o600 {
			t.Errorf("%s has permissions %o, want 0600", filepath.Base(keyFile), perm)
		}
	}
}

func TestLoadOrGenerateCertsExpiredCertsRegenerate(t *testing.T) {
	dir := t.TempDir()

	// Generate valid certs first
	cfg := CertsConfig{DataDir: dir}
	_, err := LoadOrGenerateCerts(cfg)
	if err != nil {
		t.Fatalf("first LoadOrGenerateCerts: %v", err)
	}

	// Overwrite server cert with an expired one
	writeExpiredCert(t, filepath.Join(dir, "certs", "server.crt"))

	// Second run should detect expiration and regenerate
	assets, err := LoadOrGenerateCerts(cfg)
	if err != nil {
		t.Fatalf("second LoadOrGenerateCerts: %v", err)
	}

	if !assets.WasGenerated {
		t.Error("should regenerate when certs are expired")
	}

	// Verify regenerated cert is valid (not expired)
	cert := loadCertFromFile(t, assets.ServerCertPath)
	if time.Now().After(cert.NotAfter) {
		t.Error("regenerated cert should not be expired")
	}
}

func writeExpiredCert(t *testing.T, path string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "expired"},
		NotBefore:    time.Now().Add(-2 * time.Hour),
		NotAfter:     time.Now().Add(-1 * time.Hour), // expired 1 hour ago
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("creating expired cert: %v", err)
	}
	if err := writePEMFile(path, "CERTIFICATE", certDER); err != nil {
		t.Fatalf("writing expired cert: %v", err)
	}
}

func TestLoadOrGenerateCertsExpiredClientCertRegenerates(t *testing.T) {
	dir := t.TempDir()

	// Generate valid certs first
	cfg := CertsConfig{DataDir: dir}
	_, err := LoadOrGenerateCerts(cfg)
	if err != nil {
		t.Fatalf("first LoadOrGenerateCerts: %v", err)
	}

	// Overwrite client cert with an expired one (server cert is still valid)
	writeExpiredCert(t, filepath.Join(dir, "certs", "client.crt"))

	// Second run should detect client cert expiration and regenerate
	assets, err := LoadOrGenerateCerts(cfg)
	if err != nil {
		t.Fatalf("second LoadOrGenerateCerts: %v", err)
	}

	if !assets.WasGenerated {
		t.Error("should regenerate when client cert is expired")
	}

	// Verify regenerated client cert is valid (not expired)
	cert := loadCertFromFile(t, assets.ClientCertPath)
	if time.Now().After(cert.NotAfter) {
		t.Error("regenerated client cert should not be expired")
	}
}

func TestAcquireLockRecoversStaleLock(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, ".lock")

	// Create a stale lock file with old modification time
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("creating stale lock: %v", err)
	}
	f.Close()

	// Set modification time to well beyond the stale threshold
	staleTime := time.Now().Add(-10 * time.Minute)
	if err := os.Chtimes(lockPath, staleTime, staleTime); err != nil {
		t.Fatalf("setting stale lock mtime: %v", err)
	}

	// acquireLock should recover the stale lock and succeed
	unlock, err := acquireLock(dir)
	if err != nil {
		t.Fatalf("acquireLock should recover stale lock: %v", err)
	}
	unlock()
}

func TestLoadOrGenerateCertsConcurrentSafe(t *testing.T) {
	dir := t.TempDir()
	cfg := CertsConfig{DataDir: dir}

	const goroutines = 5
	var wg sync.WaitGroup
	errs := make([]error, goroutines)

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = LoadOrGenerateCerts(cfg)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d failed: %v", i, err)
		}
	}

	// Verify the final certs are valid and loadable
	assets, err := LoadOrGenerateCerts(cfg)
	if err != nil {
		t.Fatalf("final LoadOrGenerateCerts: %v", err)
	}
	// Should reuse since they now exist
	if assets.WasGenerated {
		t.Error("final call should reuse existing certs")
	}
}

// --- Task 3: NewTLSConfig tests ---

func generateTestCerts(t *testing.T) (*TLSAssets, string) {
	t.Helper()
	dir := t.TempDir()
	cfg := CertsConfig{DataDir: dir}
	assets, err := LoadOrGenerateCerts(cfg)
	if err != nil {
		t.Fatalf("LoadOrGenerateCerts: %v", err)
	}
	return assets, dir
}

func TestNewTLSConfigMinVersionTLS13(t *testing.T) {
	assets, _ := generateTestCerts(t)

	tlsCfg, err := NewTLSConfig(assets.CACertPath, assets.ServerCertPath, assets.ServerKeyPath)
	if err != nil {
		t.Fatalf("NewTLSConfig: %v", err)
	}

	if tlsCfg.MinVersion != tls.VersionTLS13 {
		t.Errorf("expected MinVersion TLS 1.3 (%d), got %d", tls.VersionTLS13, tlsCfg.MinVersion)
	}
}

func TestNewTLSConfigRequiresClientCert(t *testing.T) {
	assets, _ := generateTestCerts(t)

	tlsCfg, err := NewTLSConfig(assets.CACertPath, assets.ServerCertPath, assets.ServerKeyPath)
	if err != nil {
		t.Fatalf("NewTLSConfig: %v", err)
	}

	if tlsCfg.ClientAuth != tls.RequireAndVerifyClientCert {
		t.Errorf("expected RequireAndVerifyClientCert, got %v", tlsCfg.ClientAuth)
	}
}

func TestNewTLSConfigHasClientCAs(t *testing.T) {
	assets, _ := generateTestCerts(t)

	tlsCfg, err := NewTLSConfig(assets.CACertPath, assets.ServerCertPath, assets.ServerKeyPath)
	if err != nil {
		t.Fatalf("NewTLSConfig: %v", err)
	}

	if tlsCfg.ClientCAs == nil {
		t.Error("expected ClientCAs to be set")
	}
}

func TestNewTLSConfigHasServerCert(t *testing.T) {
	assets, _ := generateTestCerts(t)

	tlsCfg, err := NewTLSConfig(assets.CACertPath, assets.ServerCertPath, assets.ServerKeyPath)
	if err != nil {
		t.Fatalf("NewTLSConfig: %v", err)
	}

	if len(tlsCfg.Certificates) == 0 {
		t.Error("expected at least one server certificate")
	}
}

func TestTLSHandshakeValidClientAccepted(t *testing.T) {
	assets, _ := generateTestCerts(t)

	tlsCfg, err := NewTLSConfig(assets.CACertPath, assets.ServerCertPath, assets.ServerKeyPath)
	if err != nil {
		t.Fatalf("NewTLSConfig: %v", err)
	}

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	srv.TLS = tlsCfg
	srv.StartTLS()
	defer srv.Close()

	// Load client cert
	clientCert, err := tls.LoadX509KeyPair(assets.ClientCertPath, assets.ClientKeyPath)
	if err != nil {
		t.Fatalf("loading client cert: %v", err)
	}

	// Load CA cert for server verification
	caCertPEM, err := os.ReadFile(assets.CACertPath)
	if err != nil {
		t.Fatalf("reading CA cert: %v", err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCertPEM)

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				Certificates: []tls.Certificate{clientCert},
				RootCAs:      caCertPool,
			},
		},
	}

	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("request with valid client cert should succeed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestTLSHandshakeNoClientCertRejected(t *testing.T) {
	assets, _ := generateTestCerts(t)

	tlsCfg, err := NewTLSConfig(assets.CACertPath, assets.ServerCertPath, assets.ServerKeyPath)
	if err != nil {
		t.Fatalf("NewTLSConfig: %v", err)
	}

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	srv.TLS = tlsCfg
	srv.StartTLS()
	defer srv.Close()

	// Load CA cert for server verification but provide NO client cert
	caCertPEM, err := os.ReadFile(assets.CACertPath)
	if err != nil {
		t.Fatalf("reading CA cert: %v", err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCertPEM)

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: caCertPool,
			},
		},
	}

	_, err = client.Get(srv.URL)
	if err == nil {
		t.Error("request without client cert should fail")
	}
}

func TestTLSHandshakeWrongCARejected(t *testing.T) {
	assets, _ := generateTestCerts(t)

	tlsCfg, err := NewTLSConfig(assets.CACertPath, assets.ServerCertPath, assets.ServerKeyPath)
	if err != nil {
		t.Fatalf("NewTLSConfig: %v", err)
	}

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	srv.TLS = tlsCfg
	srv.StartTLS()
	defer srv.Close()

	// Generate a separate CA and client cert (different CA)
	otherDir := t.TempDir()
	otherAssets, err := LoadOrGenerateCerts(CertsConfig{DataDir: otherDir})
	if err != nil {
		t.Fatalf("generating other certs: %v", err)
	}

	// Use client cert signed by different CA
	clientCert, err := tls.LoadX509KeyPair(otherAssets.ClientCertPath, otherAssets.ClientKeyPath)
	if err != nil {
		t.Fatalf("loading other client cert: %v", err)
	}

	// Use original CA to verify server (so TLS handshake starts)
	caCertPEM, err := os.ReadFile(assets.CACertPath)
	if err != nil {
		t.Fatalf("reading CA cert: %v", err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCertPEM)

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				Certificates: []tls.Certificate{clientCert},
				RootCAs:      caCertPool,
			},
		},
	}

	_, err = client.Get(srv.URL)
	if err == nil {
		t.Error("request with client cert from wrong CA should fail")
	}
}
