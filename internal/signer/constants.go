package signer

import (
	"crypto/elliptic"
	"errors"
	"time"
)

// Errors
var (
	ErrCertIsNil                         = errors.New("Certificate isn't initialized")
	ErrPKIsNil                           = errors.New("Private key isn't initialized")
	ErrPKFileReadFailed                  = errors.New("Failed to read bytes from the private key file")
	ErrCAFileReadFailed                  = errors.New("Failed to read bytes from the certificate file")
	ErrInvalidPKPEMBlock                 = errors.New("Loaded private key is invalid")
	ErrInvalidCertificatePEMBlock        = errors.New("Loaded certificate is invalid")
	ErrCAParsingFailed                   = errors.New("Failed to parse root certificate")
	ErrPKParsingFailed                   = errors.New("Failed to parse private key from PKCS#8")
	ErrRootCertificateCreationFailed     = errors.New("Failed to create root certificate")
	ErrFailedPKMarshal                   = errors.New("Failed to marshal PKCS#8 private key")
	ErrRootCertificateFileCreationFailed = errors.New("Failed to create root certificate file")
	ErrCertificatePEMEncodingFailed      = errors.New("Failed to write root certificate using PEM encoding to a file")
	ErrPKFileCreationFailed              = errors.New("Failed to open/create private key file")
	ErrPKPEMEncodingFailed               = errors.New("Failed to write private key using PEM encoding to a file")
	ErrPKGenerationFailed                = errors.New("Failed to generate private key")
	ErrCacheInitFailed                   = errors.New("Failed to initialize LRU cache")
	ErrLeafCertificateGenerationFailed   = errors.New("Failed to create leaf certificate")
)

// Key generation parameteres
var (
	ECDSAEllipticCurve = elliptic.P256() // Default key size for EDCSA key
	RSAKeySize         = 2048            // Default key size for RSA key
)

// Default parameters
var (
	DefaultCommonName         = "Caching Proxy Root CA"
	DefaultCABeforeTime       = -24 * time.Hour           // A day
	DefaultCAExpirationTime   = 24 * 365 * 10 * time.Hour // Roughly 10 years
	DefaultLeafBeforeTime     = -time.Hour
	DefaultLeafExpirationTime = 24 * 365 * time.Hour // Roughly a year
)

// KeyType enumeration of possible key types
type KeyType int

const (
	RSAKeyType KeyType = iota
	ECDSAKeyType
	ED25519KeyType
)

// String converts KeyType to a string
func (kt KeyType) String() string {
	switch kt {
	case RSAKeyType:
		return "rsa"
	case ECDSAKeyType:
		return "ecdsa"
	case ED25519KeyType:
		return "ed25519"
	default:
		return "unknown"
	}
}
