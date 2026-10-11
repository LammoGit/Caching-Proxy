package filter

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

/* Testing Config */

var (
	whitelistBytes = []byte("^\\w+$\n^\\d+$")
	blacklistBytes = []byte("^\\d$\n^\\w$\n^\\s$")
	whitelisted    = []string{"abc", "123"}
	blacklisted    = []string{" ", "\t"}
	bothlisted     = []string{"a", "1"}
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

	if filter.logger == nil || filter.logger.Handler() != slog.DiscardHandler {
		t.Fatalf("Logger is assigned")
	}
}

// Create a new filter with an invalid whitelist path
func TestCreateFilterWithInvalidWhitelistPath(t *testing.T) {
	dir := t.TempDir()
	whitelistPath := filepath.Join(dir, "wl.txt")
	blacklistPath := filepath.Join(dir, "bl.txt")

	if err := os.WriteFile(blacklistPath, blacklistBytes, 0644); err != nil {
		t.Fatalf("Failed to write bytes into blacklist file: %s", err)
	}

	_, err := New(whitelistPath, blacklistPath)
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
	blacklistPath := filepath.Join(dir, "bl.txt")

	if err := os.WriteFile(whitelistPath, whitelistBytes, 0644); err != nil {
		t.Fatalf("Failed to write bytes into whitelist file: %s", err)
	}

	_, err := New(whitelistPath, blacklistPath)
	if err == nil {
		t.Fatalf("Created filter with an invalid path")
	}

	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Error is not about the invalid blacklist path: %s", err)
	}
}

// Create a new filter with an empty whitelist path
func TestCreateFilterWithEmptyWhitelistPath(t *testing.T) {
	dir := t.TempDir()
	blacklistPath := filepath.Join(dir, "bl.txt")

	if err := os.WriteFile(blacklistPath, blacklistBytes, 0644); err != nil {
		t.Fatalf("Failed to write bytes into blacklist file: %s", err)
	}

	_, err := New("", blacklistPath)
	if err != nil {
		t.Fatalf("Failed to create a filter: %s", err)
	}
}

// Create a new filter with an empty blacklist path
func TestCreateFilterWithEmptyBlacklistPath(t *testing.T) {
	dir := t.TempDir()
	whitelistPath := filepath.Join(dir, "wl.txt")

	if err := os.WriteFile(whitelistPath, whitelistBytes, 0644); err != nil {
		t.Fatalf("Failed to write bytes into whitelist file: %s", err)
	}

	_, err := New(whitelistPath, "")
	if err != nil {
		t.Fatalf("Failed to create a filter: %s", err)
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
		if filter.Match(value) != MatchWhiteList {
			fmt.Fprintf(&b, "%s\n", value)
			res = true
		}
	}

	if res {
		t.Fatalf("Didn't whitelist whitelisted:\n%s", b.String())
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
		if filter.Match(value) != MatchBlackList {
			fmt.Fprintf(&b, "%s\n", value)
			res = true
		}
	}

	if res {
		t.Fatalf("Didn't blacklist blacklisted:\n%s", b.String())
	}
}

// Match bothlisted
func TestMatchingBothlisted(t *testing.T) {
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
	for _, value := range bothlisted {
		if filter.Match(value) != MatchBlackList {
			fmt.Fprintf(&b, "%s\n", value)
			res = true
		}
	}

	if res {
		t.Fatalf("Didn't blacklist both whitelisted and blacklisted:\n%s", b.String())
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
		if filter.Match(value) != MatchNeither {
			fmt.Fprintf(&b, "%s\n", value)
			res = true
		}
	}

	if res {
		t.Fatalf("Matched neither whitelisted, nor blacklisted:\n%s", b.String())
	}
}

// Match blacklisted with only blacklist set
func TestMatchBlacklistedBlackListSet(t *testing.T) {
	dir := t.TempDir()
	blacklistPath := filepath.Join(dir, "bl.txt")

	if err := os.WriteFile(blacklistPath, blacklistBytes, 0644); err != nil {
		t.Fatalf("Failed to write bytes into blacklist file: %s", err)
	}

	filter, err := New("", blacklistPath)
	if err != nil {
		t.Fatalf("Failed to create a filter: %s", err)
	}

	res := false
	var b strings.Builder
	for _, value := range blacklisted {
		if filter.Match(value) != MatchBlackList {
			fmt.Fprintf(&b, "%s\n", value)
			res = true
		}
	}

	if res {
		t.Fatalf("Didn't blacklist blacklisted: \n%s", b.String())
	}
}

// Match non-blacklisted with only blacklist set
func TestMatchNonBlacklistedBlackListSet(t *testing.T) {
	dir := t.TempDir()
	blacklistPath := filepath.Join(dir, "bl.txt")

	if err := os.WriteFile(blacklistPath, blacklistBytes, 0644); err != nil {
		t.Fatalf("Failed to write bytes into blacklist file: %s", err)
	}

	filter, err := New("", blacklistPath)
	if err != nil {
		t.Fatalf("Failed to create a filter: %s", err)
	}

	res := false
	var b strings.Builder
	for _, value := range slices.Concat(whitelisted, nonmatching) {
		if filter.Match(value) != MatchWhiteList {
			fmt.Fprintf(&b, "%s\n", value)
			res = true
		}
	}

	if res {
		t.Fatalf("Didn't whitelist non-blacklisted: \n%s", b.String())
	}
}

// Match whitelisted with only whitelist set
func TestMatchWhitelistedWhiteListSet(t *testing.T) {
	dir := t.TempDir()
	whitelistPath := filepath.Join(dir, "bl.txt")

	if err := os.WriteFile(whitelistPath, whitelistBytes, 0644); err != nil {
		t.Fatalf("Failed to write bytes into whitelist file: %s", err)
	}

	filter, err := New(whitelistPath, "")
	if err != nil {
		t.Fatalf("Failed to create a filter: %s", err)
	}

	res := false
	var b strings.Builder
	for _, value := range whitelisted {
		if filter.Match(value) != MatchWhiteList {
			fmt.Fprintf(&b, "%s\n", value)
			res = true
		}
	}

	if res {
		t.Fatalf("Didn't whitelist whitelisted: \n%s", b.String())
	}
}

// Match non-whitelisted with only whitelist set
func TestMatchNonWhitelistedWhiteListSet(t *testing.T) {
	dir := t.TempDir()
	whitelistPath := filepath.Join(dir, "bl.txt")

	if err := os.WriteFile(whitelistPath, whitelistBytes, 0644); err != nil {
		t.Fatalf("Failed to write bytes into whitelist file: %s", err)
	}

	filter, err := New(whitelistPath, "")
	if err != nil {
		t.Fatalf("Failed to create a filter: %s", err)
	}

	res := false
	var b strings.Builder
	for _, value := range slices.Concat(blacklisted, nonmatching) {
		if filter.Match(value) != MatchBlackList {
			fmt.Fprintf(&b, "%s\n", value)
			res = true
		}
	}

	if res {
		t.Fatalf("Didn't blacklist non-whitelisted: \n%s", b.String())
	}
}

// Match with neither white or black lists set
func TestMatchNeitherListSet(t *testing.T) {
	filter, err := New("", "")
	if err != nil {
		t.Fatalf("Failed to create a filter: %s", err)
	}

	res := false
	var b strings.Builder
	for range 10 {
		value := rand.Text()
		if filter.Match(value) != MatchWhiteList {
			fmt.Fprintf(&b, "%s\n", value)
			res = true
		}
	}

	if res {
		t.Fatalf("Didn't whitelist with aren't set lists:\n%s", b.String())
	}
}
