package filter

import "log/slog"

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
