package main

import (
    "flag"
    p "caching-proxy/proxy"
)

var (
    listenAddr   = flag.String("port", ":8080", "proxy listen address")
    dbPath       = flag.String("db", "./cache.db", "SQLite3 cache database filepath")
    whitePath    = flag.String("white", "./whitelist.txt", "Whitelist regex patterns filepath")
    blackPath    = flag.String("black", "./blacklist.txt", "Blacklist regex patterns filepath")
    certPath     = flag.String("cert", "./ca.crt", "CA certificate filepath")
    keyPath      = flag.String("key", "./key.key", "RSA private key of CA filepath")
)

var (
    proxy p.Proxy
)

func main() {
    flag.Parse()

    proxy, err := p.New(
        *listenAddr,
        *whitePath,
        *blackPath,
        *dbPath,
        *certPath,
        *keyPath,
    )
    if err != nil {
        panic(err)
    }

    err = proxy.Run()
    if err != nil {
        panic(err)
    }
}
