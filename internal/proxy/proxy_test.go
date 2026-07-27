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
	whiteFileContent = []byte("whitelist\ngraylist")
	blackFileContent = []byte("blacklist\ngraylist")

	listenAddr = "localhost:0"
	keySize    = 2048
)

/* Utility Functions */

func createProxy(t *testing.T, opts ...Option) (*Proxy, error) {
	t.Helper()

	dir := t.TempDir()
	whitePath := filepath.Join(dir, "whitelist.txt")
	blackPath := filepath.Join(dir, "blacklist.txt")
	dbPath := filepath.Join(dir, "db.txt")
	certPath := filepath.Join(dir, "cert.txt")
	keyPath := filepath.Join(dir, "key.txt")

	if err := os.WriteFile(whitePath, whiteFileContent, 0644); err != nil {
		return nil, err
	}

	if err := os.WriteFile(blackPath, blackFileContent, 0644); err != nil {
		return nil, err
	}

	return New(
		listenAddr,
		whitePath,
		blackPath,
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
	defer proxy.cache.Close()
}

// Create a new Proxy with a logger
func TestProxyCreationWithLogger(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	proxy, err := createProxy(t, WithLogger(logger))
	if err != nil {
		t.Fatalf("Failed to create a proxy: %s", err)
	}
	defer proxy.cache.Close()

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
	defer proxy.cache.Close()

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
	defer proxy.cache.Close()
}

// Create a new Proxy with negative signer cache size
func TestProxyCreationWithNegativeSignerCache(t *testing.T) {
	proxy, err := createProxy(t, WithSignerCache(-1))
	if err != nil {
		t.Fatalf("Failed to create a proxy: %s", err)
	}
	defer proxy.cache.Close()
}

// Create a new Proxy with http.Client
func TestProxyCreationWithClient(t *testing.T) {
	client := &http.Client{}
	proxy, err := createProxy(t, WithClient(client))
	if err != nil {
		t.Fatalf("Failed to create a proxy: %s", err)
	}
	defer proxy.cache.Close()

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
	defer proxy.cache.Close()

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
	defer proxy.cache.Close()

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
	defer proxy.cache.Close()

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

// Save response
func TestSaveResponse(t *testing.T) {
	proxy, err := createProxy(t)
	if err != nil {
		t.Fatalf("Failed to create a proxy: %s", err)
	}
	defer proxy.cache.Close()

	body := []byte{}
	resp := &http.Response{}
	req := httptest.NewRequest(
		http.MethodGet,
		"https://example.com",
		nil,
	)

	if err := proxy.SaveResponse(body, resp, req); err != nil {
		t.Fatalf("Failed to save a response: %s", err)
	}
}

// Update response
func TestUpdateResponse(t *testing.T) {
	proxy, err := createProxy(t)
	if err != nil {
		t.Fatalf("Failed to create a proxy: %s", err)
	}
	defer proxy.cache.Close()

	req := httptest.NewRequest(
		http.MethodGet,
		"https://example.com",
		nil,
	)

	wOld := httptest.NewRecorder()
	wOld.WriteHeader(http.StatusOK)
	_, _ = wOld.WriteString("Response Old")

	respOld := wOld.Result()
	defer respOld.Body.Close()

	bodyOld, err := io.ReadAll(respOld.Body)
	if err != nil {
		t.Fatalf("Failed to read old response body: %s", err)
	}

	err = proxy.SaveResponse(
		bodyOld,
		respOld,
		req,
	)
	if err != nil {
		t.Fatalf("Failed to save a response: %s", err)
	}

	wNew := httptest.NewRecorder()
	wNew.WriteHeader(http.StatusOK)
	_, _ = wNew.WriteString("Response New")

	respNew := wNew.Result()
	defer respNew.Body.Close()

	bodyNew, err := io.ReadAll(respNew.Body)
	if err != nil {
		t.Fatalf("Failed to read new response body: %s", err)
	}

	err = proxy.SaveResponse(
		bodyNew,
		respNew,
		req,
	)

	if err != nil {
		t.Fatalf("Failed to update a response: %s", err)
	}
}

// Load response
func TestLoadResponse(t *testing.T) {
	proxy, err := createProxy(t)
	if err != nil {
		t.Fatalf("Failed to create a proxy: %s", err)
	}
	defer proxy.cache.Close()

	body := []byte("Response body")
	resp := &http.Response{}
	req := httptest.NewRequest(
		http.MethodGet,
		"https://example.com",
		nil,
	)

	if err := proxy.SaveResponse(body, resp, req); err != nil {
		t.Fatalf("Failed to save a response: %s", err)
	}

	var b strings.Builder
	if ok := proxy.LoadResponse(&b, req, true); !ok {
		t.Fatalf("Didn't load saved response")
	}

	if b.String() != "HTTP/1.1 200 OK\r\n\r\nResponse body" {
		t.Fatal("Loaded wrong page from proxy's cache")
	}
}

// Load unmatched response
func TestLoadUnmatchedResponse(t *testing.T) {
	proxy, err := createProxy(t)
	if err != nil {
		t.Fatalf("Failed to create a proxy: %s", err)
	}
	defer proxy.cache.Close()

	body := []byte("Response body")
	resp := &http.Response{}
	req := httptest.NewRequest(
		http.MethodGet,
		"https://example.com",
		nil,
	)

	if err := proxy.SaveResponse(body, resp, req); err != nil {
		t.Fatalf("Failed to save a response: %s", err)
	}

	var b strings.Builder
	if ok := proxy.LoadResponse(&b, req, false); ok {
		t.Fatalf("Loaded unmatched response")
	}

	if b.String() != "HTTP/1.1 404 Not Found\r\nContent-Length: 0\r\n\r\n" {
		t.Fatal("Returned response isn't empty 404")
	}
}

// Load updated response
func TestLoadUpdatedResponse(t *testing.T) {}

// Handle HTTP request
func TestHTTPRequest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Page Content")
	}))

	proxy, err := createProxy(t, WithClient(ts.Client()))
	if err != nil {
		t.Fatalf("Failed to create a proxy: %s", err)
	}
	defer proxy.cache.Close()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodGet,
		ts.URL+"/whitelist",
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
	defer proxy.cache.Close()

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
	defer proxy.cache.Close()

	recOnline := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodGet,
		ts.URL+"/whitelist",
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
	defer resOffline.Body.Close()

	bodyOffline, _ := io.ReadAll(resOffline.Body)
	if !bytes.Equal(bodyOnline, bodyOffline) {
		t.Errorf("Cached page isn't equal to the one received online")
	}
}

// Match request with whitelisted URL
func TestMatchURL(t *testing.T) {
	proxy, err := createProxy(t)
	if err != nil {
		t.Fatalf("Failed to create a proxy: %s", err)
	}
	defer proxy.cache.Close()

	req := httptest.NewRequest(http.MethodGet, "https://whitelist.com", nil)
	req.Header.Set("Referer", "blacklist")

	if !proxy.Match(req) {
		t.Fatal("Proxy didn't match request with a whitelisted URL")
	}
}

// Match request with whitelisted Referer
func TestMatchReferer(t *testing.T) {
	proxy, err := createProxy(t)
	if err != nil {
		t.Fatalf("Failed to create a proxy: %s", err)
	}
	defer proxy.cache.Close()

	req := httptest.NewRequest(http.MethodGet, "https://blacklist.com", nil)
	req.Header.Set("Referer", "whitelist")

	if !proxy.Match(req) {
		t.Fatal("Proxy didn't match request with a whitelisted Referer")
	}
}

// Match request with both URL and Referer blacklisted
func TestMatchBlacklisted(t *testing.T) {
	proxy, err := createProxy(t)
	if err != nil {
		t.Fatalf("Failed to create a proxy: %s", err)
	}
	defer proxy.cache.Close()

	req := httptest.NewRequest(http.MethodGet, "https://blacklist.com", nil)
	req.Header.Set("Referer", "blacklist")

	if proxy.Match(req) {
		t.Fatal("Proxy matched request with both URL and Referer blacklisted")
	}
}
