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
	"path/filepath"
	"strings"
	"time"

	"github.com/LammoGit/Caching-Proxy/internal/cache"
	"github.com/LammoGit/Caching-Proxy/internal/filter"
	"github.com/LammoGit/Caching-Proxy/internal/signer"
)

/* Types */

// ProxySettings stores settings of a Proxy object
type ProxySettings struct {
	ListenAddr string
	FilterPath string
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
func New(listenAddr, filterPath, dbPath, certPath, keyPath string, keySize int, opts ...Option) (*Proxy, error) {
	var err error
	proxy := &Proxy{logger: slog.New(slog.DiscardHandler)}

	for _, opt := range opts {
		opt(proxy)
	}

	proxy.filter, err = filter.New(
		filterPath,
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

	groupPathes := make(map[string]string)
	for k, v := range proxy.filter.Groups {
		groupPathes[k] = filepath.Join(dbPath, v.FileName)
	}

	proxy.cache, err = cache.New(
		groupPathes,
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
		FilterPath: filterPath,
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
	fmt.Printf("Pathes to certificate and key files: %s %s\n", proxy.Settings.CertPath, proxy.Settings.KeyPath)

	defer proxy.cache.Close()
	if err := proxy.Server.ListenAndServe(); err != nil {
		return err
	}
	return nil
}

// Close closes running server and cache database connection
func (proxy *Proxy) Close() error {
	if proxy.cache != nil {
		defer proxy.cache.Close()
	}
	if proxy.Server != nil {
		return proxy.Server.Shutdown(context.Background())
	}
	return nil
}

// handleHTTP handles incoming HTTP requests
func (proxy *Proxy) handleHTTP(w http.ResponseWriter, req *http.Request) {
	matchedURL := proxy.filter.Match(req.URL.String())
	matchedReferer := proxy.filter.Match(req.Header.Get("Referer"))
	matched := make(map[string]struct{})
	for group, match := range matchedURL {
		if match == filter.MatchWhiteList {
			matched[group] = struct{}{}
		}
		if refMatch, ok := matchedReferer[group]; ok && refMatch == filter.MatchWhiteList {
			matched[group] = struct{}{}
		}
	}

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

	url := req.URL
	matchedURL := proxy.filter.Match(url.String())
	if len(matchedURL) == 0 {
		host, port, err := net.SplitHostPort(req.Host)
		if err != nil {
			host = req.Host
			port = "443"
		}

		host = strings.TrimPrefix(host, "//")
		targetAddr := net.JoinHostPort(host, port)

		targetConn, err := net.DialTimeout("tcp", targetAddr, 5*time.Second)

		if err != nil {
			proxy.logger.Error(fmt.Sprintf("Failed to connect to %s: %s", url.String(), err))
			clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
			return
		}
		defer targetConn.Close()

		if _, err := clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
			proxy.logger.Error(fmt.Sprintf("Failed to connect to %s: %s", url.String(), err))
			return
		}

		errChan := make(chan error, 2)
		go func() {
			_, err := io.Copy(targetConn, clientConn)
			errChan <- err
		}()

		go func() {
			_, err := io.Copy(clientConn, targetConn)
			errChan <- err
		}()

		<-errChan
		return
	}

	if _, err := clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		proxy.logger.Error("Failed to write connection established")
		return
	}

	cert, err := proxy.signer.GenerateLeafCertificate(*url, 2048)
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

	matchedURL = proxy.filter.Match(inReq.URL.String())
	matchedReferer := proxy.filter.Match(inReq.Header.Get("Referer"))
	matched := make(map[string]struct{})
	for group, match := range matchedURL {
		if match == filter.MatchWhiteList {
			matched[group] = struct{}{}
		}
		if refMatch, ok := matchedReferer[group]; ok && refMatch == filter.MatchWhiteList {
			matched[group] = struct{}{}
		}
	}
	proxy.logger.Debug(fmt.Sprintf("HTTPS %s %s", inReq.Method, inReq.URL))

	bufWriter := bufio.NewWriter(tlsConn)
	proxy.forwardRequest(bufWriter, inReq, matched)
	bufWriter.Flush()
}

// forwardRequest handles both HTTP and decrypted HTTPS requests
func (proxy *Proxy) forwardRequest(w io.Writer, req *http.Request, matched map[string]struct{}) {
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
		if len(matched) > 0 {
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

	if len(matched) > 0 {
		if err := proxy.SaveResponse(body, resp, req, matched); err != nil {
			proxy.logger.Error(fmt.Sprintf("Failed to cache %s %s", method, url))
		} else {
			proxy.logger.Debug(fmt.Sprintf("Successfully cached %s %s", method, url))
		}
	}
}

// SaveResponse saves response to cache
func (proxy *Proxy) SaveResponse(body []byte, resp *http.Response, req *http.Request, matched map[string]struct{}) error {
	headers, _ := json.Marshal(resp.Header)
	url := req.URL.String()
	method := req.Method

	page := cache.Page{
		Url:     url,
		Method:  method,
		Headers: headers,
		Content: body,
	}
	return proxy.cache.AddPage(matched, page)
}

// LoadResponse returns response from cache given request
// if page isn't found or request doesn't match filter, then 404 is returned
func (proxy *Proxy) LoadResponse(w io.Writer, req *http.Request, matched map[string]struct{}) bool {
	var page cache.Page
	var err error

	if len(matched) > 0 {
		page, err = proxy.cache.GetPage(matched, req.URL.String(), req.Method)
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
