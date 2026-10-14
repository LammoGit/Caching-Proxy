package cache

import (
	"database/sql"
	"net/http"
	"strings"
)

// getRow fetches a single cached page for an exact (url, method) pair.
// Returns sql.ErrNoRows if the pair is not cached.
func getRow(db *sql.DB, url, method string) (Page, error) {
	p := Page{URL: url, Method: method}
	err := db.QueryRow(`
		SELECT status_code, headers, content, etag, last_modified
		FROM Pages
		WHERE url = ? AND method = ?
	`, url, method).Scan(
		&p.StatusCode,
		&p.Headers,
		&p.Content,
		&p.ETag,
		&p.LastModified,
	)
	return p, err
}

// applyConditional evaluates preconditions against a cached page.
// If a precondition fails or resolves to "not modified", the returned Page
// has StatusCode set to 412 or 304 respectively and Content cleared.
func applyConditional(page Page, ifMatch, ifNoneMatch, ifModifiedSince, ifUnmodifiedSince string) Page {
	// If-Match: strong comparison; failure => 412.
	if ifMatch != "" && !etagMatches(ifMatch, page.ETag, true) {
		page.StatusCode = http.StatusPreconditionFailed
		page.Content = nil
		return page
	}

	// If-Unmodified-Since: fail if resource was modified after the given date.
	if ifUnmodifiedSince != "" && page.LastModified != "" {
		if t1, err1 := http.ParseTime(ifUnmodifiedSince); err1 == nil {
			if t2, err2 := http.ParseTime(page.LastModified); err2 == nil && t2.After(t1) {
				page.StatusCode = http.StatusPreconditionFailed
				page.Content = nil
				return page
			}
		}
	}

	// If-None-Match: match => 304 for GET/HEAD, 412 for others.
	if ifNoneMatch != "" && etagMatches(ifNoneMatch, page.ETag, false) {
		if page.Method == http.MethodGet || page.Method == http.MethodHead {
			page.StatusCode = http.StatusNotModified
		} else {
			page.StatusCode = http.StatusPreconditionFailed
		}
		page.Content = nil
		return page
	}

	// If-Modified-Since: only honored when If-None-Match is absent (RFC 9110 §13.1.3).
	if ifModifiedSince != "" && ifNoneMatch == "" && page.LastModified != "" {
		if t1, err1 := http.ParseTime(ifModifiedSince); err1 == nil {
			if t2, err2 := http.ParseTime(page.LastModified); err2 == nil && !t2.After(t1) {
				page.StatusCode = http.StatusNotModified
				page.Content = nil
				return page
			}
		}
	}

	return page
}

// etagMatches checks an If-Match/If-None-Match header value against a stored
// ETag. Handles "*" and comma-separated lists. When strong is true, weak
// (W/) tags never match; when false, W/ is stripped before comparison.
func etagMatches(header, etag string, strong bool) bool {
	header = strings.TrimSpace(header)
	if header == "*" {
		return etag != ""
	}
	for _, candidate := range strings.Split(header, ",") {
		candidate = strings.TrimSpace(candidate)
		if strong {
			if !strings.HasPrefix(candidate, "W/") &&
				!strings.HasPrefix(etag, "W/") &&
				candidate == etag {
				return true
			}
		} else {
			if strings.TrimPrefix(candidate, "W/") == strings.TrimPrefix(etag, "W/") {
				return true
			}
		}
	}
	return false
}
