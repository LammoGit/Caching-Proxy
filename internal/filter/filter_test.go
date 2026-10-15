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

// Check FilterMatch to string conversion
func TestFilterMatchString(t *testing.T) {
	for k, v := range map[FilterMatch]string{
		MatchBlackList: "blacklisted",
		MatchWhiteList: "whitelisted",
		MatchNeither:   "neither",
		-1:             "unknown",
	} {
		s := k.String()
		if s != v {
			t.Errorf("Invalid result of conversion, got `%s`, expected `%s`", s, v)
		}
	}
}

// Create a new filter
func TestCreateFilter(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "filter.yml")

	if err := os.WriteFile(configPath, filterConfigBytes, 0644); err != nil {
		t.Fatalf("Failed to write bytes into filter config file: %s", err)
	}

	_, err := New(configPath)

	if errors.Is(err, ErrCantOpenFilterConfig) {
		t.Fatalf("Failed to open a config file: %s", err)
	}

	if err != nil {
		t.Fatalf("Failed to create a filter: %s", err)
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

// Create a new filter with an invalid config
func TestCreateFilterWithInvalidConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "filter.yml")

	if err := os.WriteFile(configPath, []byte("~!a"), 0644); err != nil {
		t.Fatalf("Failed to write bytes into filter config file: %s", err)
	}

	_, err := New(configPath)
	if err == nil {
		t.Fatal("Created a filter with an invalid config")
	}

	if !errors.Is(err, ErrFailedToParseConfig) {
		t.Fatalf("Error is not for invalid YAML: %s", err)
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

// // Create a new filter with invalid whitelist patterns
// func TestCreateFilterWithInvalidWhitelistPatterns(t *testing.T) {
// 	dir := t.TempDir()
// 	configPath := filepath.Join(dir, "filter.yml")
//
// 	data := []byte(`
// storage_path: "./db"
// groups:
//   - filename: "db.db"
//     whitelist:
//       - "^*+?|"
// `)
// 	if err := os.WriteFile(configPath, data, 0644); err != nil {
// 		t.Fatalf("Failed to write bytes into filter config file: %s", err)
// 	}
//
// 	filter, err := New(configPath)
// 	if err != nil {
// 		t.Fatalf("Failed to create a filter: %s", err)
// 	}
//
// 	group, ok := filter.Groups["db.db"]
// 	if !ok {
// 		t.Fatal("Group wasn't created")
// 	}
//
// 	if group.WhiteRegex != nil {
// 		t.Fatalf("Regex pattern is compiled from invalid patterns")
// 	}
// }
//
// // Create a new filter with invalid blacklist patterns
// func TestCreateFilterWithInvalidBlacklistPatterns(t *testing.T) {
// 	dir := t.TempDir()
// 	configPath := filepath.Join(dir, "filter.yml")
//
// 	data := []byte(`
// storage_path: "./db"
// groups:
//   - filename: "db.db"
//     blacklist:
//       - "^*+?|"
// `)
// 	if err := os.WriteFile(configPath, data, 0644); err != nil {
// 		t.Fatalf("Failed to write bytes into filter config file: %s", err)
// 	}
//
// 	filter, err := New(configPath)
// 	if err != nil {
// 		t.Fatalf("Failed to create a filter: %s", err)
// 	}
//
// 	group, ok := filter.Groups["db.db"]
// 	if !ok {
// 		t.Fatal("Group wasn't created")
// 	}
//
// 	if group.BlackRegex != nil {
// 		t.Fatalf("Regex pattern is compiled from invalid patterns")
// 	}
// }

// Create a new filtering group with invalid and valid patterns
func TestCreateFilterGroupWithInvalidAndValidPatterns(t *testing.T) {
	group, err := NewFilterGroup(FilterGroupConfig{
		FileName:  "db.db",
		WhiteList: []string{"^*+?|", "whitelisted", "both"},
		BlackList: []string{"^*+?|", "blacklisted", "both"},
	})
	if err != nil {
		t.Fatalf("Failed to create a filtering group: %s", err)
	}

	if group.BlackRegex == nil || group.WhiteRegex == nil {
		t.Fatalf("Regex pattern isn't compiled from partially invalid patterns")
	}

	for k, v := range map[string]FilterMatch{
		"whitelisted": MatchWhiteList,
		"blacklisted": MatchBlackList,
		"both":        MatchBlackList,
		"neither":     MatchNeither,
	} {
		match := group.Match(k)
		if match != v {
			t.Errorf("Invalid match result for %s got `%s` and expected `%s`", k, match, v)
		}
	}
}

// Match text using filtering groups
func TestGroupMatching(t *testing.T) {
	// Function for checking whether groups match results equal expected
	check := func(group *FilterGroup, expected map[string]FilterMatch) {
		for k, v := range expected {
			match := group.Match(k)
			if match != v {
				t.Errorf("Invalid match result for %s in %s got `%s` and expected `%s`", k, group.FileName, match, v)
			}
		}
	}

	// Filter group with both lists set
	both, err := NewFilterGroup(FilterGroupConfig{
		FileName:  ".db",
		WhiteList: []string{"whitelisted", "both"},
		BlackList: []string{"blacklisted", "both"},
	})
	if err != nil {
		t.Errorf("Failed to create a filter group: %s", err)
	} else {
		check(both, map[string]FilterMatch{
			"whitelisted": MatchWhiteList,
			"blacklisted": MatchBlackList,
			"both":        MatchBlackList,
			"neither":     MatchNeither,
		})
	}

	// Filter group with only whitelist set
	whitelist, err := NewFilterGroup(FilterGroupConfig{
		FileName:  "whitelist.db",
		WhiteList: []string{"whitelisted", "both"},
	})
	if err != nil {
		t.Errorf("Failed to create a filter group: %s", err)
	} else {
		check(whitelist, map[string]FilterMatch{
			"whitelisted": MatchWhiteList,
			"blacklisted": MatchBlackList,
			"both":        MatchWhiteList,
			"neither":     MatchBlackList,
		})
	}

	// Filter group with only blacklist set
	blacklist, err := NewFilterGroup(FilterGroupConfig{
		FileName:  "blacklist.db",
		BlackList: []string{"blacklisted", "both"},
	})
	if err != nil {
		t.Errorf("Failed to create a filter group: %s", err)
	} else {
		check(blacklist, map[string]FilterMatch{
			"whitelisted": MatchWhiteList,
			"blacklisted": MatchBlackList,
			"both":        MatchBlackList,
			"neither":     MatchWhiteList,
		})
	}

	// Filter group with neither list set
	neither, err := NewFilterGroup(FilterGroupConfig{
		FileName: "neither.db",
	})
	if err != nil {
		t.Errorf("Failed to create a filter group: %s", err)
	} else {
		check(neither, map[string]FilterMatch{
			"whitelisted": MatchWhiteList,
			"blacklisted": MatchWhiteList,
			"both":        MatchWhiteList,
			"neither":     MatchWhiteList,
		})
	}
}

// Match text using Filter
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

	// Check whether returned match results of
	// the filter object are equal to the expected
	for k, v := range matchResults {
		match := filter.Match(k)
		if !maps.Equal(match, v) {
			matchS, _ := json.Marshal(match)
			vS, _ := json.Marshal(v)
			t.Errorf("Invalid match result for %s got `%s` and expected `%s`", k, matchS, vS)
		}
	}
}
