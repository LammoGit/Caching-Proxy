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

// FilterGroupConfig type used fro storing and parsing
// config of individual filter groups in a filter config
type FilterGroupConfig struct {
	FileName  string   `json:"filename" yaml:"filename"`
	WhiteList []string `json:"whitelist" yaml:"whitelist"`
	BlackList []string `json:"blacklist" yaml:"blacklist"`
}

// FilterConfig type used for storing and parsing filter config files
type FilterConfig struct {
	Groups []FilterGroupConfig `json:"groups" yaml:"groups"`
}

// FilterGroup type representing a filtering group inside of a filter
type FilterGroup struct {
	FileName      string         // Name of the group's database file
	WhiteRegex    *regexp.Regexp // Whitelist regex engine's pointer
	BlackRegex    *regexp.Regexp // Blacklist regex engine's pointer
	WhiteListSet  bool           // Indicator of whether whitelist is empty
	BlackListSet  bool           // Indicator of whether blacklist is empty
	WhitePatterns []string       // Whitelist patterns
	BlackPatterns []string       // Blacklist pattern
}

// Filter is a structure implementing string filtering based on provided white and black list of regex patterns
type Filter struct {
	Groups map[string]*FilterGroup
	logger *slog.Logger
}

// NewFilterGroup creates a new FilterGroup object using given FilterGroupConfig
// and returns its pointer
func NewFilterGroup(config FilterGroupConfig) (*FilterGroup, error) {
	var group FilterGroup

	group.FileName = config.FileName
	group.BlackListSet = len(config.BlackList) != 0
	group.WhiteListSet = len(config.WhiteList) != 0

	// Add valid patterns to the blacklisted patterns list
	for _, pattern := range config.BlackList {
		// Don't add pattern if it doesn't compile
		_, err := regexp.Compile(pattern)
		if err != nil {
			continue
		}
		group.BlackPatterns = append(group.BlackPatterns, pattern)
	}

	// Add valid patterns to the whitelisted patterns list
	for _, pattern := range config.WhiteList {
		// Don't add pattern if it doesn't compile
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

	return &group, nil
}

// New is a function that returns a pointer to a new filter
// configPath a path to the file with configuration of the filter in YAML encoding
// opts Options that are ran at beggining of creation on the filter pointer
func New(configPath string, opts ...Option) (*Filter, error) {
	filter := &Filter{
		Groups: make(map[string]*FilterGroup),
		logger: slog.New(slog.DiscardHandler),
	}

	// Apply passed Options
	for _, opt := range opts {
		opt(filter)
	}

	// Read bytes from the config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		err = fmt.Errorf("%w: %w", ErrCantOpenFilterConfig, err)
		filter.logger.Error(err.Error())
		return nil, err
	}

	// Parse read bytes from YAMl to the FilterConfig structure
	var config FilterConfig
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		err = fmt.Errorf("%w: %w", ErrFailedToParseConfig, err)
		filter.logger.Error(err.Error())
		return nil, err
	}

	// Create a filter group for each group config
	for _, groupConfig := range config.Groups {
		// Create a new filtering group
		group, err := NewFilterGroup(groupConfig)
		if err != nil {
			filter.logger.Debug(err.Error())
			continue
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
	// White list if neither list is set
	if !group.BlackListSet && !group.WhiteListSet {
		return MatchWhiteList
	}

	// If blacklist is set and text matches one of the patterns, then blacklist
	if group.BlackRegex != nil && group.BlackRegex.MatchString(text) {
		return MatchBlackList
	}

	// If whitelist is set and text matches one of the patterns, then whitelist
	if group.WhiteRegex != nil && group.WhiteRegex.MatchString(text) {
		return MatchWhiteList
	}

	// If one of the list isn't set, then return its match type
	// If both lists are set, then return neither match type
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
		// Add a group with its match type to map if not blacklisted
		if matched := group.Match(text); matched != MatchBlackList {
			res[groupName] = matched
		}
	}
	return res
}
