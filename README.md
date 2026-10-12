# Caching Proxy

A forward HTTP/HTTPS caching proxy with regex-based filtering, MITM certificate generation, and SQLite storage. 
It intercepts requests, caches responses for URLs that match your rules, and serves them offline when the target is unreachable.

## Features

- If the target server is down and the request is cacheable, the cached response is served.
- Dynamically generates leaf certificates for HTTPS sites using a custom CA.
- Decide which requests to cache using whitelist and blacklist patterns (applied to URL and `Referer` header).
- Stores pages in multiple SQLite databases depending on the patterns specified in `filter.yml`.
- Passes raw bytes without decrypting if URL is blacklisted by all filtering groups.

## Installation

### Docker
Ensure you have [Docker](https://www.docker.com/) installed, add `filter.yml` file, then run:

```bash
docker build -t caching-proxy .
docker run --name=caching-proxy -p 8080:8080 caching-proxy
```

### Go
Ensure you have [Go](https://go.dev) installed, then run:

```bash
go install github.com/LammoGit/Caching-Proxy/cmd/caching-proxy@latest
```

## Usage

```text
caching-proxy --help
Usage of caching-proxy:
  -cert string
    	CA certificate filepath (default "./ca.crt")
  -db string
    	path to a folder storing cache (default "cache")
  -filter string
    	YAML config file for filtering (default "filter.yml")
  -key string
    	RSA private key of CA filepath (default "./key.key")
  -logger string
    	Path to save the logs
  -port string
    	proxy listen address (default ":8080")
  -v string
    	Level of verbosity (error, warning, info debug) (default "info")
```

## Quick Start

1. Create filtering config:
Create a `filter.yml` file in your working directory and add filtering groups and regex patterns for URLs you want to cache:
```yaml
groups:
  - filename: "wiki.db"
    whitelist:
      - "en\\.wikipedia\\.org"
  - filename: "docs.db"
    whitelist:
      - "docs"
      - "documentation"
    blacklist:
      - "\\.png$"
      - "\\.webp$"
      - "\\.jpg$"
      - "\\.svg$"
      - "\\.ico$"
  - filename: "arxiv_html.db"
    whitelist:
      - "arxiv\\.org\/(?:abs|html)"
  - filename: "arxiv_pdf.db"
    whitelist:
      - "arxiv\\.org\/(?:abs|pdf)"
```
2. Run the proxy. On first launch it automatically generates a root certificate (ca.crt) and private key (key.key) if they don't exist:
```bash
caching-proxy
```
3. Add generated certificate to trusted certificates in your browser.
4. Configure browser to route traffic through the proxy.

## How It Works

1. Creates or loads X.509 certificate and its private key.
2. Initializes an HTTP server on the given port using generated/loaded before certificate.
3. Listens for incoming CONNECT or regular HTTP requests.
4. On CONNECT request if URL is blacklisted by all groups just passes raw bytes.
5. On CONNECT request if URL isn't blacklisted by any group, then establishes TLS connection with a browser.
6. Once the connection is established and request is received it's decrypted.
7. If URL or `Referer` header are blacklisted, or neither are whitelisted, then just passes request to the target server and returns response.
8. Else request is sent via the client and stored in the cache, or tries to load from the cache if wasn't able to connect via the client.

## Roadmap

- Web UI for managing proxy settings and cache
- Support streamed content caching

## License

This project is licensed under the [MIT License](LICENSE).
