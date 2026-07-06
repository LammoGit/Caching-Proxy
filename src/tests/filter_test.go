package test

import (
	"os"
	"testing"
	"strings"
	"fmt"

	f "caching-proxy/filter"
)

var (
	whitePattern = "a.*\n[b-d].*"
	blackPattern = "d.*\n[e-g].*"
	whiteList = []string {
		"a",
		"b",
		"c",
	}
	blackList = []string {
		"e",
		"f",
		"g",
	}
	grayList = []string {
		"d",
	}
	nonmatchList = []string {
		"z",
		"x",
		"u",
	}
)

func getFilterFilePathes(t *testing.T) (whitePath, blackPath string) {
	t.Helper()

	// 
	whiteFile, err := os.CreateTemp("", "test-whitelist-*.txt")
	if err != nil {
		t.Fatalf("Couldn't create a temporary whitelist file")
	}
	whitePath = whiteFile.Name()
	whiteFile.WriteString(whitePattern)
	whiteFile.Close()

	// 
	blackFile, err := os.CreateTemp("", "test-blacklist-*.txt")
	if err != nil {
		t.Fatalf("Couldn't create a temporary blacklist file")
	}
	blackPath = blackFile.Name()
	blackFile.WriteString(blackPattern)
	blackFile.Close()
	return
}

// Create New Filter
func TestCreateFilter(t *testing.T) {
	whitePath, blackPath := getFilterFilePathes(t)
	defer os.Remove(whitePath)
	defer os.Remove(blackPath)
	
	_, err := f.New(whitePath, blackPath)
	if err != nil {
		t.Fatalf("Failed to create a filter: %s", err)
	}
}

// Match Whitelisted
func TestMatchingWhitelisted(t *testing.T) {
	whitePath, blackPath := getFilterFilePathes(t)
	defer os.Remove(whitePath)
	defer os.Remove(blackPath)

	filter, err := f.New(whitePath, blackPath)
	if err != nil {
		t.Fatalf("Failed to create a filter: %s", err)
	}

	res := false
	var builder strings.Builder
	builder.WriteString("Didn't match whitelisted patterns\n")
	for _, value := range whiteList {
		if !filter.Match(value) {
			builder.WriteString(fmt.Sprintf("%s\n", value))
			res = true
		}
	}
	if res {
		t.Fatalf("%s", builder.String())
	}
}

// Match Blacklisted
func TestMatchingBlacklisted(t *testing.T) {
	whitePath, blackPath := getFilterFilePathes(t)
	defer os.Remove(whitePath)
	defer os.Remove(blackPath)

	filter, err := f.New(whitePath, blackPath)
	if err != nil {
		t.Fatalf("Failed to create a filter: %s", err)
	}

	res := false
	var builder strings.Builder
	builder.WriteString("Matched blacklisted patterns\n")
	for _, value := range blackList {
		if filter.Match(value) {
			builder.WriteString(fmt.Sprintf("%s\n", value))
			res = true
		}
	}
	if res {
		t.Fatalf("%s", builder.String())
	}
}

// Match Graylisted
func TestMatchingGraylisted(t *testing.T) {
	whitePath, blackPath := getFilterFilePathes(t)
	defer os.Remove(whitePath)
	defer os.Remove(blackPath)

	filter, err := f.New(whitePath, blackPath)
	if err != nil {
		t.Fatalf("Failed to create a filter: %s", err)
	}

	res := false
	var builder strings.Builder
	builder.WriteString("Matched graylisted patterns\n")
	for _, value := range grayList {
		if filter.Match(value) {
			builder.WriteString(fmt.Sprintf("%s\n", value))
			res = true
		}
	}
	if res {
		t.Fatalf("%s", builder.String())
	}
}

// Match non-matching string
func TestNonMatching(t *testing.T) {
	whitePath, blackPath := getFilterFilePathes(t)
	defer os.Remove(whitePath)
	defer os.Remove(blackPath)

	filter, err := f.New(whitePath, blackPath)
	if err != nil {
		t.Fatalf("Failed to create a filter: %s", err)
	}

	res := false
	var builder strings.Builder
	builder.WriteString("Matched non-matching patterns\n")
	for _, value := range nonmatchList {
		if filter.Match(value) {
			builder.WriteString(fmt.Sprintf("%s\n", value))
			res = true
		}
	}
	if res {
		t.Fatalf("%s", builder.String())
	}
}
