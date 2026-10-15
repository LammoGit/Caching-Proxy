package signer

import (
	"crypto/tls"
	"log/slog"

	lru "github.com/hashicorp/golang-lru/v2"
)

// Option represents functions that are used for initial configuration of a Signer object
type Option func(*Signer)

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
