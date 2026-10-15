package signer

import (
	"crypto/elliptic"
	"errors"
)

// Errors
var (
	ErrCertIsNil                         = errors.New("certificate isn't initialized")
	ErrPKIsNil                           = errors.New("private key isn't initialized")
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
	ErrPKGenerationFailed                = errors.New("failed to generate private key")
	ErrCacheInitFailed                   = errors.New("failed to initialize LRU cache")
	ErrLeafCertificateGenerationFailed   = errors.New("failed to create leaf certificate")
)

// Key generation parameteres
var (
	ECDSAEllipticCurve = elliptic.P256()
	RSAKeySize         = 2048
)

// KeyType enumeration of possible key types
type KeyType uint

const (
	RSAKeyType KeyType = iota
	ECDSAKeyType
	ED25519KeyType
)
