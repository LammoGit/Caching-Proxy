package main

import (
	"os"
	"io"
	"log/slog"
	"fmt"
    "flag"
	l "caching-proxy/logger"
    p "caching-proxy/proxy"
)

var (
    listenAddr   = flag.String("port", ":8080", "proxy listen address")
    dbPath       = flag.String("db", "./cache.db", "SQLite3 cache database filepath")
    whitePath    = flag.String("white", "./whitelist.txt", "Whitelist regex patterns filepath")
    blackPath    = flag.String("black", "./blacklist.txt", "Blacklist regex patterns filepath")
    certPath     = flag.String("cert", "./ca.crt", "CA certificate filepath")
    keyPath      = flag.String("key", "./key.key", "RSA private key of CA filepath")
	logPath      = flag.String("logger", "", "Path to save the logs")
	verbosity    = flag.String("v", "info", "Level of verbosity (error, warning, info debug)")
)

var (
    proxy p.Proxy
)

func main() {
    flag.Parse()

	var level slog.Level
	switch *verbosity {
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
		Level: level,
	})
	logger := slog.New(handler)

	slog.SetDefault(logger)

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
