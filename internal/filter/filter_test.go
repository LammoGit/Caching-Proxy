package filter

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

/* Testing Config */

var (
	whitelistBytes = []byte("^\\w+$\n^\\d+$")
	blacklistBytes = []byte("^\\d$\n^\\w$\n^\\s$")
	whitelisted    = []string{"abc", "123"}
	blacklisted    = []string{" ", "\t"}
	graylisted     = []string{"a", "1"}
	nonmatching    = []string{"a,", "b."}
)

/* Tests */

// Create a new filter
func TestCreateFilter(t *testing.T) {
	dir := t.TempDir()
	whitelistPath := filepath.Join(dir, "wl.txt")
	blacklistPath := filepath.Join(dir, "bl.txt")

	if err := os.WriteFile(whitelistPath, whitelistBytes, 0644); err != nil {
		t.Fatalf("Failed to write bytes into whitelist file: %s", err)
	}

	if err := os.WriteFile(blacklistPath, blacklistBytes, 0644); err != nil {
		t.Fatalf("Failed to write bytes into blacklist file: %s", err)
	}

	if _, err := New(whitelistPath, blacklistPath); err != nil {
		t.Fatalf("Failed to create a filter: %s", err)
	}
}

// Create a new filter with a logger
func TestCreateFilterWithLogger(t *testing.T) {
	dir := t.TempDir()
	whitelistPath := filepath.Join(dir, "wl.txt")
	blacklistPath := filepath.Join(dir, "bl.txt")

	if err := os.WriteFile(whitelistPath, whitelistBytes, 0644); err != nil {
		t.Fatalf("Failed to write bytes into whitelist file: %s", err)
	}

	if err := os.WriteFile(blacklistPath, blacklistBytes, 0644); err != nil {
		t.Fatalf("Failed to write bytes into blacklist file: %s", err)
	}

	var b strings.Builder
	logger := slog.New(slog.NewTextHandler(&b, nil))

	filter, err := New(
		whitelistPath,
		blacklistPath,
		WithLogger(logger),
	)
	if err != nil {
		t.Fatalf("Failed to create a filter: %s", err)
	}

	if filter.logger == nil {
		t.Fatalf("Logger isn't assigned")
	}
}

// Create a new filter with a nil logger
func TestCreateFilterWithNilLogger(t *testing.T) {
	dir := t.TempDir()
	whitelistPath := filepath.Join(dir, "wl.txt")
	blacklistPath := filepath.Join(dir, "bl.txt")

	if err := os.WriteFile(whitelistPath, whitelistBytes, 0644); err != nil {
		t.Fatalf("Failed to write bytes into whitelist file: %s", err)
	}

	if err := os.WriteFile(blacklistPath, blacklistBytes, 0644); err != nil {
		t.Fatalf("Failed to write bytes into blacklist file: %s", err)
	}

	filter, err := New(
		whitelistPath,
		blacklistPath,
		WithLogger(nil),
	)
	if err != nil {
		t.Fatalf("Failed to create a filter: %s", err)
	}

	if filter.logger != nil {
		t.Fatalf("Logger is assigned")
	}
}

// Create a new filter with an invalid whitelist path
func TestCreateFilterWithInvalidWhitelistPath(t *testing.T) {
	dir := t.TempDir()
	blacklistPath := filepath.Join(dir, "bl.txt")

	if err := os.WriteFile(blacklistPath, blacklistBytes, 0644); err != nil {
		t.Fatalf("Failed to write bytes into blacklist file: %s", err)
	}

	_, err := New("", blacklistPath)
	if err == nil {
		t.Fatalf("Created filter with an invalid path")
	}

	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Error is not about the invalid whitelist path: %s", err)
	}
}

// Create a new filter with an invalid blacklist path
func TestCreateFilterWithInvalidBlacklistPath(t *testing.T) {
	dir := t.TempDir()
	whitelistPath := filepath.Join(dir, "wl.txt")

	if err := os.WriteFile(whitelistPath, whitelistBytes, 0644); err != nil {
		t.Fatalf("Failed to write bytes into whitelist file: %s", err)
	}

	_, err := New(whitelistPath, "")
	if err == nil {
		t.Fatalf("Created filter with an invalid path")
	}

	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Error is not about the invalid blacklist path: %s", err)
	}
}

// Create a new filter with invalid whitelist patterns
func TestCreateFilterWithInvalidWhitelistPatterns(t *testing.T) {
	dir := t.TempDir()
	whitelistPath := filepath.Join(dir, "wl.txt")
	blacklistPath := filepath.Join(dir, "bl.txt")

	if err := os.WriteFile(whitelistPath, []byte("+|?\n*"), 0644); err != nil {
		t.Fatalf("Failed to write bytes into whitelist file: %s", err)
	}

	if err := os.WriteFile(blacklistPath, blacklistBytes, 0644); err != nil {
		t.Fatalf("Failed to write bytes into blacklist file: %s", err)
	}

	filter, err := New(whitelistPath, blacklistPath)
	if err != nil {
		t.Fatalf("Failed to create a filter: %s", err)
	}

	if filter.WhiteRegex != nil {
		t.Fatalf("Regex pattern is compiled from invalid patterns")
	}
}

// Create a new filter with invalid blacklist patterns
func TestCreateFilterWithInvalidBlacklistPatterns(t *testing.T) {
	dir := t.TempDir()
	whitelistPath := filepath.Join(dir, "wl.txt")
	blacklistPath := filepath.Join(dir, "bl.txt")

	if err := os.WriteFile(whitelistPath, whitelistBytes, 0644); err != nil {
		t.Fatalf("Failed to write bytes into whitelist file: %s", err)
	}

	if err := os.WriteFile(blacklistPath, []byte("+|?\n*"), 0644); err != nil {
		t.Fatalf("Failed to write bytes into blacklist file: %s", err)
	}

	filter, err := New(whitelistPath, blacklistPath)
	if err != nil {
		t.Fatalf("Failed to create a filter: %s", err)
	}

	if filter.BlackRegex != nil {
		t.Fatalf("Regex pattern is compiled from invalid patterns")
	}
}

// Match whitelisted
func TestMatchingWhitelisted(t *testing.T) {
	dir := t.TempDir()
	whitelistPath := filepath.Join(dir, "wl.txt")
	blacklistPath := filepath.Join(dir, "bl.txt")

	if err := os.WriteFile(whitelistPath, whitelistBytes, 0644); err != nil {
		t.Fatalf("Failed to write bytes into whitelist file: %s", err)
	}

	if err := os.WriteFile(blacklistPath, blacklistBytes, 0644); err != nil {
		t.Fatalf("Failed to write bytes into blacklist file: %s", err)
	}

	filter, err := New(whitelistPath, blacklistPath)
	if err != nil {
		t.Fatalf("Failed to create a filter: %s", err)
	}

	res := false
	var b strings.Builder
	for _, value := range whitelisted {
		if !filter.Match(value) {
			fmt.Fprintf(&b, "%s\n", value)
			res = true
		}
	}

	if res {
		t.Fatalf("Didn't match whitelisted:\n%s", b.String())
	}
}

// Match blacklisted
func TestMatchingBlacklisted(t *testing.T) {
	dir := t.TempDir()
	whitelistPath := filepath.Join(dir, "wl.txt")
	blacklistPath := filepath.Join(dir, "bl.txt")

	if err := os.WriteFile(whitelistPath, whitelistBytes, 0644); err != nil {
		t.Fatalf("Failed to write bytes into whitelist file: %s", err)
	}

	if err := os.WriteFile(blacklistPath, blacklistBytes, 0644); err != nil {
		t.Fatalf("Failed to write bytes into blacklist file: %s", err)
	}

	filter, err := New(whitelistPath, blacklistPath)
	if err != nil {
		t.Fatalf("Failed to create a filter: %s", err)
	}

	res := false
	var b strings.Builder
	for _, value := range blacklisted {
		if filter.Match(value) {
			fmt.Fprintf(&b, "%s\n", value)
			res = true
		}
	}

	if res {
		t.Fatalf("Matched blacklisted:\n%s", b.String())
	}
}

// Match graylisted
func TestMatchingGraylisted(t *testing.T) {
	dir := t.TempDir()
	whitelistPath := filepath.Join(dir, "wl.txt")
	blacklistPath := filepath.Join(dir, "bl.txt")

	if err := os.WriteFile(whitelistPath, whitelistBytes, 0644); err != nil {
		t.Fatalf("Failed to write bytes into whitelist file: %s", err)
	}

	if err := os.WriteFile(blacklistPath, blacklistBytes, 0644); err != nil {
		t.Fatalf("Failed to write bytes into blacklist file: %s", err)
	}

	filter, err := New(whitelistPath, blacklistPath)
	if err != nil {
		t.Fatalf("Failed to create a filter: %s", err)
	}

	res := false
	var b strings.Builder
	for _, value := range graylisted {
		if filter.Match(value) {
			fmt.Fprintf(&b, "%s\n", value)
			res = true
		}
	}

	if res {
		t.Fatalf("Matched both whitelisted and blacklisted:\n%s", b.String())
	}
}

// Match non-matching
func TestMatchingNonMatching(t *testing.T) {
	dir := t.TempDir()
	whitelistPath := filepath.Join(dir, "wl.txt")
	blacklistPath := filepath.Join(dir, "bl.txt")

	if err := os.WriteFile(whitelistPath, whitelistBytes, 0644); err != nil {
		t.Fatalf("Failed to write bytes into whitelist file: %s", err)
	}

	if err := os.WriteFile(blacklistPath, blacklistBytes, 0644); err != nil {
		t.Fatalf("Failed to write bytes into blacklist file: %s", err)
	}

	filter, err := New(whitelistPath, blacklistPath)
	if err != nil {
		t.Fatalf("Failed to create a filter: %s", err)
	}

	res := false
	var b strings.Builder
	for _, value := range nonmatching {
		if filter.Match(value) {
			fmt.Fprintf(&b, "%s\n", value)
			res = true
		}
	}

	if res {
		t.Fatalf("Matched neither whitelisted, nor blacklisted:\n%s", b.String())
	}
}
