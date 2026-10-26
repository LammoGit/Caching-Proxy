// Package proxy implements base structure for caching proxy
package proxy

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/LammoGit/Caching-Proxy/internal/cache"
	"github.com/LammoGit/Caching-Proxy/internal/filter"
	"github.com/LammoGit/Caching-Proxy/internal/signer"
)

// ProxySettings stores settings of a Proxy object
type ProxySettings struct {
	ProxyURL   *url.URL       // URL of a proxy to send requests through
	ListenAddr string         // Address to listen from
	FilterPath string         // Path to a filter config file
	CertPath   string         // Path to a certificate file
	KeyPath    string         // Path to a private key file
	cacheSize  int            // Size of a cache
	certPool   *x509.CertPool // Pool of verified certificates
}

// saveJob is a queued cache write produced by forwardRequest
type saveJob struct {
	matched map[string]struct{} // Set of matched groups
	page    cache.Page          // Page to save
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

	saveCh    chan saveJob   // Channel for saving pages jobs
	saveWg    sync.WaitGroup // Saving waiting group
	closeOnce sync.Once      // Function that closes server once
}

// New returns a pointer to a new Proxy object
func New(listenAddr, proxyAddr, filterPath, dbPath, certPath, keyTypeString, keyPath string, opts ...Option) (*Proxy, error) {
	var err error
	proxy := &Proxy{logger: slog.New(slog.DiscardHandler)}

	proxyURL, err := normalizeProxyAddr(proxyAddr)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrIvalidProxyAddress, err)
	}
	proxy.Settings = ProxySettings{
		ProxyURL:   proxyURL,
		ListenAddr: listenAddr,
		FilterPath: filterPath,
		CertPath:   certPath,
		KeyPath:    keyPath,
	}

	// Take system certificates or create a new empty pool
	pool, err := x509.SystemCertPool()
	if err != nil {
		pool = x509.NewCertPool()
	}

	// Add proxy certificate to the verified certificate pool
	innerCA, err := os.ReadFile(certPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %w", ErrFailedReadCertificiate, err)
		}
	} else if !pool.AppendCertsFromPEM(innerCA) {
		return nil, ErrNoCertificates
	}
	proxy.Settings.certPool = pool

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

	// Get key type based on the passed string
	var keyType signer.KeyType
	switch strings.ToLower(keyTypeString) {
	case "rsa":
		keyType = signer.RSAKeyType
	case "ecdsa":
		keyType = signer.ECDSAKeyType
	case "ed25519":
		keyType = signer.ED25519KeyType
	}

	proxy.signer, err = signer.New(
		certPath,
		keyPath,
		keyType,
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
		cache.WithMetaCache(1024),
		cache.WithLogger(proxy.logger),
	)
	if err != nil {
		return nil, err
	}

	if proxy.Client == nil {
		transport := &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSClientConfig: &tls.Config{
				RootCAs: proxy.Settings.certPool,
			},
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: 15 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   16,
			IdleConnTimeout:       90 * time.Second,
		}
		if proxy.Settings.ProxyURL != nil {
			transport.Proxy = http.ProxyURL(proxy.Settings.ProxyURL)
		}
		proxy.Client = &http.Client{
			Transport: transport,
		}
	}

	// Create a saving channel with a worker group and start workers
	proxy.saveCh = make(chan saveJob, DefaultSavingBufferSize)
	for i := 0; i < DefaultSavingWorkers; i++ {
		proxy.saveWg.Add(1)
		go proxy.saveWorker()
	}

	proxy.Server = &http.Server{
		Addr:              listenAddr,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.Method == http.MethodConnect {
				proxy.handleHTTPS(w, req)
			} else {
				proxy.handleHTTP(w, req)
			}
		}),
	}

	return proxy, nil
}

// Run prints out info about Proxy and starts listening
func (proxy *Proxy) Run() error {
	if proxy.Server == nil {
		return ErrRunNilServer
	}

	fmt.Printf("Starting proxy on address: %s\n", proxy.Settings.ListenAddr)
	fmt.Printf("Pathes to certificate and key files: %s %s\n", proxy.Settings.CertPath, proxy.Settings.KeyPath)

	if err := proxy.Server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Close closes running server and cache database connection
func (proxy *Proxy) Close() error {
	var err error
	proxy.closeOnce.Do(func() {
		if proxy.Server != nil {
			err = proxy.Server.Shutdown(context.Background())
		}

		if proxy.saveCh != nil {
			close(proxy.saveCh)
			proxy.saveWg.Wait()
		}

		if proxy.cache != nil {
			proxy.cache.Close()
		}
	})
	return err
}

// saveWorker drains proxy.saveCh and writes pages to the cache
func (proxy *Proxy) saveWorker() {
	defer proxy.saveWg.Done()
	for job := range proxy.saveCh {
		if err := proxy.cache.AddPage(job.matched, job.page); err != nil {
			proxy.logger.Debug("failed to cache page",
				slog.String("method", job.page.Method),
				slog.String("url", job.page.URL),
				slog.String("error", err.Error()))
		} else {
			proxy.logger.Debug("cached page",
				slog.String("method", job.page.Method),
				slog.String("url", job.page.URL))
		}
	}
}

// enqueueSave queues a page for asynchronous write
// Non-blocking: drops the job if the queue is full
func (proxy *Proxy) enqueueSave(matched map[string]struct{}, page cache.Page) {
	select {
	case proxy.saveCh <- saveJob{matched: matched, page: page}:
	default:
		proxy.logger.Debug("save queue full, dropping",
			slog.String("method", page.Method),
			slog.String("url", page.URL))
	}
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

	// Do not cache Range responses
	if req.Header.Get("Range") != "" {
		matched = nil
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

	// If all blocked, then don't decrypt
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

		upstream := proxy.Settings.ProxyURL
		if upstream == nil {
			upstream = proxyFromEnv(targetAddr)
		}
		targetConn, err := proxy.dial(upstream, targetAddr)
		if err != nil {
			proxy.logger.Error("failed to connect upstream",
				slog.String("target", targetAddr),
				slog.String("error", err.Error()))
			fmt.Fprintf(clientConn, "HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\n\r\n")
			return
		}
		defer targetConn.Close()

		if _, err := clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
			proxy.logger.Error("failed to write connection established",
				slog.String("error", err.Error()))
			return
		}

		errChan := make(chan error, 2)
		go func() {
			_, err := io.Copy(targetConn, clientConn)
			targetConn.Close()
			errChan <- err
		}()
		go func() {
			_, err := io.Copy(clientConn, targetConn)
			clientConn.Close()
			errChan <- err
		}()
		<-errChan
		return
	}

	// Send that connection is established
	if _, err := clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
		proxy.logger.Error("Failed to write connection established")
		return
	}

	// Generate leaf certificate for the requested domain
	cert, err := proxy.signer.GenerateLeafCertificate(*url)
	if err != nil {
		proxy.logger.Error("failed to generate certificate",
			slog.String("host", url.String()),
			slog.String("error", err.Error()))
		return
	}

	// Create a TLS server with the generated leaf certificate and TCP connection
	tlsConfig := &tls.Config{Certificates: []tls.Certificate{*cert}}
	tlsConn := tls.Server(clientConn, tlsConfig)
	defer tlsConn.Close()

	tlsReader := bufio.NewReaderSize(tlsConn, 64*1024)
	bufWriter := bufio.NewWriterSize(tlsConn, 64*1024)

	for {
		inReq, err := http.ReadRequest(tlsReader)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				proxy.logger.Debug("read request failed",
					slog.String("error", err.Error()))
			}
			return
		}
		inReq.URL.Scheme = "https"
		inReq.URL.Host = inReq.Host
		inReq.RequestURI = ""

		// Get set of matching groups
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

		// Do not cache Range responses
		if inReq.Header.Get("Range") != "" {
			matched = nil
		}

		proxy.logger.Debug("HTTPS",
			slog.String("method", inReq.Method),
			slog.String("url", inReq.URL.String()))

		proxy.forwardRequest(bufWriter, inReq, matched)
		if err := bufWriter.Flush(); err != nil {
			return
		}
		if inReq.Close {
			return
		}
	}
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

	ctx, cancel := context.WithTimeout(req.Context(), 60*time.Second)
	defer cancel()

	resp, err := proxy.Client.Do(req.WithContext(ctx))
	if err != nil {
		proxy.logger.Debug("couldn't reach origin",
			slog.String("method", method),
			slog.String("url", url),
			slog.String("error", err.Error()))
		proxy.LoadResponse(w, req, matched)
		return
	}
	defer resp.Body.Close()

	page := cache.Page{
		URL:          url,
		Method:       method,
		StatusCode:   resp.StatusCode,
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
	}
	if headers, err := json.Marshal(resp.Header); err == nil {
		page.Headers = headers
	}

	var body bytes.Buffer
	resp.Body = io.NopCloser(io.TeeReader(resp.Body, &body))

	if err := resp.Write(w); err != nil {
		proxy.logger.Error("failed to write response",
			slog.String("method", method),
			slog.String("url", url),
			slog.String("error", err.Error()))
		return
	}

	if len(matched) > 0 && body.Len() > 0 {
		page.Content = body.Bytes()
		proxy.enqueueSave(matched, page)
	}
}

// LoadResponse returns response from cache given request on a miss, returns 504
func (proxy *Proxy) LoadResponse(w io.Writer, req *http.Request, matched map[string]struct{}) bool {
	if len(matched) == 0 {
		fmt.Fprintf(w, "HTTP/1.1 504 Gateway Timeout\r\nContent-Length: 0\r\n\r\n")
		return false
	}

	page, err := proxy.cache.GetPage(matched, req)
	if err != nil {
		fmt.Fprintf(w, "HTTP/1.1 504 Gateway Timeout\r\nContent-Length: 0\r\n\r\n")
		return false
	}

	var headers http.Header
	if err := json.Unmarshal(page.Headers, &headers); err != nil {
		proxy.logger.Debug("failed to unmarshal cached headers",
			slog.String("url", page.URL),
			slog.String("error", err.Error()))
		fmt.Fprintf(w, "HTTP/1.1 504 Gateway Timeout\r\nContent-Length: 0\r\n\r\n")
		return false
	}

	statusText := http.StatusText(page.StatusCode)
	if statusText == "" {
		statusText = "Unknown"
	}

	if _, err := fmt.Fprintf(w, "HTTP/1.1 %d %s\r\n", page.StatusCode, statusText); err != nil {
		return false
	}
	if err := headers.Write(w); err != nil {
		return false
	}
	if _, err := fmt.Fprintf(w, "\r\n"); err != nil {
		return false
	}
	if _, err := w.Write(page.Content); err != nil {
		return false
	}
	return true
}

// dial opens a TCP connection to targetAddr, optionally through an HTTP proxy
// When proxyURL is nil, dials directly
func (proxy *Proxy) dial(proxyURL *url.URL, targetAddr string) (net.Conn, error) {
	if proxyURL == nil {
		return net.DialTimeout("tcp", targetAddr, 5*time.Second)
	}

	conn, err := net.DialTimeout("tcp", proxyURL.Host, 5*time.Second)
	if err != nil {
		return nil, err
	}

	connectReq := &http.Request{
		Method: http.MethodConnect,
		URL:    &url.URL{Opaque: targetAddr},
		Host:   targetAddr,
		Header: make(http.Header),
	}

	if proxyURL.User != nil {
		pw, _ := proxyURL.User.Password()
		auth := base64.StdEncoding.EncodeToString(
			[]byte(proxyURL.User.Username() + ":" + pw))
		connectReq.Header.Set("Proxy-Authorization", "Basic "+auth)
	}

	if err := connectReq.Write(conn); err != nil {
		conn.Close()
		return nil, err
	}

	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, connectReq)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		conn.Close()
		return nil, fmt.Errorf("upstream CONNECT failed: %s", resp.Status)
	}
	return &bufferedConn{Conn: conn, r: br}, nil
}

// bufferedConn wraps a net.Conn so that reads drain the bufio.Reader first
type bufferedConn struct {
	net.Conn
	r *bufio.Reader
}

func (c *bufferedConn) Read(p []byte) (int, error) {
	return c.r.Read(p)
}
