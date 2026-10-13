package proxy

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var (
	filterConfigBytes = []byte(`
storage_path: "./db"
groups:
  - filename: "a.db"
    whitelist:
      - "awhitelisted$"
      - "sharedwhitelisted$"
    blacklist:
      - "ablacklisted$"
      - "sharedblacklisted$"
  - filename: "b.db"
    whitelist:
      - "bwhitelisted$"
      - "sharedwhitelisted$"
    blacklist:
      - "bblacklisted$"
      - "sharedblacklisted$"
`)

	listenAddr = "localhost:0"
	keySize    = 2048
)

/* Utility Functions */

func createProxy(t *testing.T, opts ...Option) (*Proxy, error) {
	t.Helper()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "cache")
	filterPath := filepath.Join(dir, "filter.yml")
	certPath := filepath.Join(dir, "cert.txt")
	keyPath := filepath.Join(dir, "key.txt")

	if err := os.WriteFile(filterPath, filterConfigBytes, 0644); err != nil {
		return nil, err
	}

	return New(
		listenAddr,
		"",
		filterPath,
		dbPath,
		certPath,
		keyPath,
		keySize,
		opts...,
	)
}

/* Proxy Tests */

// Create a new Proxy
func TestProxyCreation(t *testing.T) {
	proxy, err := createProxy(t)
	if err != nil {
		t.Fatalf("Failed to create a proxy: %s", err)
	}
	defer proxy.Close()
}

// Create a new Proxy with a logger
func TestProxyCreationWithLogger(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	proxy, err := createProxy(t, WithLogger(logger))
	if err != nil {
		t.Fatalf("Failed to create a proxy: %s", err)
	}
	defer proxy.Close()

	if proxy.logger != logger {
		t.Fatalf("Attached to proxy logger isn't equal to the passed logger")
	}
}

// Create a new Proxy with a nil logger
func TestProxyCreationWithNilLogger(t *testing.T) {
	proxy, err := createProxy(t, WithLogger(nil))
	if err != nil {
		t.Fatalf("Failed to create a proxy: %s", err)
	}
	defer proxy.Close()

	if proxy.logger.Handler() != slog.DiscardHandler {
		t.Fatalf("Attached logger when nil logger was passed")
	}
}

// Create a new Proxy with positive signer cache size
func TestProxyCreationWithSignerCache(t *testing.T) {
	proxy, err := createProxy(t, WithSignerCache(1))
	if err != nil {
		t.Fatalf("Failed to create a proxy: %s", err)
	}
	defer proxy.Close()
}

// Create a new Proxy with negative signer cache size
func TestProxyCreationWithNegativeSignerCache(t *testing.T) {
	proxy, err := createProxy(t, WithSignerCache(-1))
	if err != nil {
		t.Fatalf("Failed to create a proxy: %s", err)
	}
	defer proxy.Close()
}

// Create a new Proxy with http.Client
func TestProxyCreationWithClient(t *testing.T) {
	client := &http.Client{}
	proxy, err := createProxy(t, WithClient(client))
	if err != nil {
		t.Fatalf("Failed to create a proxy: %s", err)
	}
	defer proxy.Close()

	if proxy.Client != client {
		t.Fatal("Proxies client isn't equal to passed")
	}
}

// Create a new Proxy with http.Transport
func TestProxyCreationWithTransport(t *testing.T) {
	transport := &http.Transport{}
	proxy, err := createProxy(t, WithTransport(transport))
	if err != nil {
		t.Fatalf("Failed to create a proxy: %s", err)
	}
	defer proxy.Close()

	if proxy.Client.Transport != transport {
		t.Fatal("Proxy's transport isn't equal to the passed")
	}
}

// Create a new Proxy with a nil http.Transport
func TestProxyCreationWithNilTransport(t *testing.T) {
	proxy, err := createProxy(t, WithTransport(nil))
	if err != nil {
		t.Fatalf("Failed to create a proxy: %s", err)
	}
	defer proxy.Close()

	if proxy.Client == nil {
		t.Fatal("Proxy's client is equal to nil")
	}

	if proxy.Client.Transport == nil {
		t.Fatal("Proxy client's transport is equal to nil")
	}
}

// Run a Proxy
func TestRunProxy(t *testing.T) {
	proxy, err := createProxy(t)
	if err != nil {
		t.Fatalf("Failed to create a proxy: %s", err)
	}
	defer proxy.Close()

	errChan := make(chan error, 1)
	go func() {
		temp := os.Stdout
		nullFile, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0666)
		if err == nil {
			os.Stdout = nullFile
		}
		errChan <- proxy.Run()
		os.Stdout = temp
	}()

	if err := proxy.Close(); err != nil {
		t.Fatalf("Failed to close a running proxy: %s", err)
	}

	if err := <-errChan; err != nil && !errors.Is(err, http.ErrServerClosed) {
		t.Fatalf("Failed to run a proxy: %s", err)
	}
}

// Run a Proxy with a nil Server
func TestRunProxyWithNilServer(t *testing.T) {
	if err := (&Proxy{}).Run(); err == nil {
		t.Fatal("Ran proxy with a nil Server successfully")
	}
}

// Close a Proxy with a nil Server
func TestCloseProxyWithNilServer(t *testing.T) {
	if err := (&Proxy{}).Close(); err != nil {
		t.Fatalf("Failed to close a nil Server: %s", err)
	}
}

// Handle HTTP request
func TestHTTPRequest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Page Content")
	}))

	proxy, err := createProxy(t, WithClient(ts.Client()))
	if err != nil {
		t.Fatalf("Failed to create a proxy: %s", err)
	}
	defer proxy.Close()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodGet,
		ts.URL+"/sharedwhitelisted",
		nil,
	)

	proxy.Server.Handler.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("Status of the received page isn't 200")
	}

	body, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(body), "Page Content") {
		t.Errorf("Received page has wrong content")
	}
}

// Handle HTTPS request
func TestHTTPSRequest(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Page Content")
	}))
	defer ts.Close()

	proxy, err := createProxy(t, WithClient(ts.Client()))
	if err != nil {
		t.Fatalf("Failed to create a proxy: %s", err)
	}
	defer proxy.Close()

	proxyServer := httptest.NewServer(proxy.Server.Handler)
	defer proxyServer.Close()

	proxyURL, _ := url.Parse(proxyServer.URL)

	transport := ts.Client().Transport.(*http.Transport).Clone()

	transport.Proxy = http.ProxyURL(proxyURL)

	transport.TLSClientConfig.InsecureSkipVerify = true

	client := &http.Client{Transport: transport}

	res, err := client.Get(ts.URL)
	if err != nil {
		t.Fatalf("Failed to execute request through proxy: %s", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", res.StatusCode)
	}

	body, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(body), "Page Content") {
		t.Errorf("Expected 'Page Content', got: %s", string(body))
	}
}

// Handle Blacklisted HTTPS request
func TestBlacklistedHTTPSRequest(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Page Content")
	}))
	defer ts.Close()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "cache")
	filterPath := filepath.Join(dir, "filter.yml")
	certPath := filepath.Join(dir, "cert.txt")
	keyPath := filepath.Join(dir, "key.txt")

	data := []byte(`
groups:
  - filename: "db.db"
    blacklist:
      - ".*"
`)
	if err := os.WriteFile(filterPath, data, 0644); err != nil {
		t.Fatalf("Failed to create a proxy: %s", err)
	}

	proxy, err := New(
		listenAddr,
		"",
		filterPath,
		dbPath,
		certPath,
		keyPath,
		keySize,
	)
	if err != nil {
		t.Fatalf("Failed to create a proxy: %s", err)
	}
	defer proxy.Close()

	proxyServer := httptest.NewServer(proxy.Server.Handler)
	defer proxyServer.Close()

	proxyURL, _ := url.Parse(proxyServer.URL)

	transport := ts.Client().Transport.(*http.Transport).Clone()

	transport.Proxy = http.ProxyURL(proxyURL)

	transport.TLSClientConfig.InsecureSkipVerify = true

	client := &http.Client{Transport: transport}

	res, err := client.Get(ts.URL)
	if err != nil {
		t.Fatalf("Failed to execute request through proxy: %s", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", res.StatusCode)
	}

	body, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(body), "Page Content") {
		t.Errorf("Expected 'Page Content', got: %s", string(body))
	}
}

// Return cached page on HTTP request
func TestHTTPRequestReturnCached(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Page Content")
	}))

	proxy, err := createProxy(t, WithClient(ts.Client()))
	if err != nil {
		t.Fatalf("Failed to create a proxy: %s", err)
	}
	defer proxy.Close()

	recOnline := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodGet,
		ts.URL+"/sharedwhitelisted",
		nil,
	)

	proxy.Server.Handler.ServeHTTP(recOnline, req)

	resOnline := recOnline.Result()
	defer resOnline.Body.Close()

	if resOnline.StatusCode != http.StatusOK {
		t.Errorf("Status of the received page isn't 200")
	}

	bodyOnline, _ := io.ReadAll(resOnline.Body)
	if !strings.Contains(string(bodyOnline), "Page Content") {
		t.Errorf("Received page has wrong content")
	}

	ts.Close()

	recOffline := httptest.NewRecorder()
	proxy.Server.Handler.ServeHTTP(recOffline, req)

	resOffline := recOffline.Result()
	bodyOffline, _ := io.ReadAll(resOffline.Body)
	defer resOffline.Body.Close()

	if !bytes.Equal(bodyOnline, bodyOffline) {
		t.Errorf("Cached page isn't equal to the one received online")
	}
}

// Handle Blacklisted HTTPS request with proxy
func TestBlacklistedHTTPSRequestWithProxy(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Page Content")
	}))
	defer ts.Close()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "cache")
	filterPath := filepath.Join(dir, "filter.yml")
	certPath := filepath.Join(dir, "cert.txt")
	keyPath := filepath.Join(dir, "key.txt")

	data := []byte(`
groups:
  - filename: "db.db"
    blacklist:
      - ".*"
`)
	if err := os.WriteFile(filterPath, data, 0644); err != nil {
		t.Fatalf("Failed to create a proxy: %s", err)
	}

	secondProxy, err := New(
		listenAddr,
		"",
		filterPath,
		dbPath,
		certPath,
		keyPath,
		keySize,
	)
	if err != nil {
		t.Fatalf("Failed to create a proxy: %s", err)
	}
	defer secondProxy.Close()

	dir = t.TempDir()
	dbPath = filepath.Join(dir, "cache")
	filterPath = filepath.Join(dir, "filter.yml")
	certPath = filepath.Join(dir, "cert.txt")
	keyPath = filepath.Join(dir, "key.txt")

	data = []byte(`
groups:
  - filename: "db.db"
    blacklist:
      - ".*"
`)
	if err := os.WriteFile(filterPath, data, 0644); err != nil {
		t.Fatalf("Failed to create a proxy: %s", err)
	}

	secondProxyServer := httptest.NewServer(secondProxy.Server.Handler)
	defer secondProxyServer.Close()
	secondProxyURL, _ := url.Parse(secondProxyServer.URL)

	firstProxy, err := New(
		listenAddr,
		secondProxyURL.String(),
		filterPath,
		dbPath,
		certPath,
		keyPath,
		keySize,
	)
	if err != nil {
		t.Fatalf("Failed to create a proxy: %s", err)
	}
	defer firstProxy.Close()

	proxyServer := httptest.NewServer(firstProxy.Server.Handler)
	defer proxyServer.Close()

	proxyURL, _ := url.Parse(proxyServer.URL)

	transport := ts.Client().Transport.(*http.Transport).Clone()

	transport.Proxy = http.ProxyURL(proxyURL)

	transport.TLSClientConfig.InsecureSkipVerify = true

	client := &http.Client{Transport: transport}

	res, err := client.Get(ts.URL)
	if err != nil {
		t.Fatalf("Failed to execute request through proxy: %s", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", res.StatusCode)
	}

	body, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(body), "Page Content") {
		t.Errorf("Expected 'Page Content', got: %s", string(body))
	}
}
