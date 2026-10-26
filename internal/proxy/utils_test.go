package proxy

import (
	"os"
	"testing"
)

// Test proxy address normalization
func TestNormalizeProxyAddr(t *testing.T) {
	tests := []struct {
		label    string
		rawAddr  string
		wantAddr string
	}{
		{
			"Correct URL",
			"https://example.com",
			"https://example.com",
		},
		{
			"URL without protocol",
			"example.com",
			"https://example.com",
		},
		{
			"Invalid URL",
			"",
			"",
		},
	}

	for _, test := range tests {
		got, _ := normalizeProxyAddr(test.rawAddr)
		if got == nil && test.wantAddr != "" {
			t.Errorf("%s: got nil, expected %q\n", test.label, test.wantAddr)
		} else if got != nil && got.String() != test.wantAddr {
			t.Errorf("%s: got %q, expected %q\n", test.label, got.String(), test.wantAddr)
		}
	}
}

// Test parsing proxy URL from proxy environment variables
func TestProxyFromEnvMatch(t *testing.T) {
	tests := []struct {
		label                string
		blocked              bool
		lowerHTTPSProxyValue string
		upperHTTPSProxyValue string
		lowerHTTPProxyValue  string
		upperHTTPProxyValue  string
		target               string
		wantAddr             string
	}{
		{
			"Blocked",
			true,
			"https://example.lower",
			"https://example.upper",
			"http://example.lower",
			"http://example.upper",
			":80",
			"",
		},
		{
			"No port",
			false,
			"https://example.lower",
			"https://example.upper",
			"http://example.lower",
			"http://example.upper",
			"",
			"https://example.lower",
		},
		{
			"HTTPS",
			false,
			"https://example.lower",
			"https://example.upper",
			"http://example.lower",
			"http://example.upper",
			":443",
			"https://example.lower",
		},
		{
			"HTTPS only upper",
			false,
			"",
			"https://example.upper",
			"",
			"http://example.upper",
			":443",
			"https://example.upper",
		},
		{
			"HTTP",
			false,
			"https://example.lower",
			"https://example.upper",
			"http://example.lower",
			"http://example.upper",
			":80",
			"http://example.lower",
		},
		{
			"HTTP only upper",
			false,
			"",
			"https://example.upper",
			"",
			"http://example.upper",
			":80",
			"http://example.upper",
		},
	}

	// Store environment variables values
	lowerNoProxy := os.Getenv("no_proxy")
	lowerHTTPSProxyValue := os.Getenv("https_proxy")
	upperHTTPSProxyValue := os.Getenv("HTTPS_PROXY")
	lowerHTTPProxyValue := os.Getenv("http_proxy")
	upperHTTPProxyValue := os.Getenv("HTTP_PROXY")

	for _, test := range tests {
		if test.blocked {
			os.Setenv("no_proxy", "*")
		} else {
			os.Setenv("no_proxy", "")
		}

		os.Setenv("https_proxy", test.lowerHTTPSProxyValue)
		os.Setenv("HTTPS_PROXY", test.upperHTTPSProxyValue)
		os.Setenv("http_proxy", test.lowerHTTPProxyValue)
		os.Setenv("HTTP_PROXY", test.upperHTTPProxyValue)

		got := proxyFromEnv(test.target)
		if got == nil && test.wantAddr != "" {
			t.Errorf("%s: got nil, expected %q\n", test.label, test.wantAddr)
		} else if got != nil && got.String() != test.wantAddr {
			t.Errorf("%s: got %q, expected %q\n", test.label, got.String(), test.wantAddr)
		}
	}

	// Return environment variables values
	os.Setenv("no_proxy", lowerNoProxy)
	os.Setenv("https_proxy", lowerHTTPSProxyValue)
	os.Setenv("HTTPS_PROXY", upperHTTPSProxyValue)
	os.Setenv("http_proxy", lowerHTTPProxyValue)
	os.Setenv("HTTP_PROXY", upperHTTPProxyValue)
}

// Test blocking proxy URL via environment variables
func TestNoProxyMatch(t *testing.T) {
	tests := []struct {
		label             string
		lowerNoProxyValue string
		upperNoProxyValue string
		target            string
		wantMatch         bool
	}{
		{
			"Both empty",
			"",
			"",
			"",
			false,
		},
		{
			"Only lower, doesn't match",
			"example.com",
			"",
			"doesn't match",
			false,
		},
		{
			"Only lower asteriks, match",
			"*",
			"",
			"anything",
			true,
		},
		{
			"Only lower domain, match domain",
			"example.com",
			"",
			"example.com",
			true,
		},
		{
			"Only lower domain, match subdomain",
			"example.com",
			"",
			"sub.example.com",
			true,
		},
		{
			"Only lower subdomain, match domain",
			".example.com",
			"",
			"example.com",
			false,
		},
		{
			"Only lower subdomain, match subdomain",
			".example.com",
			"",
			"sub.example.com",
			true,
		},
	}

	// Store environment variables values
	lowerNoProxy := os.Getenv("no_proxy")
	upperNoProxy := os.Getenv("NO_PROXY")

	for _, test := range tests {
		os.Setenv("no_proxy", test.lowerNoProxyValue)
		os.Setenv("NO_PROXY", test.upperNoProxyValue)

		got := noProxyMatch(test.target)
		if got != test.wantMatch {
			t.Errorf("%s: got %v, expected %v\n", test.label, got, test.wantMatch)
		}
	}

	// Return environment variables values
	os.Setenv("no_proxy", lowerNoProxy)
	os.Setenv("NO_PROXY", upperNoProxy)
}
