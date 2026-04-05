package main

import (
    "flag"
    "net/http"
    "fmt"
    "io"
    "strings"
    f "caching-proxy/filter"
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
)

func handleHTTP(w http.ResponseWriter, r *http.Request) {
    url := r.URL.String()
    ref := r.Header.Get("Referer")
    matched := filter.Match(url) || filter.Match(ref)

    fmt.Printf("HTTP %s %s\n", r.Method, url)
    fmt.Printf("Headers of %s\n", url)
    for k, v := range r.Header {
        fmt.Println(k, v)
    }

    if matched {
        fmt.Printf("Matched: %s\n", url)
    } else {
        fmt.Printf("Didn't match: %s\n", url)
    }

    r.RequestURI = ""

    resp, err := httpClient.Do(r)
    if err != nil {
        // Offline
        fmt.Printf("%s is unreachable\n", url)
        
        if !matched {
            return
        }

        // Load saved files
    }
    defer resp.Body.Close()

    for k, v := range resp.Header {
        w.Header()[k] = v
    }
    w.WriteHeader(resp.StatusCode)
    io.Copy(w, resp.Body)

    // Save file
}

func handleHTTPS(w http.ResponseWriter, r *http.Request) {
    // Not implemented
}

func connHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method == http.MethodConnect {
        // HTTPS
        handleHTTPS(w, r)
    } else {
        // HTTP
        handleHTTP(w, r)
    }
}

func main() {
    flag.Parse()

    err := filter.Load(*patternPath)
    if err != nil {
        filter.LoadRegex(*pattern)
    }

    httpClient = &http.Client {}

    server := &http.Server{
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
