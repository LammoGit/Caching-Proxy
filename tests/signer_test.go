package test

import (
	"fmt"
	"os"
	"testing"

	s "github.com/LammoGit/Caching-Proxy/internal/signer"
)

func getSignerFilePathes(t *testing.T) (certPath, keyPath string) {
	t.Helper()

	//
	certFile, err := os.CreateTemp("", "test-certificate-*.cert")
	if err != nil {
		t.Fatalf("Couldn't create a temporary certificate file")
	}
	certPath = certFile.Name()
	certFile.Close()
	os.Remove(certPath)

	//
	keyFile, err := os.CreateTemp("", "test-keylist-*.txt")
	if err != nil {
		t.Fatalf("Couldn't create a temporary key file")
	}
	keyPath = keyFile.Name()
	keyFile.Close()
	os.Remove(keyPath)
	return
}

// Create Signer
func TestCreateSigner(t *testing.T) {
	certPath, keyPath := getSignerFilePathes(t)
	_, err := s.New(certPath, keyPath)
	if err != nil {
		t.Fatalf("Failed to create a signer: %s", err)
	}
	defer os.Remove(certPath)
	defer os.Remove(keyPath)
}

// Load Signer
func TestLoadSigner(t *testing.T) {
	certPath, keyPath := getSignerFilePathes(t)
	signer, err := s.New(certPath, keyPath)
	if err != nil {
		t.Fatalf("Failed to create a signer: %s", err)
	}
	defer os.Remove(certPath)
	defer os.Remove(keyPath)

	// Saving signer
	err = signer.Save(certPath, keyPath)
	if err != nil {
		t.Fatalf("Failed to save a signer: %s", err)
	}

	// Load signer
	signerLoaded, err := s.New(certPath, keyPath)
	if err != nil {
		t.Fatalf("Failed to load a signer: %s", err)
	}

	// Different certificate is fine
	if !signerLoaded.Pk.Equal(signer.Pk) {
		t.Fatalf("%s", fmt.Sprintf("Saved and loaded signers have different private keys"))
	}
}
