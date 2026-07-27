// Package proxy implements base structure for caching proxy
package proxy

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/LammoGit/Caching-Proxy/internal/cache"
	"github.com/LammoGit/Caching-Proxy/internal/filter"
	"github.com/LammoGit/Caching-Proxy/internal/signer"
)

/* Types */

// ProxySettings stores settings of a Proxy object
type ProxySettings struct {
	ListenAddr string
	WhitePath  string
	BlackPath  string
	DBPath     string
	CertPath   string
	KeyPath    string
	cacheSize  int
}

// Proxy represents caching proxy
type Proxy struct {
	Server   *http.Server
	Client   *http.Client
	Settings ProxySettings
	filter   *filter.Filter
	cache    *cache.Cache
	signer   *signer.Signer
	logger   *slog.Logger
}

/* Proxy Options */

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
// if transport is nil, then returned Option does nothing
func WithTransport(transport *http.Transport) Option {
	if transport == nil {
		return func(proxy *Proxy) {}
	} else {
		return WithClient(&http.Client{
			Timeout:   10 * time.Second,
			Transport: transport,
		})
	}
}

/* Proxy Methods */

// New returns a pointer to a new Proxy object
// listenAddr socket at which Proxy's server would be listening
// whitePath path to the file with whitelist filters
// blackPath path to the file with blalcklist filters
// dbPath path to the cache database file
// certPath path to the certificate file
// keyPath path to the private key file
// keySize size of a newly generated private key in bits
// opts functions that are ran as soon as Proxy object is initialized
func New(listenAddr, whitePath, blackPath, dbPath, certPath, keyPath string, keySize int, opts ...Option) (*Proxy, error) {
	var err error
	proxy := &Proxy{logger: slog.New(slog.DiscardHandler)}

	for _, opt := range opts {
		opt(proxy)
	}

	proxy.filter, err = filter.New(
		whitePath,
		blackPath,
		filter.WithLogger(proxy.logger),
	)
	if err != nil {
		return nil, err
	}

	proxy.signer, err = signer.New(
		certPath,
		keyPath,
		keySize,
		signer.WithLogger(proxy.logger),
		signer.WithCache(proxy.Settings.cacheSize),
	)
	if err != nil {
		return nil, err
	}

	proxy.cache, err = cache.New(
		dbPath,
		cache.WithLogger(proxy.logger),
	)
	if err != nil {
		return nil, err
	}

	if proxy.Client == nil {
		proxy.Client = &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout:   5 * time.Second,
					KeepAlive: 0,
				}).DialContext,
				TLSHandshakeTimeout: 5 * time.Second,
				DisableKeepAlives:   true,
				MaxIdleConns:        0,
				IdleConnTimeout:     0,
			},
		}
	}

	proxy.Server = &http.Server{
		Addr: listenAddr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.Method == http.MethodConnect {
				proxy.handleHTTPS(w, req)
			} else {
				proxy.handleHTTP(w, req)
			}
		}),
	}

	proxy.Settings = ProxySettings{
		ListenAddr: listenAddr,
		WhitePath:  whitePath,
		BlackPath:  blackPath,
		DBPath:     dbPath,
		CertPath:   certPath,
		KeyPath:    keyPath,
	}

	return proxy, nil
}

// Run prints out info about Proxy and starts listening
func (proxy *Proxy) Run() error {
	if proxy.Server == nil {
		return errors.New("tried to run the Proxy with nil Server")
	}

	fmt.Printf("Starting proxy on address: %s\n", proxy.Settings.ListenAddr)
	fmt.Printf("Cache filepath: %s\n", proxy.Settings.DBPath)
	fmt.Printf("Pathes to certificate and key files: %s %s\n", proxy.Settings.CertPath, proxy.Settings.KeyPath)

	fmt.Println("Whitelisted patterns:")
	for _, pat := range proxy.filter.WhitePatterns {
		fmt.Println(pat)
	}

	fmt.Println("Blacklisted patterns:")
	for _, pat := range proxy.filter.BlackPatterns {
		fmt.Println(pat)
	}

	defer proxy.cache.Close()
	if err := proxy.Server.ListenAndServe(); err != nil {
		return err
	}
	return nil
}

// Close closes running server and cache database connection
func (proxy *Proxy) Close() error {
	defer proxy.cache.Close()
	if proxy.Server != nil {
		return proxy.Server.Shutdown(context.Background())
	}
	return nil
}

// handleHTTP handles incoming HTTP requests
func (proxy *Proxy) handleHTTP(w http.ResponseWriter, req *http.Request) {
	matched := proxy.Match(req)
	proxy.logger.Debug("HTTP",
		slog.String("method", req.Method),
		slog.String("url", req.URL.String()),
	)
	proxy.forwardRequest(w, req, matched)
}

// handleHTTPS handles incoming HTTPS requests
func (proxy *Proxy) handleHTTPS(w http.ResponseWriter, req *http.Request) {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		proxy.logger.Error("Failed to hijack connection")
		return
	}
	defer clientConn.Close()

	if _, err := clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		proxy.logger.Error("Failed to write connection established")
		return
	}

	url := *req.URL
	cert, err := proxy.signer.GenerateLeafCertificate(url, 2048)
	if err != nil {
		proxy.logger.Error(fmt.Sprintf("Failed to generate certificate for: %s", url.String()))
		return
	}

	tlsConfig := &tls.Config{Certificates: []tls.Certificate{*cert}}
	tlsConn := tls.Server(clientConn, tlsConfig)
	defer tlsConn.Close()

	tlsReader := bufio.NewReader(tlsConn)
	inReq, err := http.ReadRequest(tlsReader)
	if err != nil {
		proxy.logger.Error(fmt.Sprintf("Failed to read HTTPS request: %s", url.String()))
		return
	}

	inReq.URL.Scheme = "https"
	inReq.URL.Host = inReq.Host
	inReq.RequestURI = ""

	matched := proxy.Match(inReq)
	proxy.logger.Debug(fmt.Sprintf("HTTPS %s %s", inReq.Method, inReq.URL))

	bufWriter := bufio.NewWriter(tlsConn)
	proxy.forwardRequest(bufWriter, inReq, matched)
	bufWriter.Flush()
}

// forwardRequest handles both HTTP and decrypted HTTPS requests
func (proxy *Proxy) forwardRequest(w io.Writer, req *http.Request, matched bool) {
	if req.URL.Scheme == "" {
		if req.TLS != nil {
			req.URL.Scheme = "https"
		} else {
			req.URL.Scheme = "http"
		}
	}
	if req.URL.Host == "" {
		req.URL.Host = req.Host
	}
	req.RequestURI = ""

	method := req.Method
	url := req.URL.String()

	resp, err := proxy.Client.Do(req)
	if err != nil {
		if matched {
			proxy.logger.Debug(fmt.Sprintf("Couldn't reach %s %s", method, url))
			if !proxy.LoadResponse(w, req, matched) {
				proxy.logger.Debug(fmt.Sprintf("Couldn't load response from cache for %s %s", method, url))
				fmt.Fprintf(w, "HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\n\r\n")
			}
		}
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		proxy.logger.Error(fmt.Sprintf("Failed to read response body for %s %s", method, url))
	}

	resp.Body = io.NopCloser(bytes.NewReader(body))
	if err := resp.Write(w); err != nil {
		proxy.logger.Error(fmt.Sprintf("Failed to write response to %s %s", method, url))
	}

	if matched {
		if err := proxy.SaveResponse(body, resp, req); err != nil {
			proxy.logger.Error(fmt.Sprintf("Failed to cache %s %s", method, url))
		} else {
			proxy.logger.Debug(fmt.Sprintf("Successfully cached %s %s", method, url))
		}
	}
}

// SaveResponse saves response to cache
func (proxy *Proxy) SaveResponse(body []byte, resp *http.Response, req *http.Request) error {
	headers, _ := json.Marshal(resp.Header)
	url := req.URL.String()
	method := req.Method

	page := cache.Page{
		Url:     url,
		Method:  method,
		Headers: headers,
		Content: body,
	}
	return proxy.cache.AddPage(page)
}

// LoadResponse returns response from cache given request
// if page isn't found or request doesn't match filter, then 404 is returned
func (proxy *Proxy) LoadResponse(w io.Writer, req *http.Request, matched bool) bool {
	var page cache.Page
	var err error

	if matched {
		page, err = proxy.cache.GetPage(req.URL.String(), req.Method)
	} else {
		err = fmt.Errorf("%s %s didn't match", req.URL, req.Method)
	}

	if err != nil {
		fmt.Fprintf(w, "HTTP/1.1 404 Not Found\r\nContent-Length: 0\r\n\r\n")
		return false
	}

	var headers http.Header
	json.Unmarshal([]byte(page.Headers), &headers)

	fmt.Fprintf(w, "HTTP/1.1 %d %s\r\n", http.StatusOK, http.StatusText(http.StatusOK))
	if err := headers.Write(w); err != nil {
		return false
	}

	fmt.Fprintf(w, "\r\n")
	if _, err := w.Write(page.Content); err != nil {
		return false
	}
	return true
}

// Match returns true if request matches the filter
// requests matches if its URL or Referer matches
func (proxy *Proxy) Match(req *http.Request) bool {
	// URL matches or Referer's URL matches
	res := proxy.filter.Match(req.URL.String()) ||
		proxy.filter.Match(req.Header.Get("Referer"))
	if res {
		proxy.logger.Debug(fmt.Sprintf("Matched: %s", req.URL.String()))
	} else {
		proxy.logger.Debug(fmt.Sprintf("Didn't match: %s", req.URL.String()))
	}
	return res
}
