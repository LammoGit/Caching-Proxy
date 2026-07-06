package proxy

import (
    "net"
    "time"
	"log/slog"
    "net/http"
    "crypto/tls"
    "fmt"
    "bytes"
    "bufio"
    "io"
    "encoding/json"
    "caching-proxy/filter"
    "caching-proxy/cache"
    "caching-proxy/signer"
)

func (proxy *Proxy) Match(req *http.Request) bool {
	// URL matches or Referer's URL matches
	res :=  proxy.Filter.Match(req.URL.String()) ||
    		proxy.Filter.Match(req.Header.Get("Referer"))
	if res {
		slog.Debug(fmt.Sprintf("Matched: %s", req.URL.String()))
	} else {
		slog.Debug(fmt.Sprintf("Didn't match: %s", req.URL.String()))
	}
	return res
}

type ProxySettings struct {
    ListenAddr  string
    WhitePath   string
    BlackPath   string
    DBPath      string
    CertPath    string
    KeyPath     string
}

type Proxy struct {
    Server    *http.Server
    Client    *http.Client
    Filter    *filter.Filter
    Cache     *cache.Cache
    Signer    *signer.Signer
    Settings  ProxySettings
}

func New(listenAddr, whitePath, blackPath, dbPath, certPath, keyPath string) (proxy *Proxy, err error) {
	proxy = &Proxy{}

    proxy.Filter, err = filter.New(whitePath, blackPath)
    if err != nil {
        return
    }

    proxy.Signer, err = signer.New(certPath, keyPath)
    if err != nil {
        return
    }

    proxy.Cache, err = cache.New(dbPath)
    if err != nil {
        return
    }

    transport := &http.Transport{
        DialContext: (&net.Dialer{
            Timeout:   5 * time.Second,
            KeepAlive: 0,
        }).DialContext,
        TLSHandshakeTimeout:   5 * time.Second,
        DisableKeepAlives:     true,
        MaxIdleConns:          0,
        IdleConnTimeout:       0,
    }

    proxy.Client = &http.Client {
        Timeout:   10 * time.Second,
        Transport: transport,
    }

    proxy.Server = &http.Server {
        Addr: listenAddr,
        Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
            if req.Method == http.MethodConnect {
                proxy.handleHTTPS(w, req)
            } else {
                proxy.handleHTTP(w, req)
            }
        }),
    }

    proxy.Settings = ProxySettings {
        ListenAddr:  listenAddr,
        WhitePath:   whitePath,
        BlackPath:   blackPath,
        DBPath:      dbPath,
        CertPath:    certPath,
        KeyPath:     keyPath,
    }

    return
}

func (proxy *Proxy) Run() error{
    fmt.Printf("Starting proxy on address: %s\n", proxy.Settings.ListenAddr)
    fmt.Printf("Cache filepath: %s\n", proxy.Settings.DBPath)
    fmt.Printf("Pathes to certificate and key files: %s %s\n", proxy.Settings.CertPath, proxy.Settings.KeyPath)

    fmt.Println("Whitelisted patterns:")
    for _, pat := range proxy.Filter.WhitePatterns {
        fmt.Println(pat)
    }

    fmt.Println("Blacklisted patterns:")
    for _, pat := range proxy.Filter.BlackPatterns {
    	fmt.Println(pat)
    }

    defer proxy.Cache.Close()
    if err := proxy.Server.ListenAndServe(); err != nil {
        return err
    }
    return nil
}

func (proxy *Proxy) handleHTTP(w http.ResponseWriter, req *http.Request) {
    matched := proxy.Match(req)
    slog.Debug(fmt.Sprintf("HTTP %s %s", req.Method, req.URL))
    proxy.forwardRequest(w, req, matched)
}

func (proxy *Proxy) handleHTTPS(w http.ResponseWriter, req *http.Request) {
    hijacker, ok := w.(http.Hijacker)
    if !ok {
        http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
        return
    }

    clientConn, _, err := hijacker.Hijack()
    if err != nil {
		slog.Error("Failed to hijack connection")
        return
    }
    defer clientConn.Close()

    if _, err := clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
        slog.Error("Failed to write connection established")
        return
    }

	url := *req.URL
    cert, err := proxy.Signer.GenerateCertificate(url)
    if err != nil {
		slog.Error(fmt.Sprintf("Failed to generate certificate for: %s", url))
        return
    }

    tlsConfig := &tls.Config{Certificates: []tls.Certificate{*cert}}
    tlsConn := tls.Server(clientConn, tlsConfig)
    defer tlsConn.Close()

    tlsReader := bufio.NewReader(tlsConn)
    inReq, err := http.ReadRequest(tlsReader)
    if err != nil {
    	slog.Error(fmt.Sprintf("Failed to read HTTPS request: %s", url))
        return
    }

    inReq.URL.Scheme = "https"
    inReq.URL.Host = inReq.Host
    inReq.RequestURI = ""

    matched := proxy.Match(inReq)
    slog.Debug(fmt.Sprintf("HTTPS %s %s", inReq.Method, inReq.URL))

    bufWriter := bufio.NewWriter(tlsConn)
    proxy.forwardRequest(bufWriter, inReq, matched)
    bufWriter.Flush()
}

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
            slog.Debug(fmt.Sprintf("Couldn't reach %s %s", method, url))
            if !proxy.loadResponse(w, req, matched) {
				slog.Debug(fmt.Sprintf("Couldn't load response from cache for %s %s", method, url))
                fmt.Fprintf(w, "HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\n\r\n")
            }
        }
        return
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
		slog.Error(fmt.Sprintf("Failed to read response body for %s %s", method, url))
    }

    resp.Body = io.NopCloser(bytes.NewReader(body))
    if err := resp.Write(w); err != nil {
		slog.Error(fmt.Sprintf("Failed to write response to %s %s", method, url))
    }

    if matched {
        if err := proxy.saveResponse(body, resp, req); err != nil {
            slog.Error(fmt.Sprintf("Failed to cache %s %s", method, url))
        } else {
            slog.Debug(fmt.Sprintf("Successfully cached %s %s", method, url))
		}
    }
}

func (proxy *Proxy) saveResponse(body []byte, resp *http.Response, req *http.Request) error {
    headers, _ := json.Marshal(resp.Header)
    url := req.URL.String()
    method := req.Method

    page := cache.Page {
        Url:      url,
        Method:   method,
        Headers:  headers,
        Content:  body,
    }
    return proxy.Cache.AddPage(page)
}

func (proxy *Proxy) loadResponse(w io.Writer, req *http.Request, matched bool) bool {
    var page cache.Page
    var err error

    if matched {
        page, err = proxy.Cache.GetPage(req.URL.String(), req.Method)
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

    fmt.Fprintf(w, "\r")
    if _, err := w.Write(page.Content); err != nil {
        return false
    }
    return true
}
