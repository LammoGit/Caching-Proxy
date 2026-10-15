// Package signer implements Signer objects which implements methods for
// loading and generating private keys, and root and leaf certificates
package signer

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log/slog"
	"math/big"
	u "net/url"
	"os"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
)

// Signer generates certificates and primary keys
type Signer struct {
	cert    *x509.Certificate                    // Root certificate
	pk      any                                  // Private key with which certificate is signed
	logger  *slog.Logger                         // Attached slog logger
	cache   *lru.Cache[string, *tls.Certificate] // Internal cache for storing leaf certificates
	keyType KeyType                              // Type of the private key
}

// New creates a new Signer object and returns pointer to it
// certPath path to an existing certificate file or where it should be created
// keyPath path to an existing private key file or where it should be created
// keyType expected type of key to be generated
// opts signer Options which are run at the start of signer object creation
// If private key is loaded and it matches certificate's public key, then keyType is ignored
func New(certPath, keyPath string, keyType KeyType, opts ...Option) (signer *Signer, err error) {
	signer = &Signer{
		logger:  slog.New(slog.DiscardHandler),
		keyType: keyType,
	}

	// Run Options
	for _, opt := range opts {
		opt(signer)
	}

	// Try to load private key from the given path
	err = signer.LoadPK(keyPath)
	if err == nil && signer.pk != nil {
		// Try to load certificate from the given path
		err = signer.LoadCA(certPath)

		// Check whether loaded certificate is valid
		if err == nil &&
			signer.cert != nil &&
			signer.cert.BasicConstraintsValid && /* Check basic constrains are validity */
			signer.cert.NotAfter.After(time.Now()) /* Check expiration */ {

			// Check whether signer's public key matches loaded private key
			pub := signer.cert.PublicKey
			switch pk := signer.pk.(type) {
			case *rsa.PrivateKey:
				if pub, ok := pub.(*rsa.PublicKey); ok && pub.Equal(&pk.PublicKey) {
					signer.keyType = RSAKeyType
					signer.logger.Debug("Loaded existing CA and private key")
					return
				}
			case ed25519.PrivateKey:
				if pub, ok := pub.(ed25519.PublicKey); ok && pub.Equal(pk.Public().(ed25519.PublicKey)) {
					signer.keyType = ED25519KeyType
					signer.logger.Debug("Loaded existing CA and private key")
					return
				}
			case *ecdsa.PrivateKey:
				if pub, ok := pub.(*ecdsa.PublicKey); ok && pub.Equal(&pk.PublicKey) {
					signer.keyType = ECDSAKeyType
					signer.logger.Debug("Loaded existing CA and private key")
					return
				}
			}
		}

		// Generate if it isn't valid
		err = signer.GenerateCA()
		if err != nil {
			signer.logger.Error("Failed to load and generate a certificate")
			return nil, err
		}

		// Save both certificate and PK at given pathes
		if err = signer.Save(certPath, keyPath); err != nil {
			return nil, err
		}

		return
	}

	// Generate a new private key
	if err = signer.GeneratePK(); err != nil {
		return nil, err
	}

	// Generate a new root certificate
	if err = signer.GenerateCA(); err != nil {
		return nil, err
	}

	// Save both certificate and PK at given pathes
	if err = signer.Save(certPath, keyPath); err != nil {
		return nil, err
	}

	signer.logger.Debug("Successfully generated and saved signer")
	return
}

// LoadCA loads a root certificate from the given path
func (signer *Signer) LoadCA(certPath string) (err error) {
	// Read the certificate file bytes
	certPEMBytes, err := os.ReadFile(certPath)
	if err != nil {
		err = fmt.Errorf("%w: %w", ErrCAFileReadFailed, err)
		signer.logger.Debug(err.Error())
		return
	}

	// Parse certificate PEM bytes into a PEM block
	certBlock, _ := pem.Decode(certPEMBytes)
	if certBlock == nil {
		err = ErrInvalidCertificatePEMBlock
		signer.logger.Error(err.Error())
		return
	}

	// Parse PEM block into a certificate object
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		err = fmt.Errorf("%w: %w", ErrCAParsingFailed, err)
		signer.logger.Error(err.Error())
		return
	}

	signer.cert = cert
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
			CommonName: DefaultCommonName,
		},
		NotBefore:             time.Now().Add(DefaultCABeforeTime),
		NotAfter:              time.Now().Add(DefaultCAExpirationTime),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}

	// Get a public key from the private key
	var pub any
	switch pk := signer.pk.(type) {
	case *rsa.PrivateKey:
		pub = &pk.PublicKey
	case ed25519.PrivateKey:
		pub = pk.Public().(ed25519.PublicKey)
	case *ecdsa.PrivateKey:
		pub = &pk.PublicKey
	}

	// Create a self-signed root certificate, returns bytes
	certBytes, err := x509.CreateCertificate(
		rand.Reader,
		certTemplate,
		certTemplate,
		pub,
		signer.pk,
	)
	if err != nil {
		err = fmt.Errorf("%w: %w", ErrRootCertificateCreationFailed, err)
		signer.logger.Error(err.Error())
		return err
	}

	// Parse certificate bytes into a certificate object
	cert, err := x509.ParseCertificate(certBytes)
	if err != nil {
		err = fmt.Errorf("%w: %w", ErrCAParsingFailed, err)
		signer.logger.Error(err.Error())
		return err
	}

	signer.cert = cert
	return nil
}

// LoadPK loads a private key from the given file path
func (signer *Signer) LoadPK(keyPath string) (err error) {
	// Reading bytes from the private key file
	pkPEMBytes, err := os.ReadFile(keyPath)
	if err != nil {
		err = fmt.Errorf("%w: %w", ErrPKFileReadFailed, err)
		signer.logger.Debug(err.Error())
		return
	}

	// Decode private key from PEM bytes to a PEM block
	pkBlock, _ := pem.Decode(pkPEMBytes)
	if pkBlock == nil {
		err = ErrInvalidPKPEMBlock
		signer.logger.Error(err.Error())
		return
	}

	// Parse private key object from the PEM block
	pk, err := x509.ParsePKCS8PrivateKey(pkBlock.Bytes)
	if err != nil {
		err = fmt.Errorf("%w: %w", ErrPKParsingFailed, err)
		signer.logger.Error(err.Error())
		return
	}
	signer.pk = pk

	signer.logger.Debug("Successfully loaded private key")
	return
}

// GeneratePK generates and assigns a new random private key of the given size
func (signer *Signer) GeneratePK() (err error) {
	// Generate random private key
	var pk crypto.PrivateKey
	switch signer.keyType {
	case RSAKeyType:
		pk, err = rsa.GenerateKey(rand.Reader, RSAKeySize)
	case ECDSAKeyType:
		pk, err = ecdsa.GenerateKey(ECDSAEllipticCurve, rand.Reader)
	case ED25519KeyType:
		_, pk, err = ed25519.GenerateKey(rand.Reader)
	}

	if err != nil {
		err = fmt.Errorf("%w: %w", ErrPKGenerationFailed, err)
		signer.logger.Error(err.Error())
		return
	}

	signer.pk = pk

	return nil
}

// Save writes certificate and private key to the given pathes
func (signer *Signer) Save(certPath, keyPath string) error {
	if signer.cert == nil {
		signer.logger.Error(ErrCertIsNil.Error())
		return ErrCertIsNil
	}
	if signer.pk == nil {
		signer.logger.Error(ErrPKIsNil.Error())
		return ErrPKIsNil
	}

	// Creating or truncating the certificate file
	certFile, err := os.Create(certPath)
	if err != nil {
		err = fmt.Errorf("%w: %w", ErrRootCertificateFileCreationFailed, err)
		signer.logger.Error(err.Error())
		return err
	}
	defer certFile.Close()

	// Creating PEM block object for certificate
	certBlock := pem.Block{
		Type:  "CERTIFICATE",
		Bytes: signer.cert.Raw,
	}

	// Writing encoded PEM certificate block to the certificate file
	err = pem.Encode(certFile, &certBlock)
	if err != nil {
		err = fmt.Errorf("%w: %w", ErrCertificatePEMEncodingFailed, err)
		signer.logger.Error(err.Error())
		return err
	}

	// Private key bytes
	pkBytes, err := x509.MarshalPKCS8PrivateKey(signer.pk)
	if err != nil {
		err = fmt.Errorf("%w: %w", ErrFailedPKMarshal, err)
		signer.logger.Error(err.Error())
		return err
	}

	// Creating or truncating the private key file
	keyFile, err := os.Create(keyPath)
	if err != nil {
		err = fmt.Errorf("%w: %w", ErrPKFileCreationFailed, err)
		signer.logger.Error(err.Error())
		return err
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
		err = fmt.Errorf("%w: %w", ErrPKPEMEncodingFailed, err)
		signer.logger.Error(err.Error())
		return err
	}

	return nil
}

// GenerateLeafCertificate generates a new leaf certificate for the given URL of the given size
func (signer *Signer) GenerateLeafCertificate(url u.URL) (*tls.Certificate, error) {
	hostname := url.Hostname()

	// Check if certificate is in cache and if it exists
	if signer.cache != nil {
		cert, ok := signer.cache.Get(hostname)
		if ok {
			signer.logger.Debug(fmt.Sprintf("Leaf certificate cache hit for %s", hostname))
			return cert, nil
		}
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
		NotBefore:   time.Now().Add(DefaultLeafBeforeTime),
		NotAfter:    time.Now().Add(DefaultLeafExpirationTime),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:    []string{hostname},
	}

	// Get public key of the signer's private key
	var pub any
	switch pk := signer.pk.(type) {
	case *rsa.PrivateKey:
		pub = &pk.PublicKey
	case ed25519.PrivateKey:
		pub = pk.Public().(ed25519.PublicKey)
	case *ecdsa.PrivateKey:
		pub = &pk.PublicKey
	}

	// Sign the leaf certificate and get its bytes
	certBytes, err := x509.CreateCertificate(
		rand.Reader,
		&cert,
		signer.cert,
		pub,
		signer.pk,
	)
	if err != nil {
		err = fmt.Errorf("%w: %w", ErrLeafCertificateGenerationFailed, err)
		signer.logger.Error(err.Error())
		return nil, err
	}

	// Create certificate object for TLS connection
	tlsCert := tls.Certificate{
		Certificate: [][]byte{certBytes, signer.cert.Raw},
		PrivateKey:  signer.pk,
	}

	// Add TLS certificate to cache if it exists
	if signer.cache != nil {
		signer.cache.Add(hostname, &tlsCert)
	}

	return &tlsCert, nil
}
