// Package filter implements structure for filtering strings using white and black lists
package filter

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"
)

// Filter is a structure implementing string filtering based on provided white and black list of regex patterns
type Filter struct {
	WhiteRegex    *regexp.Regexp
	BlackRegex    *regexp.Regexp
	WhitePatterns []string
	BlackPatterns []string
	logger        *slog.Logger
}

/* Filter Options */

// Option type for functions that are run at start of creation on a filter
type Option func(*Filter)

// WithLogger is an Option that attaches given logger to a filter
// if logger pointer is nil, then log messages are discarded
func WithLogger(logger *slog.Logger) Option {
	return func(filter *Filter) {
		if logger == nil {
			filter.logger = slog.New(slog.DiscardHandler)
		} else {
			filter.logger = logger
		}
	}
}

/* Filter Methods */

// New is a function that returns a pointer to a new filter
// whitePath gives a path to the file with whitelisted regex patterns separated by a linefeed
// blackPath gives a path to the file with blacklisted regex patterns separated by a linefeed
// opts Options that are ran at beggining of creation
func New(whitePath, blackPath string, opts ...Option) (*Filter, error) {
	filter := &Filter{logger: slog.New(slog.DiscardHandler)}

	// Run Option functions
	for _, opt := range opts {
		opt(filter)
	}

	// Open whitelist file
	wlFile, err := os.Open(whitePath)
	if err != nil {
		filter.logger.Debug(fmt.Sprintf("Couldn't open the whitelist file at path: %s", whitePath))
		return filter, err
	} else {
		filter.logger.Debug(fmt.Sprintf("Successfully opened the whitelist file at path: %s", whitePath))
	}
	defer wlFile.Close()

	// Open blacklist file
	blFile, err := os.Open(blackPath)
	if err != nil {
		filter.logger.Debug(fmt.Sprintf("Couldn't open the blacklist file at path: %s", blackPath))
		return filter, err
	} else {
		filter.logger.Debug(fmt.Sprintf("Successfully opened the blacklist file at path: %s", blackPath))
	}
	defer blFile.Close()

	// Initialize scanners for whitelist and blacklist files
	wlScanner := bufio.NewScanner(wlFile)
	blScanner := bufio.NewScanner(blFile)

	// Compile regex pattern on every line and add valid patterns to a list of patterns
	var line string
	for wlScanner.Scan() {
		line = wlScanner.Text()
		_, err = regexp.Compile(line)
		if err != nil {
			filter.logger.Warn(fmt.Sprintf("Whitelist pattern is invalid, therefore removed: %s\n", line))
			continue
		}
		filter.WhitePatterns = append(filter.WhitePatterns, line)
	}

	for blScanner.Scan() {
		line = blScanner.Text()
		_, err = regexp.Compile(line)
		if err != nil {
			filter.logger.Warn(fmt.Sprintf("Blacklist pattern is invalid, therefore removed: %s\n", line))
			continue
		}
		filter.BlackPatterns = append(filter.BlackPatterns, line)
	}

	// If no valid whitelisted patterns, then set regex pointer to nil,
	// otherwise compile patterns joined with `|` separator
	if len(filter.WhitePatterns) > 0 {
		filter.WhiteRegex = regexp.MustCompile(strings.Join(filter.WhitePatterns, "|"))
	} else {
		filter.WhiteRegex = nil
	}

	// If no valid blacklisted patterns, then set regex pointer to nil,
	// otherwise compile patterns joined with `|` separator
	if len(filter.BlackPatterns) > 0 {
		filter.BlackRegex = regexp.MustCompile(strings.Join(filter.BlackPatterns, "|"))
	} else {
		filter.BlackRegex = nil
	}

	return filter, err
}

// Match returns true if string doesn't match any of blacklisted patterns and matches any whitelisted patterns,
// otherwise false is returned
func (filter *Filter) Match(text string) bool {
	return (filter.BlackRegex == nil || !filter.BlackRegex.MatchString(text)) &&
		filter.WhiteRegex != nil &&
		filter.WhiteRegex.MatchString(text)
}
