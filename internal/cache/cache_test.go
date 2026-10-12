package cache

import (
	"crypto/rand"
	"errors"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

/* Utility Functions */

func randomPage() Page {
	return Page{
		URL:          rand.Text(),
		Method:       "GET",
		StatusCode:   200,
		Headers:      []byte(rand.Text()),
		Content:      []byte(rand.Text()),
		ETag:         "",
		LastModified: "",
	}
}

/* Page Tests */

// Check Page equality
func TestPageEquality(t *testing.T) {
	const checks = 10
	for range checks {
		page := randomPage()
		if !page.Equal(page) {
			t.Fatalf("Page isn't equal to itself")
		}
	}
}

/* Cache Tests */

// Create a new Cache
func TestCacheCreation(t *testing.T) {
	dir := t.TempDir()
	groupPathes := map[string]string{"db.db": filepath.Join(dir, "db.db")}

	cache, err := New(groupPathes)
	if err != nil {
		t.Fatalf("Failed to create a cache: %s", err)
	}
	defer cache.Close()
}

// Create a new Cache with a logger
func TestCacheCreationWithLogger(t *testing.T) {
	dir := t.TempDir()
	groupPathes := map[string]string{"db.db": filepath.Join(dir, "db.db")}

	var b strings.Builder
	handler := slog.NewTextHandler(&b, nil)
	logger := slog.New(handler)
	cache, err := New(groupPathes, WithLogger(logger))
	if err != nil {
		t.Fatalf("Failed to create a cache: %s", err)
	}
	defer cache.Close()

	if cache.logger == nil {
		t.Fatal("Non-nil logger is set to nil")
	}

	if cache.logger.Handler() != handler {
		t.Fatal("Non-nil logger's handler doesn't match given")
	}
}

// Create a new Cache with a nil logger
func TestCacheCreationWithNilLogger(t *testing.T) {
	dir := t.TempDir()
	groupPathes := map[string]string{"db.db": filepath.Join(dir, "db.db")}

	cache, err := New(groupPathes, WithLogger(nil))
	if err != nil {
		t.Fatalf("Failed to create a cache: %s", err)
	}
	defer cache.Close()

	if cache.logger == nil {
		t.Fatal("Nil logger is set to nil")
	}

	if cache.logger.Handler() != slog.DiscardHandler {
		t.Fatal("Nil logger's handler isn't set as a discard handler")
	}
}

// Create a new Cache with a metadata LRU cache
func TestCacheCreationWithMetaCache(t *testing.T) {
	dir := t.TempDir()
	groupPathes := map[string]string{"db.db": filepath.Join(dir, "db.db")}

	cache, err := New(groupPathes, WithMetaCache(1024))
	if err != nil {
		t.Fatalf("Failed to create a cache: %s", err)
	}
	defer cache.Close()

	if cache.metaCache == nil {
		t.Fatal("Didn't create a meta cache of valid size")
	}
}

// Create a new Cache with a metadata LRU cache
func TestCacheCreationWithMetaCacheOfInvalidSize(t *testing.T) {
	dir := t.TempDir()
	groupPathes := map[string]string{"db.db": filepath.Join(dir, "db.db")}

	cache, err := New(groupPathes, WithMetaCache(-1))
	if err != nil {
		t.Fatalf("Failed to create a cache: %s", err)
	}
	defer cache.Close()

	if cache.metaCache != nil {
		t.Fatal("Created a meta cache of invalid size")
	}
}

// Create a new Cache with an invalid db path
func TestCacheCreationWithInvalidDBPath(t *testing.T) {
	cache, err := New(map[string]string{"test": "///"})
	if err == nil {
		t.Fatalf("Created cache with an invalid db path")
	}
	defer cache.Close()

	if !errors.Is(err, ErrInvalidDBPath) {
		t.Fatalf("Returned an error not cause of invalid db path: %s", err)
	}
}

// Add a cacheable page to a databases
func TestAddGetCacheablePage(t *testing.T) {
	dir := t.TempDir()
	groupPathes := map[string]string{
		"a.db": filepath.Join(dir, "a.db"),
		"b.db": filepath.Join(dir, "b.db"),
	}
	matched := map[string]struct{}{
		"a.db": struct{}{},
		"b.db": struct{}{},
	}

	cache, err := New(groupPathes)
	if err != nil {
		t.Fatalf("Failed to create a cache: %s", err)
	}
	defer cache.Close()

	page := randomPage()
	err = cache.AddPage(matched, page)
	if err != nil {
		t.Fatalf("Failed to add a page to a cache: %s", err)
	}

	req, err := http.NewRequest(page.Method, page.URL, http.NoBody)
	if err != nil {
		t.Errorf("Failed to create a new request %s", err)
	}

	for group := range matched {
		pageCached, err := cache.GetPage(map[string]struct{}{
			group: struct{}{},
		}, req)
		if err != nil {
			t.Errorf("Failed to get a page from a cache for group %s, got `%s`", group, pageCached)
		}

		if !pageCached.Equal(page) {
			t.Errorf("Page isn't equal to the original one for group %s, got `%s`", group, pageCached)
		}
	}
}

// Add an uncacheable page to a databases
func TestAddUncacheablePage(t *testing.T) {
	dir := t.TempDir()
	groupPathes := map[string]string{
		"a.db": filepath.Join(dir, "a.db"),
		"b.db": filepath.Join(dir, "b.db"),
	}
	matched := map[string]struct{}{
		"a.db": struct{}{},
		"b.db": struct{}{},
	}

	cache, err := New(groupPathes)
	if err != nil {
		t.Fatalf("Failed to create a cache: %s", err)
	}
	defer cache.Close()

	page := randomPage()
	page.Method = "POST"
	err = cache.AddPage(matched, page)

	if err == nil {
		t.Fatal("Cached a page with unchacheable method")
	}

	if !errors.Is(err, ErrUncachableMethod) {
		t.Fatalf("Unexpected error was thrown during caching uncacheable page: %s", err)
	}
}

// Update a page in a database
func TestUpdateGetPage(t *testing.T) {
	dir := t.TempDir()
	groupPathes := map[string]string{
		"a.db": filepath.Join(dir, "a.db"),
		"b.db": filepath.Join(dir, "b.db"),
	}
	matched := map[string]struct{}{
		"a.db": struct{}{},
		"b.db": struct{}{},
	}

	cache, err := New(groupPathes)
	if err != nil {
		t.Fatalf("Failed to create a cache: %s", err)
	}
	defer cache.Close()

	pageOld := randomPage()
	pageOld.Content = []byte("Old")

	err = cache.AddPage(matched, pageOld)
	if err != nil {
		t.Fatalf("Failed to add a page to a cache: %s", err)
	}

	pageNew := pageOld
	pageNew.Content = []byte("New")

	err = cache.AddPage(matched, pageNew)
	if err != nil {
		t.Fatalf("Failed to update a page in a cache: %s", err)
	}

	req, err := http.NewRequest(pageNew.Method, pageNew.URL, http.NoBody)
	if err != nil {
		t.Errorf("Failed to create a new request %s", err)
	}

	for group := range matched {
		pageCached, err := cache.GetPage(map[string]struct{}{
			group: struct{}{},
		}, req)
		if err != nil {
			t.Errorf("Failed to get a page from a cache for a group %s", group)
		}

		if pageCached.Equal(pageOld) {
			t.Errorf("Page wasn't updated in cache for a group %s", group)
		}

		if !pageCached.Equal(pageNew) {
			t.Errorf("Page is neither old or new for a group %s", group)
		}
	}
}

// Delete a page from a database
func TestDeletePage(t *testing.T) {
	dir := t.TempDir()
	groupPathes := map[string]string{
		"a.db": filepath.Join(dir, "a.db"),
		"b.db": filepath.Join(dir, "b.db"),
	}
	matched := map[string]struct{}{
		"a.db": struct{}{},
		"b.db": struct{}{},
	}

	cache, err := New(groupPathes)
	if err != nil {
		t.Fatalf("Failed to create a cache: %s", err)
	}
	defer cache.Close()

	page := randomPage()
	err = cache.AddPage(matched, page)
	if err != nil {
		t.Fatalf("Failed to add a page to a cache: %s", err)
	}

	err = cache.DeletePage(matched, page.URL, page.Method)
	if err != nil {
		t.Fatalf("Failed to delete a page")
	}

	req, err := http.NewRequest(page.Method, page.URL, http.NoBody)
	if err != nil {
		t.Errorf("Failed to create a new request %s", err)
	}

	for group := range matched {
		_, err := cache.GetPage(map[string]struct{}{
			group: struct{}{},
		}, req)
		if err == nil {
			t.Errorf("Got a page from a cache after deletion for group %s", group)
		}
	}
}
