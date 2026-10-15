package filter

import (
	"errors"
)

// Filter Errors
var (
	ErrCantOpenFilterConfig = errors.New("Couldn't open the filter config file")
	ErrFailedToParseConfig  = errors.New("Failed to parse filter's YAML config")
)

// FilterMatch represents different types of matching
type FilterMatch int

const (
	MatchBlackList FilterMatch = iota
	MatchWhiteList
	MatchNeither
)

// String implements conversion from FilterMatch to a string
func (fm FilterMatch) String() string {
	switch fm {
	case MatchBlackList:
		return "blacklisted"
	case MatchWhiteList:
		return "whitelisted"
	case MatchNeither:
		return "neither"
	default:
		return "unknown"
	}
}
