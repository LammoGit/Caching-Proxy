// Package cache implements structure for managing web-pages and their contents
package cache

import (
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed init.sql
var initStmt string

/* Errors */

var (
	ErrUnknownDriver    = errors.New("unknown SQL driver")
	ErrInvalidDBPath    = errors.New("database path is invalid")
	ErrInitScriptFailed = errors.New("initialization script failed")
)

/* Types */

// Page holds web-page's contents
type Page struct {
	Url     string
	Method  string
	Headers []byte
	Content []byte
}

// Cache used to manage web-pages and their contents in database
type Cache struct {
	path   string
	db     *sql.DB
	logger *slog.Logger
}

/* Page Methods */

// Equal method that returns true if two pages are equal
func (page Page) Equal(other Page) bool {
	return page.Url == other.Url &&
		page.Method == other.Method &&
		slices.Equal(page.Headers, other.Headers) &&
		slices.Equal(page.Content, other.Content)
}

/* Cache Options */

type Option func(*Cache)

func WithLogger(logger *slog.Logger) Option {
	return func(cache *Cache) {
		if logger == nil {
			cache.logger = slog.New(slog.DiscardHandler)
		} else {
			cache.logger = logger
		}
	}
}

/* Cache Methods */

// New returns a pointer to a new cache from the given path to a database file
func New(path string, opts ...Option) (*Cache, error) {
	cache := &Cache{
		path:   path,
		logger: slog.New(slog.DiscardHandler),
	}

	// Apply options
	for _, opt := range opts {
		opt(cache)
	}

	// Initialize connection with database file
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		err = fmt.Errorf("%w: %s", ErrUnknownDriver, err)
		cache.logger.Error("", slog.String("error", err.Error()))
		return nil, err
	}
	cache.db = db

	// Execute initialization SQL script
	_, err = cache.db.Exec(initStmt)
	if err != nil {
		if err.Error() == "unable to open database file: The specified path is invalid." {
			err = ErrInvalidDBPath
		} else {
			err = fmt.Errorf("%w: %s", ErrInitScriptFailed, err)
		}
		cache.logger.Error("", slog.String("error", err.Error()))
	}

	return cache, err
}

// AddPage inserts page object data into database
// if URL and method combination is already taken, then rewrites
func (cache *Cache) AddPage(page Page) (err error) {
	// Inserts page fields into table
	// if URL and method are already taken replace headers and content
	_, err = cache.db.Exec(`
        INSERT INTO Pages(url, method, headers, content)
        VALUES (?, ?, ?, ?)
        ON CONFLICT(url, method) DO UPDATE SET
            headers = excluded.headers,
            content = excluded.content;
    `, page.Url, page.Method, page.Headers, page.Content)
	if err != nil {
		cache.logger.Debug(fmt.Sprintf("Failed to insert/update page: %s %s", page.Method, page.Url))
	} else {
		cache.logger.Debug(fmt.Sprintf("Inserted/Updated page: %s %s", page.Method, page.Url))
	}
	return
}

// GetPage returns page object with data from database searching by URL and method
func (cache *Cache) GetPage(url, method string) (page Page, err error) {
	page.Url = url
	page.Method = method

	// Assigns page headers and page content for the given URL and method
	err = cache.db.QueryRow(`
        SELECT
            headers,
            content
        FROM Pages
        WHERE url = ? AND method = ?
    `, url, method).Scan(&page.Headers, &page.Content)
	if err != nil {
		cache.logger.Debug(fmt.Sprintf("Failed to get the page: %s %s", method, url))
	} else {
		cache.logger.Debug(fmt.Sprintf("Successfully got the page: %s %s", method, url))
	}
	return
}

// DeletePage deletes a page with the given URL and method
func (cache *Cache) DeletePage(url, method string) (err error) {
	_, err = cache.db.Exec(`
        DELETE FROM Pages
        WHERE url = ? AND method = ?
    `, url, method)
	if err != nil {
		cache.logger.Debug(fmt.Sprintf("Failed to delete the page: %s %s", method, url))
	} else {
		cache.logger.Debug(fmt.Sprintf("Successfully deleted the page: %s %s", method, url))
	}
	return
}

// Close closes cache's connection to the database
func (cache *Cache) Close() error {
	cache.logger.Debug("Closing DB connection")
	return cache.db.Close()
}
