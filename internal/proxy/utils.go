package proxy

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
)

// normalizeProxyAddr converts string URL to the *url.URL
func normalizeProxyAddr(raw string) (*url.URL, error) {
	if raw == "" {
		return nil, nil
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if u.Host == "" {
		return nil, fmt.Errorf("proxy address %s has no host", raw)
	}
	return u, nil
}

// proxyFromEnv resolves an upstream proxy URL for a raw
// TCP target from the standard environment variables
func proxyFromEnv(targetHost string) *url.URL {
	if noProxyMatch(targetHost) {
		return nil
	}

	_, port, err := net.SplitHostPort(targetHost)
	if err != nil {
		port = "443"
	}

	var keys []string
	if port == "443" {
		keys = []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy"}
	} else {
		keys = []string{"HTTP_PROXY", "http_proxy"}
	}

	for _, k := range keys {
		v := os.Getenv(k)
		if v == "" {
			continue
		}
		u, err := url.Parse(v)
		if err == nil && u.Host != "" {
			return u
		}
	}
	return nil
}

// noProxyMatch returns true if target matches
// one of domain wildcrads in NO_PROXY environment variable
func noProxyMatch(target string) bool {
	npx := os.Getenv("NO_PROXY")
	if npx == "" {
		npx = os.Getenv("no_proxy")
	}
	if npx == "" {
		return false
	}
	host, _, _ := net.SplitHostPort(target)
	if host == "" {
		host = target
	}
	for _, entry := range strings.Split(npx, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "*" || entry == host ||
			strings.HasSuffix(host, "."+strings.TrimPrefix(entry, ".")) {
			return true
		}
	}
	return false
}
