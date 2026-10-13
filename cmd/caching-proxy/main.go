package main

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"
	"strings"

	l "github.com/LammoGit/Caching-Proxy/internal/logger"
	p "github.com/LammoGit/Caching-Proxy/internal/proxy"
)

var (
	listenAddr = flag.String("port", ":8080", "proxy listen address")
	proxyAddr  = flag.String("proxy", "", "URL through which to route connections")
	filterPath = flag.String("filter", "filter.yml", "YAML config file for filtering")
	dbPath     = flag.String("db", "cache", "path to a folder storing cache")
	certPath   = flag.String("cert", "./ca.crt", "filepath to CA certificate")
	keyPath    = flag.String("key", "./key.key", "filepath to CA's private key")
	keyType    = flag.String("key_type", "ecdsa", "key type used for signing (rsa, ecdsa, ed25519)")
	logPath    = flag.String("logger", "", "Path to save the logs")
	verbosity  = flag.String("v", "info", "Level of verbosity (error, warning, info, debug)")
)

func validateArgs() error {
	if !slices.Contains([]string{"rsa", "ecdsa", "ed25519"}, strings.ToLower(*keyType)) {
		return fmt.Errorf("invalid key type, only `rsa`, `ecdsa`, and `ed25519` are supported: %s", *keyType)
	}
	if !slices.Contains([]string{"error", "warning", "info", "debug"}, strings.ToLower(*verbosity)) {
		return fmt.Errorf("invalid verbosity level, only `error`, `warning`, `info`, and `debug` are supported: %s", *verbosity)
	}
	return nil
}

func main() {
	flag.Parse()

	if err := validateArgs(); err != nil {
		panic(err)
	}

	var level slog.Level
	switch strings.ToLower(*verbosity) {
	case "debug":
		level = slog.LevelDebug
	case "warning", "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	var logDest io.Writer = os.Stderr

	if *logPath != "" {
		file, err := os.OpenFile(*logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to open log file, defaulting to stderr: %v\n", err)
		} else {
			defer file.Close()
			logDest = io.MultiWriter(os.Stderr, file)
		}
	}

	handler := l.New(logDest, &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: l.WithShortener(80),
	})
	logger := slog.New(handler)

	slog.SetDefault(logger)

	proxy, err := p.New(
		*listenAddr,
		*proxyAddr,
		*filterPath,
		*dbPath,
		*certPath,
		*keyType,
		*keyPath,
		p.WithLogger(logger),
		p.WithSignerCache(128),
	)
	if err != nil {
		panic(err)
	}

	err = proxy.Run()
	if err != nil {
		panic(err)
	}
}
