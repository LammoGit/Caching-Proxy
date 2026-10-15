package proxy

import (
	"log/slog"
	"net/http"
)

// Option functions that are ran as Proxy object is initialized
type Option func(*Proxy)

// WithLogger Option sets the given logger pointer as a logger for the Proxy
// if the given logger pointer is nil, then Proxy's logger is set to discard log messages
func WithLogger(logger *slog.Logger) Option {
	return func(proxy *Proxy) {
		if logger == nil {
			proxy.logger = slog.New(slog.DiscardHandler)
		} else {
			proxy.logger = logger
		}
	}
}

// WithSignerCache Option sets size of the Signer's LRU cache
func WithSignerCache(cacheSize int) Option {
	return func(proxy *Proxy) {
		proxy.Settings.cacheSize = max(0, cacheSize)
	}
}

// WithClient Option sets the given client pointer as a client for the Proxy
// if the given client pointer is nil, then it's replaced by the default client
func WithClient(client *http.Client) Option {
	return func(proxy *Proxy) {
		proxy.Client = client
	}
}

// WithTransport Option sets the given transport pointer as a transport for the Proxy's client
// If transport is nil, the returned Option does nothing
func WithTransport(transport *http.Transport) Option {
	if transport == nil {
		return func(proxy *Proxy) {}
	}
	return func(proxy *Proxy) {
		proxy.Client = &http.Client{Transport: transport}
	}
}
