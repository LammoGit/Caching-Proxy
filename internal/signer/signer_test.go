package signer

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	u "net/url"
)

// Test Config
const testPKSize = 2048

// Create Signer
func TestCreateSigner(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "ca.cert")
	keyPath := filepath.Join(dir, "ca.key")

	_, err := New(certPath, keyPath, testPKSize)
	if err != nil {
		t.Fatalf("Failed to create a signer: %s", err)
	}
}

// Create a Signer with a logger
func TestCreateSignerWithLogger(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "ca.cert")
	keyPath := filepath.Join(dir, "ca.key")

	var b strings.Builder
	logger := slog.New(slog.NewTextHandler(&b, nil))
	_, err := New(certPath, keyPath, testPKSize, WithLogger(logger))
	if err != nil {
		t.Fatalf("Failed to create a signer: %s", err)
	}

	if !strings.Contains(b.String(), "Successfully generated and saved signer") {
		t.Fatalf("Success string isn't written to the logger")
	}
}

// Create a Signer with a cache
func TestCreateSignerWithCache(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "ca.cert")
	keyPath := filepath.Join(dir, "ca.key")

	signer, err := New(certPath, keyPath, testPKSize, WithCache(8))
	if err != nil {
		t.Fatalf("Failed to create a signer: %s", err)
	}

	if signer.cache == nil {
		t.Fatalf("Failed to create a LRU cache")
	}
}

// Create a Signer with a cache with size 0 or smaller
func TestCreateSignerWithNoCache(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "ca.cert")
	keyPath := filepath.Join(dir, "ca.key")

	signer, err := New(certPath, keyPath, testPKSize, WithCache(-1))
	if err != nil {
		t.Fatalf("Failed to create a signer: %s", err)
	}

	if signer.cache == nil {
		t.Fatalf("Failed to create a LRU cache")
	}
}

// Load a root certificate
func TestLoadCertificate(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "ca.cert")
	keyPath := filepath.Join(dir, "ca.key")

	signer, err := New(certPath, keyPath, testPKSize)
	if err != nil {
		t.Fatalf("Failed to create a signer: %s", err)
	}

	cert := *signer.Cert
	signer.Cert = nil
	err = signer.LoadCA(certPath)
	if err != nil {
		t.Fatalf("Failed to load a root certificate: %s", err)
	}

	if !cert.Equal(signer.Cert) {
		t.Fatalf("Loaded certificate isn't equal to the saved root certificate")
	}
}

// Load a root certificate with an invalid PEM block
func TestIvalidPEMLoadCertificate(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "ca.cert")
	keyPath := filepath.Join(dir, "ca.key")

	signer, err := New(certPath, keyPath, testPKSize)
	if err != nil {
		t.Fatalf("Failed to create a signer: %s", err)
	}

	err = os.WriteFile(certPath, []byte("Invalid PEM"), 0644)
	if err != nil {
		t.Fatalf("Failed to write to a file: %s", err)
	}

	err = signer.LoadCA(certPath)
	if err == nil {
		t.Fatalf("Didn't return an error on invalid certificate")
	}

	if !errors.Is(err, ErrInvalidCertificatePEMBlock) {
		t.Fatalf("Error returned wasn't for invalid PEM block: %s", err)
	}
}

// Generate and assign a new root certificate
func TestGenerateCertificate(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "ca.cert")
	keyPath := filepath.Join(dir, "ca.key")

	signer, err := New(certPath, keyPath, testPKSize)
	if err != nil {
		t.Fatalf("Failed to create a signer: %s", err)
	}

	err = signer.GenerateCA()
	if err != nil {
		t.Fatalf("Failed to generate a new root certificate: %s", err)
	}

	if signer.Cert == nil || !signer.Cert.IsCA {
		t.Fatalf("Generated root certificate is invalid")
	}
}

// Load a private key file
func TestLoadPK(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "ca.cert")
	keyPath := filepath.Join(dir, "ca.key")

	signer, err := New(certPath, keyPath, testPKSize)
	if err != nil {
		t.Fatalf("Failed to create a signer: %s", err)
	}

	pk := *signer.Pk
	signer.Pk = nil
	err = signer.LoadPK(keyPath)
	if err != nil {
		t.Fatalf("Failed to load a private key: %s", err)
	}

	if !pk.Equal(signer.Pk) {
		t.Fatalf("Loaded private key isn't equal to the saved private key")
	}
}

// Load a private key file with an invalid PEM block
func TestInvalidPEMLoadPK(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "ca.cert")
	keyPath := filepath.Join(dir, "ca.key")

	signer, err := New(certPath, keyPath, testPKSize)
	if err != nil {
		t.Fatalf("Failed to create a signer: %s", err)
	}

	err = os.WriteFile(keyPath, []byte("Invalid PEM"), 0644)

	err = signer.LoadPK(keyPath)
	if err == nil {
		t.Fatalf("Didn't return an error on invalid certificate")
	}

	if !errors.Is(err, ErrInvalidPKPEMBlock) {
		t.Fatalf("Error returned wasn't for invalid PEM block: %s", err)
	}
}

// Generate and assign a new random private key
func TestGeneratePK(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "ca.cert")
	keyPath := filepath.Join(dir, "ca.key")

	signer, err := New(certPath, keyPath, testPKSize)
	if err != nil {
		t.Fatalf("Failed to create a signer: %s", err)
	}

	err = signer.GeneratePK(testPKSize)
	if err != nil {
		t.Fatalf("Failed to generate a new private key: %s", err)
	}

	if signer.Pk == nil || signer.Pk.Size() != testPKSize {
		t.Fatalf("Generated private key is invalid")
	}
}

// Save a private key and a root certficate of a Signer
func TestSignerSave(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "ca.cert")
	keyPath := filepath.Join(dir, "ca.key")

	signer, err := New(certPath, keyPath, testPKSize)
	if err != nil {
		t.Fatalf("Failed to create a signer: %s", err)
	}

	signerNew, err := New(certPath, keyPath, testPKSize)
	if err != nil {
		t.Fatalf("Failed to create a signer: %s", err)
	}
	if !signerNew.Pk.Equal(signer.Pk) || !signerNew.Cert.Equal(signer.Cert) {
		t.Fatalf("Loaded certificate and private key aren't equal to saved")
	}
}

// Generate a leaf certificate
func TestGenerateLeafCertificate(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "ca.cert")
	keyPath := filepath.Join(dir, "ca.key")

	signer, err := New(certPath, keyPath, testPKSize)
	if err != nil {
		t.Fatalf("Failed to create a signer: %s", err)
	}

	url, err := u.Parse("https://example.com")
	if err != nil {
		t.Fatalf("Failed to parse example URL: %s", err)
	}
	leafCert, err := signer.GenerateLeafCertificate(*url, testPKSize)
	if err != nil {
		t.Fatalf("Failed to generate a leaf certificate: %s", err)
	}
	if !bytes.Equal(leafCert.Certificate[1], signer.Cert.Raw) {
		t.Fatalf("Generated leaf certificate is invalid")
	}
}
