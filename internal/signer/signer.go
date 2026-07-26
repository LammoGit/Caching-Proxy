// Package signer implements Signer objects which implements methods for
// loading and generating private keys, and root and leaf certificates
package signer

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	u "net/url"
	"os"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
)

/* Errors */

var (
	ErrCertIsNil                         = errors.New("certificate isn't initialized")
	ErrPKIsNil                           = errors.New("private key isn't initialized")
	ErrPKIsNotRSA                        = errors.New("loaded PK is not an RSA PK")
	ErrPKFileReadFailed                  = errors.New("failed to read bytes from the private key file")
	ErrCAFileReadFailed                  = errors.New("failed to read bytes from the certificate file")
	ErrInvalidPKPEMBlock                 = errors.New("loaded private key is invalid")
	ErrInvalidCertificatePEMBlock        = errors.New("loaded certificate is invalid")
	ErrCAParsingFailed                   = errors.New("failed to parse root certificate")
	ErrPKParsingFailed                   = errors.New("failed to parse private key from PKCS#8")
	ErrRootCertificateCreationFailed     = errors.New("failed to create root certificate")
	ErrFailedPKMarshal                   = errors.New("failed to marshal PKCS#8 private key")
	ErrRootCertificateFileCreationFailed = errors.New("failed to create root certificate file")
	ErrCertificatePEMEncodingFailed      = errors.New("failed to write root certificate using PEM encoding to a file")
	ErrPKFileCreationFailed              = errors.New("failed to open/create private key file")
	ErrPKPEMEncodingFailed               = errors.New("failed to write private key using PEM encoding to a file")
	ErrPKGenerationFailed                = errors.New("failed to generate RSA private key")
	ErrCacheInitFailed                   = errors.New("failed to initialize LRU cache")
	ErrLeafCertificateGenerationFailed   = errors.New("failed to create leaf certificate")
)

/* Signer Type */

// Signer generates certificates and primary keys
type Signer struct {
	Cert   *x509.Certificate
	Pk     *rsa.PrivateKey
	logger *slog.Logger
	cache  *lru.Cache[string, *tls.Certificate]
	mu     sync.RWMutex
}

// Option represents functions that are used for initial configuration of a Signer object
type Option func(*Signer)

/* Signer Options */

// WithLogger Option that is used to set a signer.logger to a Signer
// if passed signer.logger pointer is nil, then logging messages are discarded
func WithLogger(logger *slog.Logger) Option {
	return func(signer *Signer) {
		if logger != nil {
			signer.logger = logger
		}
	}
}

// WithCache Option that is used to set a LRU cache size to a Signer
// if given size argument is zero, then no cache is used
func WithCache(size int) Option {
	return func(signer *Signer) {
		cache, err := lru.New[string, *tls.Certificate](size)
		if err == nil {
			signer.cache = cache
		} else {
			signer.cache = nil
		}
	}
}

/* Signer Methods */

// New creates a new Signer object and returns pointer to it
// accepts pathes to certificate and key files, PK size in bits, and Option functions for configuration
func New(certPath, keyPath string, keySize int, opts ...Option) (signer *Signer, err error) {
	signer = &Signer{
		logger: slog.New(slog.DiscardHandler),
	}

	for _, opt := range opts {
		opt(signer)
	}

	// Try to load both root certificate and private key
	if signer.LoadCA(certPath) == nil &&
		signer.LoadPK(keyPath) == nil &&
		signer.Cert != nil && signer.Pk != nil &&
		signer.Cert.IsCA && signer.Cert.BasicConstraintsValid {

		pub, ok := signer.Cert.PublicKey.(*rsa.PublicKey)
		if ok && pub.Equal(&signer.Pk.PublicKey) {
			signer.logger.Debug("Loaded existing CA and private key")
			return
		}
	}

	// Generate a new private key
	if err = signer.GeneratePK(keySize); err != nil {
		return
	}

	// Generate a new root certificate
	signer.GenerateCA()

	// Save both certificate and PK at given pathes
	if err = signer.Save(certPath, keyPath); err != nil {
		return
	}

	signer.logger.Debug("Successfully generated and saved signer")
	return
}

// LoadCA loads a root certificate from the given path
func (signer *Signer) LoadCA(certPath string) (err error) {
	// Read the certificate file bytes
	certPEMBytes, err := os.ReadFile(certPath)
	if err != nil {
		err = fmt.Errorf("%w: %s", ErrCAFileReadFailed, err)
		signer.logger.Debug("", "error", err)
		return
	}

	// Parse certificate PEM bytes into certificate object
	certBlock, _ := pem.Decode(certPEMBytes)
	if certBlock == nil {
		err = ErrInvalidCertificatePEMBlock
		signer.logger.Error("", "error", err)
		return
	}

	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		err = fmt.Errorf("%w: %s", ErrCAParsingFailed, err)
		signer.logger.Error("", "error", err)
		return
	}

	signer.Cert = cert
	return
}

// GenerateCA generates and assigns a new root certificate
func (signer *Signer) GenerateCA() error {
	// Generate random serial number for the root certificate
	serialNumber, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))

	// Create the root certificate template
	certTemplate := &x509.Certificate{
		Version:      3,
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: "Caching Proxy Root CA",
		},
		NotBefore:             time.Now().Add(-24 * time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}

	// Create self-signed root certificate
	certBytes, err := x509.CreateCertificate(
		rand.Reader,
		certTemplate,
		certTemplate,
		&signer.Pk.PublicKey,
		signer.Pk,
	)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrRootCertificateCreationFailed, err)
	}

	cert, err := x509.ParseCertificate(certBytes)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrCAParsingFailed, err)
	}

	signer.Cert = cert
	return nil
}

// LoadPK loads a private key from the given file path
func (signer *Signer) LoadPK(keyPath string) (err error) {
	// Reading bytes from the private key file
	pkPEMBytes, err := os.ReadFile(keyPath)
	if err != nil {
		err = fmt.Errorf("%w: %s", ErrPKFileReadFailed, err)
		signer.logger.Debug("", "error", err)
		return
	}

	// Decode private key from PEM bytes to RSA PK
	pkBlock, _ := pem.Decode(pkPEMBytes)
	if pkBlock == nil {
		err = ErrInvalidPKPEMBlock
		signer.logger.Error("", "error", err)
		return
	}

	pk, err := x509.ParsePKCS8PrivateKey(pkBlock.Bytes)
	if err != nil {
		err = fmt.Errorf("%w: %s", ErrPKParsingFailed, err)
		signer.logger.Error("", "error", err)
		return
	}
	var ok bool
	signer.Pk, ok = pk.(*rsa.PrivateKey)
	if !ok {
		err = ErrPKIsNotRSA
		signer.logger.Error("", "error", err)
		return
	}

	signer.logger.Debug("Successfully loaded private key")
	return
}

// GeneratePK generates and assigns a new random RSA private key of the given size
func (signer *Signer) GeneratePK(keySize int) error {
	// Generate random private key
	pk, err := rsa.GenerateKey(rand.Reader, keySize)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrPKGenerationFailed, err)
	}
	signer.Pk = pk
	return nil
}

// Save writes certificate and private key to the given pathes
func (signer *Signer) Save(certPath, keyPath string) error {
	if signer.Cert == nil {
		return ErrCertIsNil
	}
	if signer.Pk == nil {
		return ErrPKIsNil
	}

	// Creating or truncating the certificate file
	certFile, err := os.Create(certPath)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrRootCertificateFileCreationFailed, err)
	}
	defer certFile.Close()

	// Creating PEM block object for certificate
	certBlock := pem.Block{
		Type:  "CERTIFICATE",
		Bytes: signer.Cert.Raw,
	}

	// Writing encoded PEM certificate block to the certificate file
	err = pem.Encode(certFile, &certBlock)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrCertificatePEMEncodingFailed, err)
	}

	// Private key bytes
	pkBytes, err := x509.MarshalPKCS8PrivateKey(signer.Pk)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrFailedPKMarshal, err)
	}

	// Creating or truncating the private key file
	keyFile, err := os.Create(keyPath)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrPKFileCreationFailed, err)
	}
	defer keyFile.Close()

	// Creating PEM block object for private key
	pkBlock := pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: pkBytes,
	}

	// Writing encoded PEM private key block to the PK file
	err = pem.Encode(keyFile, &pkBlock)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrPKPEMEncodingFailed, err)
	}

	return nil
}

// GenerateLeafCertificate generates a new leaf certificate for the given URL of the given size
func (signer *Signer) GenerateLeafCertificate(url u.URL, keySize int) (*tls.Certificate, error) {
	hostname := url.Hostname()

	// Check if certificate is in cache and if it exists
	if signer.cache != nil {
		signer.mu.RLock()
		cert, ok := signer.cache.Get(hostname)
		signer.mu.RUnlock()
		if ok {
			signer.logger.Debug(fmt.Sprintf("Leaf certificate cache hit for %s", hostname))
			return cert, nil
		}
	}

	// Generate a new random private key
	pk, err := rsa.GenerateKey(rand.Reader, keySize)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrPKGenerationFailed, err)
	}

	// Generate a random serial number for a new leaf certificate
	serialNumber, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))

	// Generate a leaf certificate
	cert := x509.Certificate{
		Version:      3,
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: hostname,
		},
		NotBefore:   time.Now().Add(-time.Hour),
		NotAfter:    time.Now().AddDate(1, 0, 0),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:    []string{hostname},
	}

	// Sign the leaf certificate and get its bytes
	certBytes, err := x509.CreateCertificate(
		rand.Reader,
		&cert,
		signer.Cert,
		&pk.PublicKey,
		signer.Pk,
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrLeafCertificateGenerationFailed, err)
	}

	// Create certificate object for TLS connection
	tlsCert := tls.Certificate{
		Certificate: [][]byte{certBytes, signer.Cert.Raw},
		PrivateKey:  pk,
	}

	// Add TLS certificate to cache if it exists
	if signer.cache != nil {
		signer.mu.Lock()
		signer.cache.Add(hostname, &tlsCert)
		signer.mu.Unlock()
	}

	return &tlsCert, nil
}
