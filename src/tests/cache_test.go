package test

import (
	"os"
	"testing"
	"slices"
	"database/sql"

	_ "github.com/mattn/go-sqlite3"

	c "caching-proxy/cache"
)

func equalPages(p1, p2 c.Page) bool {
	return  p1.Url == p2.Url &&
			p1.Method == p2.Method &&
			slices.Equal(p1.Headers, p2.Headers) &&
			slices.Equal(p1.Content, p2.Content)
}

func setupCache(t *testing.T) *c.Cache {
	t.Helper()

	// Opening DB in RAM
	cache, err := c.New("file:testdb?mode=memory")
	if err != nil {
		t.Fatalf("Failed to create a cache: %s", err)
	}

	return cache
}

func TestCacheCreation(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "cache-test-*.db")
	if err != nil {
		t.Fatalf("Temporary file creation failed: %s", err)
	}
	filepath := tmpFile.Name()

	tmpFile.Close()
	os.Remove(filepath)
	
	cache, err := c.New(filepath)
	if err != nil {
		t.Fatalf("Failed to create a cache: %s", err)
	}
	cache.Close()
}

func TestPageAddition(t *testing.T) {
	cache := setupCache(t)

	page := c.Page {
		Url:     "example.com",
		Method:  "GET",
		Headers: []byte("1"),
		Content: []byte("1"),
	}

	err := cache.AddPage(page)
	if err != nil {
		t.Fatalf("Couldn't add page to cache: %s", err)
	}

	cachedPage, err := cache.GetPage(page.Url, page.Method)
	if err != nil {
		t.Fatalf("Couldn't get the added page from cache: %s", err)
	}

	if !equalPages(cachedPage, page) {
		t.Fatalf("Cached page isn't equal to the actual page\nAdded: %s\nReturned: %s", page, cachedPage)
	}
}

func TestPageUpdate(t *testing.T) {
	cache := setupCache(t)

	// The first page version
	page1 := c.Page {
		Url:     "example.com",
		Method:  "GET",
		Headers: []byte("1"),
		Content: []byte("1"),
	}

	// The second page version
	page2 := c.Page {
		Url:     page1.Url,
		Method:  page1.Method,
		Headers: []byte("2"),
		Content: []byte("2"),
	}

	// Add first page version
	err := cache.AddPage(page1)
	if err != nil {
		t.Fatalf("Couldn't add page to cache: %s", err)
	}

	// Add new page version
	err = cache.AddPage(page2)
	if err != nil {
		t.Fatalf("Couldn't update page in cache: %s", err)
	}

	// Get new page version from cache
	cachedPage, err := cache.GetPage(page1.Url, page1.Method)
	if err != nil {
		t.Fatalf("Couldn't get the added page from cache: %s", err)
	}

	if equalPages(cachedPage, page2) {
		return
	}

	if equalPages(cachedPage, page1) {
		t.Fatalf("Page wasn't updated in cache, the first version is returned")
	} else {
		t.Fatalf("Not any of the two pages was returned, but something was returned.\nFirst: %s\nSecond: %s\nReturned: %s", page1, page2, cachedPage)
	}
}

func TestPageUniqueness(t *testing.T) {
	cache := setupCache(t)

	// The first page version
	page1 := c.Page {
		Url:     "example.com",
		Method:  "GET",
		Headers: []byte("1"),
		Content: []byte("1"),
	}

	// The second page version
	page2 := c.Page {
		Url:     page1.Url,
		Method:  "POST",
		Headers: []byte("2"),
		Content: []byte("2"),
	}

	// Add page GET version
	err := cache.AddPage(page1)
	if err != nil {
		t.Fatalf("Couldn't add GET page to cache: %s", err)
	}

	// Add page POST version
	err = cache.AddPage(page2)
	if err != nil {
		t.Fatalf("Couldn't add POST page to cache: %s", err)
	}

	// Get new page version from cache
	cachedGET, err := cache.GetPage(page1.Url, page1.Method)
	if err != nil {
		t.Fatalf("Couldn't get the added GET page from cache: %s", err)
	}

	// Get new page version from cache
	cachedPOST, err := cache.GetPage(page2.Url, page2.Method)
	if err != nil {
		t.Fatalf("Couldn't get the added POST page from cache: %s", err)
	}

	if equalPages(cachedGET, page1) && equalPages(cachedPOST, page2) {
		return
	}

	if equalPages(cachedGET, cachedPOST) {
		t.Fatalf("Both GET and POST methods have the same page cached\nGET: %s\nPOST: %s\nResult: %s", page1, page2, cachedGET)
	} else if equalPages(cachedGET, page1) {
		t.Fatalf("Cached POST page doesn't match added POST page\nGET: %s\nPOST: %s\nCached POST: %s", page1, page2, cachedPOST)
	} else {
		t.Fatalf("Cached GET page doesn't match added GET page\nGET: %s\nPOST: %s\nCached GET: %s", page1, page2, cachedGET)
	}
}

func TestGetUncachedPage(t *testing.T) {
	cache := setupCache(t)

	// Get uncached page
	cached, err := cache.GetPage("example.com", "GET")
	if err == nil {
		t.Fatalf("Didn't receive error on uncached page request\nResult: %s", cached)
	}

	if err != sql.ErrNoRows {
		t.Fatalf("Received unexpected error: %s", err)
	}
}

func TestDeletePage(t *testing.T) {
	cache := setupCache(t)

	page := c.Page {
		Url:     "example.com",
		Method:  "GET",
		Headers: []byte("1"),
		Content: []byte("1"),
	}

	err := cache.AddPage(page)
	if err != nil {
		t.Fatalf("Couldn't add a page to cache: %s", err)
	}

	err = cache.DeletePage(page.Url, page.Method)
	if err != nil {
		t.Fatalf("Couldn't delete a page from cache: %s", err)
	}

	cachedPage, err := cache.GetPage(page.Url, page.Method)
	if err == nil {
		t.Fatalf("Received a page after deletion: %s", cachedPage)
	}

	if err != sql.ErrNoRows {
		t.Fatalf("Received unexpected error: %s", err)
	}
}
