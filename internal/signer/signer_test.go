package signer

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	u "net/url"
)

// createSigner creates signer object with passed or default arguments
func createSigner(t *testing.T, args ...map[string]any) (*Signer, error) {
	t.Helper()

	dir := t.TempDir()

	// Default values
	keyPath := filepath.Join(dir, "key.key")
	certPath := filepath.Join(dir, "ca.cert")
	keyType := ECDSAKeyType
	var opts []Option

	var kwargs map[string]any
	if len(args) == 0 {
		kwargs = make(map[string]any)
	} else {
		kwargs = args[0]
	}

	// If `keyPath` is set, then change variables value
	if keyPathAny, ok := kwargs["keyPath"]; ok {
		keyPath, ok = keyPathAny.(string)
		if !ok {
			return nil, errors.New("keyPath argument must have type string")
		}
	}

	// If `certPath` is set, then change variables value
	if certPathAny, ok := kwargs["certPath"]; ok {
		certPath, ok = certPathAny.(string)
		if !ok {
			return nil, errors.New("certPath argument must have type string")
		}
	}

	// If `keyTypeAny` is set, then change variables value
	if keyTypeAny, ok := kwargs["keyType"]; ok {
		keyType, ok = keyTypeAny.(KeyType)
		if !ok {
			return nil, errors.New("keyType argument must have type KeyType")
		}
	}

	// If `optsAny` is set, then change variables value
	if optsAny, ok := kwargs["opts"]; ok {
		opts, ok = optsAny.([]Option)
		if !ok {
			return nil, errors.New("opts argument must have type []Option")
		}
	}

	return New(certPath, keyPath, keyType, opts...)
}

// KeyType to sting conversion
func TestKeyTypeString(t *testing.T) {
	for k, v := range map[KeyType]string{
		RSAKeyType:     "rsa",
		ED25519KeyType: "ed25519",
		ECDSAKeyType:   "ecdsa",
		-1:             "unknown",
	} {
		s := k.String()
		if s != v {
			t.Errorf("Invalid conversion, got `%s`, expected `%s`", s, v)
		}
	}
}

// Create Signer
func TestCreateSigner(t *testing.T) {
	check := func(keyType KeyType) func(t *testing.T) {
		return func(t *testing.T) {
			_, err := createSigner(t, map[string]any{
				"keyType": keyType,
			})

			if err != nil {
				t.Fatalf("Failed to create a signer: %s", err)
			}
		}
	}

	t.Run("ECDSA", check(ECDSAKeyType))
	t.Run("RSA", check(RSAKeyType))
	t.Run("ED25519", check(ED25519KeyType))
}

// Create a Signer with a logger
func TestCreateSignerWithLogger(t *testing.T) {
	// Create io.ReadWriter to check logs
	var b strings.Builder
	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	logger := slog.New(slog.NewTextHandler(&b, opts))

	// Creating a new signer with the logger created before
	_, err := createSigner(t, map[string]any{
		"opts": []Option{WithLogger(logger)},
	})
	if err != nil {
		t.Fatalf("Failed to create a signer: %s", err)
	}

	// Check for success string in the logs
	if !strings.Contains(b.String(), "Successfully generated and saved signer") {
		t.Fatalf("Success string isn't written to the logger")
	}
}

// Create a Signer with a cache
func TestCreateSignerWithCache(t *testing.T) {
	// Creating a new signer with a leaf certificate cache of the specified size
	cacheSize := 128
	signer, err := createSigner(t, map[string]any{
		"opts": []Option{WithCache(cacheSize)},
	})

	if err != nil {
		t.Fatalf("Failed to create a signer: %s", err)
	}

	if signer.cache == nil {
		t.Fatalf("Failed to create a LRU cache")
	}
}

// Create a Signer with a cache with size 0 or smaller
func TestCreateSignerWithNoCache(t *testing.T) {
	signer, err := createSigner(t, map[string]any{
		"opts": []Option{WithCache(0)},
	})

	if err != nil {
		t.Fatalf("Failed to create a signer: %s", err)
	}

	if signer.cache != nil {
		t.Fatalf("Cache was created with an invalid size")
	}
}

// Load certificate and private key, but private key doesn't match certificate
func TestCreateSignerWithInvalidPk(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "key.key")
	certPath := filepath.Join(dir, "ca.cert")

	signer, err := createSigner(t, map[string]any{
		"keyPath":  keyPath,
		"certPath": certPath,
	})
	if err != nil {
		t.Fatalf("Failed to create a signer: %s", err)
	}

	// Generate a new private key
	pk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate a new private key: %s", err)
	}

	// Change signers key and save certificate and key
	signer.pk = pk
	err = signer.Save(certPath, keyPath)
	if err != nil {
		t.Fatalf("Failed to save a new private key: %s", err)
	}

	// Loading signer from the saved private key and certificate
	signerNew, err := createSigner(t, map[string]any{
		"keyPath":  keyPath,
		"certPath": certPath,
	})
	if err != nil {
		t.Fatalf("Failed to create a signer: %s", err)
	}

	// If signer is equal to previous, than it's loaded together with wrong private key
	if signerNew.cert.Equal(signer.cert) {
		t.Fatal("Loaded certificate that doesn't match private key")
	}

	// Check that new certificate matches loaded private key
	pub := signerNew.cert.PublicKey
	if pub, ok := pub.(*ecdsa.PublicKey); !ok || !pub.Equal(&pk.PublicKey) {
		t.Fatal("New certificates public key doesn't match loaded private key's public key")
	}
}

// Create a Signer with invalid pathes
func TestCreateSignerWithInvalidCertPath(t *testing.T) {
	_, err := createSigner(t, map[string]any{
		"certPath": "",
	})

	if err == nil {
		t.Fatalf("Signer was created without an error with an invalid certificate path")
	}

	if !errors.Is(err, ErrRootCertificateFileCreationFailed) {
		t.Fatalf("Error was returned, but it's not because of the an invalid certificate path")
	}
}

// Create a Signer with invalid pathes
func TestCreateSignerWithInvalidKeyPath(t *testing.T) {
	_, err := createSigner(t, map[string]any{
		"keyPath": "",
	})

	if err == nil {
		t.Fatalf("Signer was created without an error with an invalid private key path")
	}

	if !errors.Is(err, ErrPKFileCreationFailed) {
		t.Fatalf("Error was returned, but it's not because of the an invalid private key path")
	}
}

// Load a root certificate
func TestLoadCertificate(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "ca.cert")

	signer, err := createSigner(t, map[string]any{
		"certPath": certPath,
	})

	if err != nil {
		t.Fatalf("Failed to create a signer: %s", err)
	}

	// Clear signer certificate and load again
	cert := *signer.cert
	signer.cert = nil
	err = signer.LoadCA(certPath)
	if err != nil {
		t.Fatalf("Failed to load a root certificate: %s", err)
	}

	// Error if loaded certificate isn't equal to the saved one
	if !cert.Equal(signer.cert) {
		t.Fatalf("Loaded certificate isn't equal to the saved root certificate")
	}
}

// Load a root certificate with an invalid PEM block
func TestInvalidPEMLoadCertificate(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "ca.cert")

	signer, err := createSigner(t, map[string]any{
		"certPath": certPath,
	})

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
	signer, err := createSigner(t)
	if err != nil {
		t.Fatalf("Failed to create a signer: %s", err)
	}

	err = signer.GenerateCA()
	if err != nil {
		fmt.Errorf("Error during root certificate generation")
	}

	// Check if generated certificate is a valid root certificate
	if signer.cert == nil || !signer.cert.IsCA {
		t.Fatalf("Generated root certificate is invalid")
	}
}

// Load a private key file
func TestLoadPK(t *testing.T) {
	check := func(keyType KeyType) func(t *testing.T) {
		return func(t *testing.T) {
			dir := t.TempDir()
			keyPath := filepath.Join(dir, "key.key")

			signer, err := createSigner(t, map[string]any{
				"keyPath": keyPath,
				"keyType": keyType,
			})

			if err != nil {
				t.Fatalf("Failed to create a signer: %s", err)
			}

			// Clear signer's key and reload it
			pk := signer.pk
			signer.pk = nil
			err = signer.LoadPK(keyPath)
			if err != nil {
				t.Fatalf("Failed to load a private key: %s", err)
			}

			// Check whether loaded private key equals to the saved one
			switch pk := pk.(type) {
			case *rsa.PrivateKey:
				spk, ok := signer.pk.(*rsa.PrivateKey)
				if !ok || !pk.Equal(spk) {
					t.Fatalf("Loaded private key isn't equal to the saved private key")
				}
			case *ecdsa.PrivateKey:
				spk, ok := signer.pk.(*ecdsa.PrivateKey)
				if !ok || !pk.Equal(spk) {
					t.Fatalf("Loaded private key isn't equal to the saved private key")
				}
			case ed25519.PrivateKey:
				spk, ok := signer.pk.(ed25519.PrivateKey)
				if !ok || !pk.Equal(spk) {
					t.Fatalf("Loaded private key isn't equal to the saved private key")
				}
			default:
				t.Fatalf("Private key type isn't supported")
			}
		}
	}

	t.Run("RSA", check(RSAKeyType))
	t.Run("ECDSA", check(ECDSAKeyType))
	t.Run("ED25519", check(ED25519KeyType))
}

// Load a private key file with an invalid PEM block
func TestInvalidPEMLoadPK(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "key.key")

	signer, err := createSigner(t, map[string]any{
		"keyPath": keyPath,
	})

	if err != nil {
		t.Fatalf("Failed to create a signer: %s", err)
	}

	// Write invalid PEM to the key file
	err = os.WriteFile(keyPath, []byte("Invalid PEM"), 0644)
	if err != nil {
		t.Fatalf("Failed to write bytes to PK file: %s", err)
	}

	err = signer.LoadPK(keyPath)
	if err == nil {
		t.Fatalf("Didn't return an error on invalid certificate")
	}

	// Check that error is for invalid PEM block
	if !errors.Is(err, ErrInvalidPKPEMBlock) {
		t.Fatalf("Error returned wasn't for invalid PEM block: %s", err)
	}
}

// Generate and assign a new random private key
func TestGeneratePK(t *testing.T) {
	signer, err := createSigner(t)
	if err != nil {
		t.Fatalf("Failed to create a signer: %s", err)
	}

	err = signer.GeneratePK()
	if err != nil {
		t.Fatalf("Failed to generate a new private key: %s", err)
	}

	if signer.pk == nil {
		t.Fatalf("Generated private key is invalid")
	}
}

// Save a private key and a root certficate of a Signer with ecdsa key type
func TestSignerSave(t *testing.T) {
	check := func(keyType KeyType) func(t *testing.T) {
		return func(t *testing.T) {
			dir := t.TempDir()
			certPath := filepath.Join(dir, "ca.cert")
			keyPath := filepath.Join(dir, "key.key")

			signer, err := createSigner(t, map[string]any{
				"certPath": certPath,
				"keyPath":  keyPath,
				"keyType":  keyType,
			})
			if err != nil {
				t.Fatalf("Failed to create a signer: %s", err)
			}

			signerNew, err := createSigner(t, map[string]any{
				"certPath": certPath,
				"keyPath":  keyPath,
				"keyType":  keyType,
			})
			if err != nil {
				t.Fatalf("Failed to create a signer: %s", err)
			}

			// Check whether loaded signer's key is equal to the saved ones
			switch pk := signerNew.pk.(type) {
			case *rsa.PrivateKey:
				spk, ok := signer.pk.(*rsa.PrivateKey)
				if !ok || !pk.Equal(spk) {
					t.Fatalf("Loaded private key isn't equal to the saved private key")
				}
			case *ecdsa.PrivateKey:
				spk, ok := signer.pk.(*ecdsa.PrivateKey)
				if !ok || !pk.Equal(spk) {
					t.Fatalf("Loaded private key isn't equal to the saved private key")
				}
			case *ed25519.PrivateKey:
				spk, ok := signer.pk.(*ed25519.PrivateKey)
				if !ok || !pk.Equal(spk) {
					t.Fatalf("Loaded private key isn't equal to the saved private key")
				}
			}

			// Check whether loaded signer's certificate is equal to the saved ones
			if !signerNew.cert.Equal(signer.cert) {
				t.Fatalf("Loaded certificate isn't equal to saved")
			}
		}
	}

	t.Run("RSA", check(RSAKeyType))
	t.Run("ED25519", check(ED25519KeyType))
	t.Run("ECDSA", check(ECDSAKeyType))
}

// Generate a leaf certificate
func TestGenerateLeafCertificate(t *testing.T) {
	signer, err := createSigner(t)
	if err != nil {
		t.Fatalf("Failed to create a signer: %s", err)
	}

	url, err := u.Parse("https://example.com")
	if err != nil {
		t.Fatalf("Failed to parse example URL: %s", err)
	}

	leafCert, err := signer.GenerateLeafCertificate(*url)
	if err != nil {
		t.Fatalf("Failed to generate a leaf certificate: %s", err)
	}

	if !bytes.Equal(leafCert.Certificate[1], signer.cert.Raw) {
		t.Fatalf("Generated leaf certificate is invalid")
	}
}

// Generate a leaf certificate with cache
func TestGenerateLeafCertificateWithCache(t *testing.T) {
	cacheSize := 128
	signer, err := createSigner(t, map[string]any{
		"opts": []Option{WithCache(cacheSize)},
	})
	if err != nil {
		t.Fatalf("Failed to create a signer: %s", err)
	}

	// Check that cache is created
	if signer.cache == nil {
		t.Fatal("Leaf cache wasn't created and no error was returned")
	}

	// Create an example URL
	url, err := u.Parse("https://example.com")
	if err != nil {
		t.Fatalf("Failed to parse example URL: %s", err)
	}

	// Generate a leaf certificate for the exmaple URL
	leafCert, err := signer.GenerateLeafCertificate(*url)
	if err != nil {
		t.Fatalf("Failed to generate a leaf certificate: %s", err)
	}

	// Check that leaf certificate containes CA certificate
	if !bytes.Equal(leafCert.Certificate[1], signer.cert.Raw) {
		t.Fatalf("Generated leaf certificate is invalid")
	}

	// Get a leaf certificate from cache
	leafCertFromCache, ok := signer.cache.Get(url.Hostname())
	if !ok {
		t.Fatalf("Leaf certificate wasn't saved to cache")
	}

	// Check that a leaf certificate from cache and returned by function are equal
	if !slices.EqualFunc(leafCert.Certificate, leafCertFromCache.Certificate, bytes.Equal) {
		t.Fatalf("Certificate saved to cache isn't equal to the returned one")
	}

	// Get a leaf certificate again
	leafCertFromCache, err = signer.GenerateLeafCertificate(*url)
	if err != nil {
		t.Fatalf("Failed to generate a leaf certificate: %s", err)
	}

	// Check that a leaf certificate from cache and returned by function are equal
	if !slices.EqualFunc(leafCert.Certificate, leafCertFromCache.Certificate, bytes.Equal) {
		t.Fatalf("Certificate returned from cache isn't equal to the cached one")
	}
}
