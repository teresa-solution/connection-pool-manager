package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestHelper provides utilities for testing
type TestHelper struct {
	t        *testing.T
	tempDir  string
	certFile string
	keyFile  string
}

// NewTestHelper creates a new test helper
func NewTestHelper(t *testing.T) *TestHelper {
	tempDir, err := os.MkdirTemp("", "main_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	return &TestHelper{
		t:        t,
		tempDir:  tempDir,
		certFile: filepath.Join(tempDir, "cert.pem"),
		keyFile:  filepath.Join(tempDir, "key.pem"),
	}
}

// Cleanup removes temporary files
func (h *TestHelper) Cleanup() {
	if err := os.RemoveAll(h.tempDir); err != nil {
		h.t.Errorf("Failed to cleanup temp dir: %v", err)
	}
}

// CreateTestCertificates generates self-signed certificates for testing
func (h *TestHelper) CreateTestCertificates() error {
	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"Test Company"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"Test City"},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1)},
		DNSNames:    []string{"localhost"},
	}

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return err
	}

	// Save certificate to file
	certOut, err := os.Create(h.certFile)
	if err != nil {
		return err
	}
	defer certOut.Close()

	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return err
	}

	// Save private key to file
	keyOut, err := os.Create(h.keyFile)
	if err != nil {
		return err
	}
	defer keyOut.Close()

	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return err
	}

	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyDER}); err != nil {
		return err
	}

	return nil
}

// GetCertPaths returns the paths to the test certificates
func (h *TestHelper) GetCertPaths() (string, string) {
	return h.certFile, h.keyFile
}

// CreateInvalidCertFiles creates files that are not valid certificates
func (h *TestHelper) CreateInvalidCertFiles() error {
	// Create invalid cert file
	if err := os.WriteFile(h.certFile, []byte("invalid cert content"), 0644); err != nil {
		return err
	}

	// Create invalid key file
	if err := os.WriteFile(h.keyFile, []byte("invalid key content"), 0644); err != nil {
		return err
	}

	return nil
}
