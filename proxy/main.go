package main

import (
    "flag"
    "net/http"
    "crypto/tls"
    "fmt"
    "bytes"
    "bufio"
    "io"
    "encoding/json"
    "github.com/cespare/xxhash/v2"
    f "caching-proxy/filter"
    c "caching-proxy/cache"
    s "caching-proxy/signer"
)

var (
    listenAddr   = flag.String("port", ":8080", "proxy listen address")
    dbPath       = flag.String("db", "./cache.db", "SQLite3 cache database filepath")
    whitePath    = flag.String("white", "./whitelist.txt", "Whitelist regex patterns filepath")
    blackPath    = flag.String("black", "./blacklist.txt", "Blacklist regex patterns filepath")
    certPath     = flag.String("cert", "./ca.cert", "CA certificate filepath")
    keyPath      = flag.String("key", "./key.key", "RSA private key of CA filepath")
)

var (
    httpClient  *http.Client
    filter      f.Filter
    cache       c.Cache
    signer      s.Signer
)

type RequestType int
const (
    DidntMatch RequestType = iota
    PageMatch
    AssetMatch
)

func Match(req *http.Request) RequestType {
    if filter.Match(req.URL.String()) {
        return PageMatch
    }

    if !filter.Match(req.Header.Get("Referer")) {
        return DidntMatch
    }

    if req.Header.Get("Sec-Fetch-Mode") == "navigate" {
        return AssetMatch
    }

    return DidntMatch
}

func SaveResponse(body []byte, resp *http.Response, req *http.Request, matched RequestType) error {
    headers, _ := json.Marshal(resp.Header)
    hash := xxhash.Sum64(body)
    url := req.URL.String()

    fmt.Printf("Saving %s to cache...\n", url)

    switch matched {
    case PageMatch:
        page := c.Page {
            Url:      url,
            Headers:  headers,
            Content:  body,
            Hash:     hash,
        }
        fmt.Printf("Saved %s to cache\n", url)
        return cache.AddPage(page)

    case AssetMatch:
        pageURL := req.Header.Get("Referer")
        if pageURL == "" {
            return fmt.Errorf("Asset missing Referer header")
        }

        asset := c.Page {
            Url:      url,
            Headers:  headers,
            Content:  body,
            Hash:     hash,
        }
        fmt.Printf("Saved %s to cache\n", url)
        return cache.AddAsset(pageURL, asset)

    default:
        return nil
    }
}

func writeCachedResponse(w io.Writer, req *http.Request, matched RequestType) bool {
    fmt.Println("Reading", req.URL, "from cache")
    var page c.Page
    var err error

    switch matched {
    case PageMatch:
        page, err = cache.GetPage(req.URL.String())
    case AssetMatch:
        page, err = cache.GetAsset(req.URL.String())
    default:
        err = fmt.Errorf("%s didn't match", req.URL)
    }

    if err != nil {
        fmt.Fprintf(w, "HTTP/1.1 404 Not Found\r\nContent-Length: 0\r\n\r\n")
        return false
    }

    var headers http.Header
    json.Unmarshal([]byte(page.Headers), &headers)

    fmt.Fprintf(w, "HTTP/1.1 %d %s\r\n", http.StatusOK, http.StatusText(http.StatusOK))
    if err := headers.Write(w); err != nil {
        fmt.Printf("Failed to write headers: %v\n", err)
        return false
    }
    headers.Write(w)
    fmt.Fprintf(w, "\r\n")
    if _, err := w.Write(page.Content); err != nil {
        fmt.Printf("Failed to write body: %v\n", err)
        return false
    }
    fmt.Println("Written", req.URL, "from cache")
    return true
}

func forwardRequest(w io.Writer, req *http.Request, matched RequestType) {
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

    resp, err := httpClient.Do(req)
    if err != nil {
        fmt.Printf("%s is unreachable: %v\n", req.URL, err)
        if !writeCachedResponse(w, req, matched) {
            fmt.Fprintf(w, "HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\n\r\n")
        }
        return
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        fmt.Printf("Failed to read response body: %v\n", err)
    }

    resp.Body = io.NopCloser(bytes.NewReader(body))
    if err := resp.Write(w); err != nil {
        fmt.Printf("Failed to write response: %v\n", err)
    }

    if resp.StatusCode == http.StatusOK {
        if err := SaveResponse(body, resp, req, matched); err != nil {
            fmt.Printf("Failed to cache %s: %v\n", req.URL, err)
        }
    }
}

func handleHTTP(w http.ResponseWriter, req *http.Request) {
    matched := Match(req)
    fmt.Printf("HTTP %s %s\n", req.Method, req.URL)
    forwardRequest(w, req, matched)
}

func handleHTTPS(w http.ResponseWriter, req *http.Request) {
    hijacker, ok := w.(http.Hijacker)
    if !ok {
        http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
        return
    }

    clientConn, _, err := hijacker.Hijack()
    if err != nil {
        fmt.Println(err)
        return
    }
    defer clientConn.Close()

    if _, err := clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
        fmt.Printf("Failed to write connection established: %v\n", err)
        return
    }

    cert, err := signer.GenerateCertificate(*req.URL)
    if err != nil {
        fmt.Println(err)
        return
    }

    tlsConfig := &tls.Config{Certificates: []tls.Certificate{*cert}}
    tlsConn := tls.Server(clientConn, tlsConfig)
    defer tlsConn.Close()

    tlsReader := bufio.NewReader(tlsConn)
    inReq, err := http.ReadRequest(tlsReader)
    if err != nil {
        fmt.Println(err)
        return
    }

    inReq.URL.Scheme = "https"
    inReq.URL.Host = inReq.Host
    inReq.RequestURI = ""

    matched := Match(inReq)
    fmt.Printf("HTTPS %s %s\n", inReq.Method, inReq.URL)

    bufWriter := bufio.NewWriter(tlsConn)
    forwardRequest(bufWriter, inReq, matched)
    bufWriter.Flush()
}

func connHandler(w http.ResponseWriter, req *http.Request) {
    if req.Method == http.MethodConnect {
        // HTTPS
        handleHTTPS(w, req)
    } else {
        // HTTP
        handleHTTP(w, req)
    }
}

func main() {
    flag.Parse()

    err := filter.Load(*whitePath, *blackPath)
    if err != nil {
        panic(err)
    }

    err = cache.Load(*dbPath)
    if err != nil {
        panic(err)
    }
    defer cache.Close()

    err = signer.LoadOrCreate(*certPath, *keyPath)
    if err != nil {
        panic(err)
    }

    httpClient = &http.Client {}

    server := &http.Server {
        Addr: *listenAddr,
        Handler: http.HandlerFunc(connHandler),
        TLSConfig: &tls.Config {
            InsecureSkipVerify: true,
        },
    }

    fmt.Printf("Server started on %s\n", *listenAddr)
    fmt.Printf("Cache database path: %s\n", *dbPath)
    fmt.Println("Whitelisted patterns:")
    for _, pattern := range filter.WhitePatterns {
        fmt.Println(pattern)
    }

    fmt.Println("Blacklisted patterns:")
    for _, pattern := range filter.BlackPatterns {
        fmt.Println(pattern)
    }

    if err := server.ListenAndServe(); err != nil {
        fmt.Printf("Listen error: %s\n", err)
    }
}
