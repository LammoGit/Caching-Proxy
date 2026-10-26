package proxy

import "errors"

// Errors
var (
	ErrIvalidProxyAddress     = errors.New("Invalid proxy address")
	ErrFailedReadCertificiate = errors.New("Failed to read a certificate")
	ErrNoCertificates         = errors.New("Certificate file contained no certificates")
	ErrRunNilServer           = errors.New("Tried to run the Proxy with nil Server")
)

// Magic variables
var (
	DefaultSavingBufferSize = 256
	DefaultSavingWorkers    = 4
)
