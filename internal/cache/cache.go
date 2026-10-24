// Package cache implements structure for managing web-pages and their contents
package cache

import (
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	lru "github.com/hashicorp/golang-lru/v2"
	_ "github.com/mattn/go-sqlite3"
)

// Cache used to manage web-pages and their contents in database
type cacheKey struct{ url, method string } // Type for uniquely representing a metaCache entry
type Cache struct {
	caches    map[string]*sql.DB         // Map with a group name as key and a database connection as a value
	metaCache *lru.Cache[cacheKey, Page] // LRU cache for storing page metadata
	logger    *slog.Logger               // Logger pointer
}

// New returns a pointer to a new cache object from given group pathes
func New(groupPathes map[string]string, opts ...Option) (*Cache, error) {
	cache := &Cache{
		caches: make(map[string]*sql.DB),
		logger: slog.New(slog.DiscardHandler),
	}

	// Apply options
	for _, opt := range opts {
		opt(cache)
	}

	// Initialize connection with database files
	for groupName, path := range groupPathes {
		// Skip existing groups
		if _, ok := cache.caches[groupName]; ok {
			continue
		}

		// Create group folders
		err := os.MkdirAll(filepath.Dir(path), 0755)
		if err != nil {
			err = fmt.Errorf("%w: %w", ErrFailedToCreateDirForCache, err)
			cache.logger.Error(err.Error())
			return nil, err
		}

		// Opening database file with additional options for optimization
		// journal_mode=WAL - enables WAL which increases writes performance
		// synchronous=NORMAL - recommended with WAL for the best durability & perfomance balance
		// busy_timeout=5000 - SQLITE_BUSY error is returned by sqlite after waiting for 5 seconds
		// cache_size=-4000 - sets cache size limit to 4 MiB
		db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000&_cache_size=-4000")
		if err != nil {
			err = fmt.Errorf("%w: %w", ErrUnknownDriver, err)
			cache.logger.Error(err.Error())
			return nil, err
		}
		cache.caches[groupName] = db

		// Execute initialization SQL script
		_, err = db.Exec(initStmt)
		if err != nil {
			if strings.HasPrefix(err.Error(), "unable to open database file") {
				err = ErrInvalidDBPath
			} else {
				err = fmt.Errorf("%w: %w", ErrInitScriptFailed, err)
			}
			cache.logger.Error(err.Error())
		}

		if err != nil {
			return nil, err
		}
	}

	return cache, nil
}

// AddPage inserts or updates a page across every matched group
// Only pages whose method and status are cacheable are accepted,
// others are rejected with ErrUncachableMethod / ErrUncachableStatusCode.
func (cache *Cache) AddPage(matched map[string]struct{}, page Page) error {
	// Return error on uncacheable method or status code
	if !slices.Contains(CacheableMethods, page.Method) {
		return fmt.Errorf("%w: %s", ErrUncachableMethod, page.Method)
	}
	if !slices.Contains(CacheableStatusCodes, page.StatusCode) {
		return fmt.Errorf("%w: %d", ErrUncachableStatusCode, page.StatusCode)
	}

	var lastErr error
	cached := false
	for group := range matched {
		// Get database of a group
		db, ok := cache.caches[group]
		if !ok {
			continue
		}

		// Insert a new page
		_, err := db.Exec(`
			INSERT INTO Pages(url, method, status_code, headers, content, etag, last_modified)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(url, method) DO UPDATE SET
				status_code   = excluded.status_code,
				headers       = excluded.headers,
				content       = excluded.content,
				etag          = excluded.etag,
				last_modified = excluded.last_modified,
				updated_at    = unixepoch()
		`,
			page.URL, page.Method, page.StatusCode,
			page.Headers, page.Content,
			page.ETag, page.LastModified,
		)

		if err != nil {
			lastErr = fmt.Errorf("%w (%s, %s): %w", ErrFailedToAddPage, page.Method, page.URL, err)
			cache.logger.Debug(lastErr.Error())
		} else {
			// Add to cache, if isn't cached yet
			if !cached && cache.metaCache != nil {
				cached = true
				key := cacheKey{page.URL, page.Method}
				pageCopy := page
				pageCopy.Content = nil
				cache.metaCache.Add(key, pageCopy)
			}
		}
		cache.logger.Debug(fmt.Sprintf("Added a page (%s, %s)", page.Method, page.URL))
	}
	return lastErr
}

// GetPage returns a cached Page matching the given request.
//
// Resolution order:
//  1. exact (url, method) match
//  2. for HEAD:     fall back to cached GET
//  3. for OPTIONS:  fall back to cached HEAD, then GET
//
// Conditional request headers (If-Match, If-None-Match, If-Modified-Since,
// If-Unmodified-Since) are evaluated against the cached entry and may
// downgrade the returned status code to 304 or 412.
//
// Returns sql.ErrNoRows if nothing matching is cached in any matched group.
func (cache *Cache) GetPage(matched map[string]struct{}, req *http.Request) (Page, error) {
	url := req.URL.String()
	method := req.Method

	// Conditional headers, pulled once.
	ifMatch := req.Header.Get("If-Match")
	ifNoneMatch := req.Header.Get("If-None-Match")
	ifModifiedSince := req.Header.Get("If-Modified-Since")
	ifUnmodifiedSince := req.Header.Get("If-Unmodified-Since")

	key := cacheKey{url, method}

	// Get page from a cache if it's cached
	if cache.metaCache != nil {
		if meta, ok := cache.metaCache.Get(key); ok {
			meta = applyConditional(meta, ifMatch, ifNoneMatch, ifModifiedSince, ifUnmodifiedSince)

			if method == http.MethodHead {
				meta.Content = nil
			}
			if meta.StatusCode == http.StatusNotModified ||
				meta.StatusCode == http.StatusPreconditionFailed ||
				method == http.MethodHead {
				return meta, nil
			}
		}
	}

	var lastErr error
	for group := range matched {
		// Get database of a group
		db, ok := cache.caches[group]
		if !ok {
			continue
		}

		// Exact method match
		page, err := getRow(db, url, method)
		if err == nil {
			page = applyConditional(page, ifMatch, ifNoneMatch, ifModifiedSince, ifUnmodifiedSince)
			return page, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			lastErr = fmt.Errorf("%w for (%s, %s): %w", ErrNoDirectMatchWasFound, method, url, err)
			cache.logger.Debug(lastErr.Error())
			continue
		}

		// Method fallbacks
		var fallbacks []string
		switch method {
		case http.MethodHead:
			fallbacks = []string{http.MethodGet}
		case http.MethodOptions:
			fallbacks = []string{http.MethodHead, http.MethodGet}
		}

		// Try to get a page using fallback methods
		for _, m := range fallbacks {
			page, err = getRow(db, url, m)
			if err == nil {
				page.Method = method
				if method == http.MethodHead {
					page.Content = nil
				}
				page = applyConditional(page, ifMatch, ifNoneMatch, ifModifiedSince, ifUnmodifiedSince)
				return page, nil
			}
			if !errors.Is(err, sql.ErrNoRows) {
				lastErr = fmt.Errorf("%w for (%s, %s): %w", ErrNoFallbackMatchWasFound, m, url, err)
				cache.logger.Debug(lastErr.Error())
			}
		}
	}

	// Return last error or sql.ErrNoRows if page wasn't found
	if lastErr != nil {
		lastErr = sql.ErrNoRows
	}
	return Page{URL: url, Method: method}, sql.ErrNoRows
}

// DeletePage removes the cached page for (url, method) from every matched group.
func (cache *Cache) DeletePage(matched map[string]struct{}, url, method string) error {
	cached := true
	var lastErr error
	for group := range matched {
		// Get database of a group
		db, ok := cache.caches[group]
		if !ok {
			continue
		}

		_, err := db.Exec(`
			DELETE FROM Pages
			WHERE url = ? AND method = ?
		`, url, method)

		if err != nil {
			lastErr = fmt.Errorf("%w (%s, %s): %w", ErrFailedToDeletePage, method, url, err)
			cache.logger.Debug(lastErr.Error())
		} else {
			// Remove cache entry
			if cached && cache.metaCache != nil {
				cached = false
				cache.metaCache.Remove(cacheKey{url, method})
			}
		}
		cache.logger.Debug(fmt.Sprintf("Deleted page (%s, %s)", method, url))
	}
	return lastErr
}

// Close closes cache's connection to the database
func (cache *Cache) Close() error {
	// If no cache or cache groups than leave
	if cache == nil || cache.caches == nil {
		return nil
	}

	var lastErr error
	for group, groupCache := range cache.caches {
		groupCache.Exec("PRAGMA optimize;") // updates statistics for the planner
		err := groupCache.Close()
		if err != nil {
			lastErr = fmt.Errorf("%w %s: %w", ErrFailedToCloseDatabase, group, err)
			cache.logger.Error(lastErr.Error())
		} else {
			cache.logger.Debug(fmt.Sprintf("Successfuly closed the database connection to group %s", group))
		}
	}
	cache.metaCache = nil
	return lastErr
}
