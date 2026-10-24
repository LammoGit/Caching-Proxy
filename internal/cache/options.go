package cache

import (
	"fmt"
	"log/slog"

	lru "github.com/hashicorp/golang-lru/v2"
)

// Option functions that are ran as Cache object is initialized
type Option func(*Cache)

// WithLogger Option sets the given logger pointer as a logger for the Cache
// if the given logger pointer is nil, then Cache's logger is set to discard log messages
func WithLogger(logger *slog.Logger) Option {
	return func(cache *Cache) {
		if logger == nil {
			cache.logger = slog.New(slog.DiscardHandler)
		} else {
			cache.logger = logger
		}
	}
}

// WithMetaCache Option sets the given size for the Cache's LRU cache pointer
// if the given size is 0, then Cache's LRU cache is set to nil
func WithMetaCache(size int) Option {
	return func(cache *Cache) {
		if size < 1 {
			cache.metaCache = nil
			return
		}
		c, err := lru.New[cacheKey, Page](size)
		if err != nil {
			cache.logger.Error(fmt.Sprintf("Failed to create meta cache: %s", err))
			return
		}
		cache.metaCache = c
	}
}
