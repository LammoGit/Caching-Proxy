package main

import (
    "flag"
    "net/http"
    "fmt"
    "io"
    "strings"
    "encoding/json"
    "github.com/cespare/xxhash/v2"
    f "caching-proxy/filter"
    c "caching-proxy/cache"
)

var (
    listenAddr   = flag.String("port", ":8080", "proxy listen address")
    dbPath       = flag.String("db", "./cache.db", "SQLite3 cache database filepath")
    pattern      = flag.String("pattern", ".*", "URL regex pattern")
    patternPath  = flag.String("patterns", "None", "URL regex patterns file")
)

var (
    httpClient  *http.Client
    filter      f.Filter
    cache       c.Cache
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

func LoadResponse(w http.ResponseWriter, req *http.Request, matched RequestType) error {
    fmt.Printf("Loading %s from cache...\n", req.URL)

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
        w.WriteHeader(http.StatusNotFound)
        w.Write([]byte("404 Not Found\n"))
        return err
    }

    var headers http.Header
    json.Unmarshal([]byte(page.Headers), &headers)
    for k, v := range headers {
        w.Header()[k] = v
    }
    w.WriteHeader(http.StatusOK)
    w.Write(page.Content)

    return nil
}

func handleHTTP(w http.ResponseWriter, req *http.Request) {
    url := req.URL.String()
    matched := Match(req)

    fmt.Printf("HTTP %s %s\n", req.Method, url)
    fmt.Printf("Headers of %s\n", url)
    for k, v := range req.Header {
        fmt.Println(k, v)
    }

    req.RequestURI = ""

    resp, err := httpClient.Do(req)
    if err != nil {
        fmt.Printf("%s is unreachable\n", url)
        LoadResponse(w, req, matched)
        return
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)

    for k, v := range resp.Header {
        w.Header()[k] = v
    }
    w.WriteHeader(resp.StatusCode)
    w.Write(body)

    // Save file
    if resp.StatusCode == http.StatusOK {
        if err := SaveResponse(body, resp, req, matched); err != nil {
            fmt.Printf("Failed to cache %s: %s", url, err)
        }
    }
}

func handleHTTPS(w http.ResponseWriter, req *http.Request) {
    // Not implemented
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

    err := filter.Load(*patternPath)
    if err != nil {
        fmt.Println(err)
        filter.LoadRegex(*pattern)
    }

    err = cache.Load(*dbPath)
    if err != nil {
        fmt.Println(err)
    }
    defer cache.Close()

    httpClient = &http.Client {}

    server := &http.Server {
        Addr: *listenAddr,
        Handler: http.HandlerFunc(connHandler),
    }

    fmt.Printf("Server started on %s\n", *listenAddr)
    fmt.Printf("Cache database path: %s\n", *dbPath)
    fmt.Printf("Pattern specified: %s\n", strings.Join(filter.Valid, "|"))

    if err := server.ListenAndServe(); err != nil {
        fmt.Printf("Listen error: %s\n", err)
    }
}
