package cache

import (
	"crypto/rand"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

/* Utility Functions */

func randomPage() Page {
	return Page{
		Url:     rand.Text(),
		Method:  rand.Text(),
		Headers: []byte(rand.Text()),
		Content: []byte(rand.Text()),
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
	dbPath := filepath.Join(dir, "db.db")

	cache, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create a cache: %s", err)
	}
	defer cache.Close()
}

// Create a new Cache with a logger
func TestCacheCreationWithLogger(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "db.db")

	var b strings.Builder
	handler := slog.NewTextHandler(&b, nil)
	logger := slog.New(handler)
	cache, err := New(dbPath, WithLogger(logger))
	if err != nil {
		t.Fatalf("Failed to create a cache: %s", err)
	}
	defer cache.Close()
}

// Create a new Cache with a nil logger
func TestCacheCreationWithNilLogger(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "db.db")

	cache, err := New(dbPath, WithLogger(nil))
	if err != nil {
		t.Fatalf("Failed to create a cache: %s", err)
	}
	defer cache.Close()
}

// Create a new Cache with an invalid db path
func TestCacheCreationWithInvalidDBPath(t *testing.T) {
	cache, err := New("///")
	if err == nil {
		t.Fatalf("Created cache with an invalid db path")
	}
	defer cache.Close()

	if !errors.Is(err, ErrInvalidDBPath) {
		t.Fatalf("Returned an error not cause of invalid db path: %s", err)
	}
}

// Add a page to a database
func TestAddPage(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "db.db")

	cache, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create a cache: %s", err)
	}
	defer cache.Close()

	err = cache.AddPage(randomPage())
	if err != nil {
		t.Fatalf("Failed to add a page to a cache: %s", err)
	}
}

// Update a page in a database
func TestUpdatePage(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "db.db")

	cache, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create a cache: %s", err)
	}
	defer cache.Close()

	pageOld := randomPage()
	pageOld.Content = []byte("Old")

	err = cache.AddPage(pageOld)
	if err != nil {
		t.Fatalf("Failed to add a page to a cache: %s", err)
	}

	pageNew := pageOld
	pageNew.Content = []byte("New")

	err = cache.AddPage(pageNew)
	if err != nil {
		t.Fatalf("Failed to update a page in a cache: %s", err)
	}
}

// Get a page from a database
func TestGetPage(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "db.db")

	cache, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create a cache: %s", err)
	}
	defer cache.Close()

	page := randomPage()
	err = cache.AddPage(page)
	if err != nil {
		t.Fatalf("Failed to add a page to a cache: %s", err)
	}

	pageCached, err := cache.GetPage(page.Url, page.Method)
	if err != nil {
		t.Fatalf("Failed to get a page from a cache: %s", pageCached)
	}

	if !page.Equal(pageCached) {
		t.Fatalf("Loaded page isn't equal to saved")
	}
}

// Get an updated page from a database
func TestGetUpdatedPage(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "db.db")

	cache, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create a cache: %s", err)
	}
	defer cache.Close()

	pageOld := randomPage()
	pageOld.Content = []byte("Old")

	err = cache.AddPage(pageOld)
	if err != nil {
		t.Fatalf("Failed to add a page to a cache: %s", err)
	}

	pageNew := pageOld
	pageNew.Content = []byte("New")

	err = cache.AddPage(pageNew)
	if err != nil {
		t.Fatalf("Failed to add a page to a cache: %s", err)
	}

	pageCached, err := cache.GetPage(pageOld.Url, pageOld.Method)
	if err != nil {
		t.Fatalf("Failed to get a page from a cache: %s", pageCached)
	}

	if pageCached.Equal(pageOld) {
		t.Fatal("Page wasn't updated in cache")
	}

	if !pageCached.Equal(pageNew) {
		t.Fatalf("Page is neither old or new")
	}
}

// Delete a page from a database
func TestDeletePage(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "db.db")

	cache, err := New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create a cache: %s", err)
	}
	defer cache.Close()

	page := randomPage()
	err = cache.AddPage(page)
	if err != nil {
		t.Fatalf("Failed to add a page to a cache: %s", err)
	}

	err = cache.DeletePage(page.Url, page.Method)
	if err != nil {
		t.Fatalf("Failed to delete a page")
	}
}
