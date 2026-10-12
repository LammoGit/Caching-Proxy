// Package filter implements structure for filtering strings using white and black lists
package filter

import (
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"

	"github.com/goccy/go-yaml"
)

// FilterMatch represents different types of matching
type FilterMatch int

const (
	MatchBlackList FilterMatch = iota
	MatchWhiteList
	MatchNeither
)

// FilterConfig type used for storing and parsing filter config files
type FilterConfig struct {
	Groups []struct {
		FileName  string   `json:"filename" yaml:"filename"`
		WhiteList []string `json:"whitelist" yaml:"whitelist"`
		BlackList []string `json:"blacklist" yaml:"blacklist"`
	} `json:"groups" yaml:"groups"`
}

// FilterGroup type representing a filtering group inside of a filter
type FilterGroup struct {
	FileName      string
	WhiteRegex    *regexp.Regexp
	BlackRegex    *regexp.Regexp
	WhiteListSet  bool
	BlackListSet  bool
	WhitePatterns []string
	BlackPatterns []string
}

// Filter is a structure implementing string filtering based on provided white and black list of regex patterns
type Filter struct {
	Groups map[string]FilterGroup
	logger *slog.Logger
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
func New(configPath string, opts ...Option) (*Filter, error) {
	var err error

	filter := &Filter{
		Groups: make(map[string]FilterGroup),
		logger: slog.New(slog.DiscardHandler),
	}

	// Run Option functions
	for _, opt := range opts {
		opt(filter)
	}

	// Read bytes from the config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		filter.logger.Debug(fmt.Sprintf("Couldn't open the filter config file at path: %s", configPath))
		return nil, err
	}

	var config FilterConfig
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	for _, groupConfig := range config.Groups {
		var group FilterGroup

		group.FileName = groupConfig.FileName

		group.BlackListSet = len(groupConfig.BlackList) != 0
		group.WhiteListSet = len(groupConfig.WhiteList) != 0

		for _, pattern := range groupConfig.BlackList {
			_, err := regexp.Compile(pattern)
			if err != nil {
				continue
			}
			group.BlackPatterns = append(group.BlackPatterns, pattern)
		}

		for _, pattern := range groupConfig.WhiteList {
			_, err := regexp.Compile(pattern)
			if err != nil {
				continue
			}
			group.WhitePatterns = append(group.WhitePatterns, pattern)
		}

		// If no valid whitelisted patterns, then set regex pointer to nil,
		// otherwise compile patterns joined with `|` separator
		if len(group.WhitePatterns) > 0 {
			group.WhiteRegex = regexp.MustCompile(strings.Join(group.WhitePatterns, "|"))
		} else {
			group.WhiteRegex = nil
		}

		// If no valid blacklisted patterns, then set regex pointer to nil,
		// otherwise compile patterns joined with `|` separator
		if len(group.BlackPatterns) > 0 {
			group.BlackRegex = regexp.MustCompile(strings.Join(group.BlackPatterns, "|"))
		} else {
			group.BlackRegex = nil
		}

		// Append group to the slice of groups
		filter.Groups[groupConfig.FileName] = group
	}

	return filter, nil
}

// Match returns matching type of the given string depending on whether lists were specified
// returns direct matching type if both white and black lists were specified
// blacklists non-whitelisted if only white list was specified
// whitelists non-blacklisted if only black list was specified
// whitelists all if neither list was specified
func (group *FilterGroup) Match(text string) FilterMatch {
	if !group.BlackListSet && !group.WhiteListSet {
		return MatchWhiteList
	}

	if group.BlackRegex != nil && group.BlackRegex.MatchString(text) {
		return MatchBlackList
	}

	if group.WhiteRegex != nil && group.WhiteRegex.MatchString(text) {
		return MatchWhiteList
	}

	switch false {
	case group.BlackListSet:
		return MatchBlackList
	case group.WhiteListSet:
		return MatchWhiteList
	default:
		return MatchNeither
	}
}

// Match returns a map with non-blacklisted groups' names as keys and match type as value
func (filter *Filter) Match(text string) map[string]FilterMatch {
	res := make(map[string]FilterMatch)
	for groupName, group := range filter.Groups {
		if matched := group.Match(text); matched != MatchBlackList {
			res[groupName] = matched
		}
	}
	return res
}
