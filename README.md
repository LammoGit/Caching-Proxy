# Caching Proxy

A forward HTTP/HTTPS caching proxy with regex-based filtering, MITM certificate generation, and SQLite storage. 
It intercepts requests, caches responses for URLs that match your rules, and serves them offline when the origin is unreachable.

## Features

- If the origin is down and the request is cacheable, the cached response is served.
- Dynamically generates leaf certificates for HTTPS sites using a custom CA.
- Decide which requests to cache using whitelist and blacklist patterns (applied to URL and `Referer` header).
- Stores method, headers, and body in SQLite; updates on new responses.

## Installation

Ensure you have [Go](https://go.dev) installed, then run:

```bash
go install github.com/LammoGit/Caching-Proxy/cmd/caching-proxy@latest
```

## Usage

```bash
caching-proxy --help
Usage of caching-proxy:
  -blacklist string
        Blacklist regex patterns filepath (default "./blacklist.txt")
  -cert string
        CA certificate filepath (default "./ca.crt")
  -db string
        SQLite3 cache database filepath (default "./cache.db")
  -key string
        RSA private key of CA filepath (default "./key.key")
  -logger string
        Path to save the logs
  -port string
        proxy listen address (default ":8080")
  -v string
        Level of verbosity (error, warning, info debug) (default "info")
  -whitelist string
        Whitelist regex patterns filepath (default "./whitelist.txt")
```

## Quick Start

1. Create filter rules:
Create a `whitelist.txt` file in your working directory and add regex patterns for domains you want to cache:
```whitelist.txt
https?://en\.wikipedia\.org.*
.*\.pdf
```
2. Run the proxy. On first launch it automatically generates a root certificate (ca.crt) and private key (key.key) if they don't exist:
```bash
caching-proxy -port ":8080" -whitelist "./whitelist.txt"
```
3. Add generated certificate to trusted certificates in your browser
4. Configure browser to route traffic through the proxy

## How It Works

1. Creates or loads X.509 certificate with its private key.
2. Initializes an HTTP server on the given port using generated/loaded before certificate.
3. Listens for incoming CONNECT or regular HTTP requests.
4. On CONNECT request creates a client with the requested URL.
5. If request's URL or Referer header don't match, then it works normally.
6. If request matched and proxy received response from the target URL, then save it into cache (SQLite DB).
7. If request matched and proxy didn't receive a response from the target URL, then load response from the cache.

## Roadmap

- [ ] Improve CLI interface
- [ ] Container image
- [ ] Databases grouped by filters
- [ ] Web UI for managing proxy settings and cache
- [ ] Support streamed content caching
- [ ] Connect to the next proxy in chain

## License

This project is licensed under the [MIT License](LICENSE).
