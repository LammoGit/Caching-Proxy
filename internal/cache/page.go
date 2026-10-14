package cache

import (
	"encoding/json"
	"slices"
)

// Page holds web-page's contents
type Page struct {
	URL          string
	Method       string
	StatusCode   int
	Headers      []byte
	Content      []byte
	ETag         string
	LastModified string
}

// Equal method that returns true if two pages are equal
func (page Page) Equal(other Page) bool {
	return page.URL == other.URL &&
		page.Method == other.Method &&
		page.StatusCode == other.StatusCode &&
		page.ETag == other.ETag &&
		page.LastModified == other.LastModified &&
		slices.Equal(page.Headers, other.Headers) &&
		slices.Equal(page.Content, other.Content)
}

// String returns string representation of a page
func (page Page) String() string {
	v := map[string]any{
		"url":           page.URL,
		"method":        page.Method,
		"status_code":   page.StatusCode,
		"headers":       string(page.Headers),
		"content":       string(page.Content),
		"etag":          page.ETag,
		"last_modified": page.LastModified,
	}
	s, _ := json.Marshal(v)
	return string(s)
}
