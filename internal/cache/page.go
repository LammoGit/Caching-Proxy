package cache

import (
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
