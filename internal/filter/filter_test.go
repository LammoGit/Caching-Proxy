package filter

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

/* Testing Config */

var (
	filterConfigBytes = []byte(`
storage_path: "./db"
groups:
  - filename: "a.db"
    whitelist:
      - "^a$"
      - "^a\\d$"
      - "^shared$"
    blacklist:
      - "^a1$"
      - "^aa$"
  - filename: "b.db"
    whitelist:
      - "^b$"
      - "^b\\d$"
      - "^shared$"
    blacklist:
      - "^b1$"
      - "^bb$"
`)
	matchResults = map[string]map[string]FilterMatch{
		"a":  {"a.db": MatchWhiteList, "b.db": MatchNeither},
		"a1": {"b.db": MatchNeither},
		"a2": {"a.db": MatchWhiteList, "b.db": MatchNeither},
		"aa": {"b.db": MatchNeither},

		"b":  {"a.db": MatchNeither, "b.db": MatchWhiteList},
		"b1": {"a.db": MatchNeither},
		"b2": {"a.db": MatchNeither, "b.db": MatchWhiteList},
		"bb": {"a.db": MatchNeither},

		"shared": {"a.db": MatchWhiteList, "b.db": MatchWhiteList},
	}
)

/* Tests */

// Create a new filter
func TestCreateFilter(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "filter.yml")

	if err := os.WriteFile(configPath, filterConfigBytes, 0644); err != nil {
		t.Fatalf("Failed to write bytes into filter config file: %s", err)
	}

	if _, err := New(configPath); err != nil {
		t.Fatalf("Failed to create a filter: %s", err)
	}
}

// Create a new filter with an invalid config
func TestCreateFilterWithInvalidConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "filter.yml")

	if err := os.WriteFile(configPath, []byte("~!a"), 0644); err != nil {
		t.Fatalf("Failed to write bytes into filter config file: %s", err)
	}

	if _, err := New(configPath); err == nil {
		t.Fatal("Create a filter with an invalid config")
	}
}

// Create a new filter with a logger
func TestCreateFilterWithLogger(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "filter.yml")

	if err := os.WriteFile(configPath, filterConfigBytes, 0644); err != nil {
		t.Fatalf("Failed to write bytes into filter config file: %s", err)
	}

	var b strings.Builder
	logger := slog.New(slog.NewTextHandler(&b, nil))

	filter, err := New(
		configPath,
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
	configPath := filepath.Join(dir, "filter.yml")

	if err := os.WriteFile(configPath, filterConfigBytes, 0644); err != nil {
		t.Fatalf("Failed to write bytes into filter config file: %s", err)
	}

	filter, err := New(
		configPath,
		WithLogger(nil),
	)
	if err != nil {
		t.Fatalf("Failed to create a filter: %s", err)
	}

	if filter.logger == nil || filter.logger.Handler() != slog.DiscardHandler {
		t.Fatalf("Logger is assigned")
	}
}

// Create a new filter with an invalid filter config path
func TestCreateFilterWithInvalidWhitelistPath(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "filter.yml")

	_, err := New(configPath)
	if err == nil {
		t.Fatalf("Created filter with an invalid path")
	}

	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Error is not about the invalid whitelist path: %s", err)
	}
}

func TestCreateWithInvalidStoragePath(t *testing.T) {}

// Create a new filter with invalid whitelist patterns
func TestCreateFilterWithInvalidWhitelistPatterns(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "filter.yml")

	data := []byte(`
storage_path: "./db"
groups:
  - filename: "db.db"
    whitelist:
      - "^*+?|"
`)
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("Failed to write bytes into filter config file: %s", err)
	}

	filter, err := New(configPath)
	if err != nil {
		t.Fatalf("Failed to create a filter: %s", err)
	}

	group, ok := filter.Groups["db.db"]
	if !ok {
		t.Fatal("Group wasn't created")
	}

	if group.WhiteRegex != nil {
		t.Fatalf("Regex pattern is compiled from invalid patterns")
	}
}

// Create a new filter with invalid blacklist patterns
func TestCreateFilterWithInvalidBlacklistPatterns(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "filter.yml")

	data := []byte(`
storage_path: "./db"
groups:
  - filename: "db.db"
    blacklist:
      - "^*+?|"
`)
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("Failed to write bytes into filter config file: %s", err)
	}

	filter, err := New(configPath)
	if err != nil {
		t.Fatalf("Failed to create a filter: %s", err)
	}

	group, ok := filter.Groups["db.db"]
	if !ok {
		t.Fatal("Group wasn't created")
	}

	if group.BlackRegex != nil {
		t.Fatalf("Regex pattern is compiled from invalid patterns")
	}
}

// Create a new filter with invalid blacklist patterns
func TestCreateFilterWithInvalidAndValidPatterns(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "filter.yml")

	data := []byte(`
storage_path: "./db"
groups:
  - filename: "db.db"
    whitelist:
      - "^*+?|"
      - "^whitelisted$"
    blacklist:
      - "^*+?|"
      - "^blacklisted$"
`)
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("Failed to write bytes into filter config file: %s", err)
	}

	filter, err := New(configPath)
	if err != nil {
		t.Fatalf("Failed to create a filter: %s", err)
	}

	group, ok := filter.Groups["db.db"]
	if !ok {
		t.Fatal("Group wasn't created")
	}

	if group.BlackRegex == nil || group.WhiteRegex == nil {
		t.Fatalf("Regex pattern isn't compiled from partially invalid patterns")
	}

	res := filter.Match("neither")
	expected := map[string]FilterMatch{"db.db": MatchNeither}
	if !maps.Equal(res, expected) {
		if match, ok := res["db.db"]; !ok {
			t.Error("Blacklisted pattern that is neither whitelisted, nor blacklisted")
		} else if match == MatchWhiteList {
			t.Error("Whitelisted pattern that is neither whitelisted, nor blacklisted")
		} else {
			t.Error("Returns more groups than expected or has an invalid structure")
		}
	}

	res = filter.Match("whitelisted")
	expected = map[string]FilterMatch{"db.db": MatchWhiteList}
	if !maps.Equal(res, expected) {
		if match, ok := res["db.db"]; !ok {
			t.Error("Blacklisted pattern that is whitelisted")
		} else if match == MatchNeither {
			t.Error("Neitherlisted pattern that is whitelisted")
		} else {
			t.Error("Returns more groups than expected or has an invalid structure")
		}
	}

	res = filter.Match("blacklisted")
	expected = map[string]FilterMatch{}
	if !maps.Equal(res, expected) {
		if match, ok := res["db.db"]; !ok {
			t.Error("Returns more groups than expected or has an invalid structure")
		} else if match == MatchNeither {
			t.Error("Neitherlisted pattern that is blacklisted")
		} else {
			t.Error("Whitelisted pattern that is blacklisted")
		}
	}
}

// Match text using filter group
func TestGroupMatching(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "filter.yml")

	data := []byte(`
storage_path: "./db"
groups:
  - filename: "both.db"
    whitelist:
      - "^whitelisted$"
      - "^both$"
    blacklist:
      - "^blacklisted$"
      - "^both$"
  - filename: "whitelist.db"
    whitelist:
      - "^whitelisted$"
      - "^both$"
  - filename: "blacklist.db"
    blacklist:
      - "^blacklisted$"
      - "^both$"
  - filename: "neither.db"
`)

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("Failed to write bytes into filter config file: %s", err)
	}

	expected := map[string]map[string]FilterMatch{
		"whitelisted": {
			"both.db":      MatchWhiteList,
			"whitelist.db": MatchWhiteList,
			"blacklist.db": MatchWhiteList,
			"neither.db":   MatchWhiteList,
		},
		"blacklisted": {
			"neither.db": MatchWhiteList,
		},
		"both": {
			"whitelist.db": MatchWhiteList,
			"neither.db":   MatchWhiteList,
		},
		"neither": {
			"both.db":      MatchNeither,
			"blacklist.db": MatchWhiteList,
			"neither.db":   MatchWhiteList,
		},
	}

	filter, err := New(configPath)
	if err != nil {
		t.Fatalf("Failed to create a filter: %s", err)
	}

	for k, v := range expected {
		match := filter.Match(k)
		if !maps.Equal(match, v) {
			matchS, _ := json.Marshal(match)
			vS, _ := json.Marshal(v)
			t.Errorf("Invalid match result for %s got `%s` and expected `%s`", k, matchS, vS)
		}
	}
}

// Match text using filter
func TestMatching(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "filter.yml")

	if err := os.WriteFile(configPath, filterConfigBytes, 0644); err != nil {
		t.Fatalf("Failed to write bytes into filter config file: %s", err)
	}

	filter, err := New(configPath)
	if err != nil {
		t.Fatalf("Failed to create a filter: %s", err)
	}

	for k, v := range matchResults {
		match := filter.Match(k)
		if !maps.Equal(match, v) {
			matchS, _ := json.Marshal(match)
			vS, _ := json.Marshal(v)
			t.Errorf("Invalid match result for %s got `%s` and expected `%s`", k, matchS, vS)
		}
	}
}
