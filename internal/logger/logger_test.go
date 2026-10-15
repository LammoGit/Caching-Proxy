package logger

import (
	"log/slog"
	"strings"
	"sync"
	"testing"
)

// Check Logger object creation and match logs on expected output
func TestLoggerCreationAndMatchLogs(t *testing.T) {
	var b strings.Builder
	handler := New(&b, &slog.HandlerOptions{
		Level:       slog.LevelDebug,
		ReplaceAttr: WithShortener(20),
	})

	logger := slog.New(handler)

	// Check debug message
	logger.Debug("Debug")
	if strings.HasSuffix(b.String(), "\033[32mDebug\033[0m") {
		t.Fatalf("Debug log written is wrong")
	}

	// Check info message
	logger.Info("Info")
	if strings.HasSuffix(b.String(), "Info") {
		t.Fatalf("Info log written is wrong")
	}

	// Check warning message
	logger.Warn("Warning")
	if strings.HasSuffix(b.String(), "\033[33mWarning\033[0m") {
		t.Fatalf("Warning log written is wrong")
	}

	// Check error message
	logger.Error("Error")
	if strings.HasSuffix(b.String(), "\033[31mError\033[0m") {
		t.Fatalf("Error log written is wrong")
	}

	// Check message shortening while keeping colors
	logger.Debug("Long enough message to be shortened")
	if strings.HasSuffix(b.String(), "\033[32Long enough messa...\033[0m") {
		t.Fatalf("Long log written is wrong")
	}
}

// Check loggers thread safety, must be ran with --race for it to work
func TestHandlerThreadSafety(t *testing.T) {
	var b strings.Builder
	w := &b
	handler := New(w, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})

	logger := slog.New(handler)

	const goroutines = 2         // number of goroutines
	const logsPerGoroutine = 100 // number of logs for each log level per a single goroutine

	// Waiting group to wait for all goroutine to finish
	var wg sync.WaitGroup
	for i := range goroutines {
		wg.Go(func() {
			// Writing logs for each log level
			for j := range logsPerGoroutine {
				logger.Info("Info", slog.Int("worker", i), slog.Int("index", j))
				logger.Debug("Debug", slog.Int("worker", i), slog.Int("index", j))
				logger.Error("Error", slog.Int("worker", i), slog.Int("index", j))
			}
		})
	}

	wg.Wait()
}
