package cache

import (
	"net/http"
	"testing"
)

// Test ETag matching
func TestETagMatching(t *testing.T) {
	tests := []struct {
		label     string
		header    string
		etag      string
		isStrong  bool
		wantMatch bool
	}{
		{
			label:     "Any",
			header:    `*`,
			etag:      `"abcdef"`,
			isStrong:  true,
			wantMatch: true,
		},
		{
			label:     "Strong vs Strong",
			header:    `"abcdef"`,
			etag:      `"abcdef"`,
			isStrong:  true,
			wantMatch: true,
		},
		{
			label:     "Strong vs Strong",
			header:    `"abcdef"`,
			etag:      `"xyz123"`,
			isStrong:  true,
			wantMatch: false,
		},
		{
			label:     "Weak vs Strong",
			header:    `W/"abcdef"`,
			etag:      `"abcdef"`,
			isStrong:  true,
			wantMatch: false,
		},
		{
			label:     "Strong vs Weak",
			header:    `"abcdef"`,
			etag:      `W/"abcdef"`,
			isStrong:  true,
			wantMatch: false,
		},
		{
			label:     "Weak vs Weak",
			header:    `W/"abcdef"`,
			etag:      `W/"abcdef"`,
			isStrong:  true,
			wantMatch: false,
		},

		{
			label:     "Weak vs Weak",
			header:    `W/"abcdef"`,
			etag:      `W/"abcdef"`,
			isStrong:  false,
			wantMatch: true,
		},
		{
			label:     "Weak vs Strong",
			header:    `W/"abcdef"`,
			etag:      `"abcdef"`,
			isStrong:  false,
			wantMatch: true,
		},
		{
			label:     "Strong vs Weak",
			header:    `"abcdef"`,
			etag:      `W/"abcdef"`,
			isStrong:  false,
			wantMatch: true,
		},
		{
			label:     "Weak vs Weak",
			header:    `W/"abcdef"`,
			etag:      `W/"xyz123"`,
			isStrong:  false,
			wantMatch: false,
		},
	}

	for _, test := range tests {
		t.Run(test.label, func(t *testing.T) {
			got := etagMatches(test.header, test.etag, test.isStrong)
			if got != test.wantMatch {
				t.Errorf("etagMatches(%q, %q, %t) = %t; want %t",
					test.header, test.etag, test.isStrong, got, test.wantMatch)
			}
		})
	}
}

// Test application of conditionals
func TestApplyConditional(t *testing.T) {
	tests := []struct {
		label                              string
		etag, lastModified                 string
		ifMatch, ifNoneMatch               string
		ifModifiedSince, ifUnmodifiedSince string
		method                             string
		wantStatusCode                     int
	}{
		{
			label:             "Matching ETag ifMatch",
			etag:              `"etag"`,
			lastModified:      ``,
			ifMatch:           `"etag"`,
			ifNoneMatch:       ``,
			ifModifiedSince:   ``,
			ifUnmodifiedSince: ``,
			wantStatusCode:    http.StatusOK,
		},
		{
			label:             "Non-matching ETag ifMatch",
			etag:              `"etag"`,
			lastModified:      ``,
			ifMatch:           `""`,
			ifNoneMatch:       ``,
			ifModifiedSince:   ``,
			ifUnmodifiedSince: ``,
			wantStatusCode:    http.StatusPreconditionFailed,
		},
		{
			label:             "Matching ETag ifNoneMatch with `GET`",
			etag:              `"etag"`,
			lastModified:      ``,
			ifMatch:           ``,
			ifNoneMatch:       `"etag"`,
			ifModifiedSince:   ``,
			ifUnmodifiedSince: ``,
			method:            "GET",
			wantStatusCode:    http.StatusNotModified,
		},
		{
			label:             "Matching ETag ifNoneMatch with an invalid method",
			etag:              `"etag"`,
			lastModified:      ``,
			ifMatch:           ``,
			ifNoneMatch:       `"etag"`,
			ifModifiedSince:   ``,
			ifUnmodifiedSince: ``,
			wantStatusCode:    http.StatusPreconditionFailed,
		},
		{
			label:             "Non-matching ETag ifNoneMatch",
			etag:              `"etag"`,
			lastModified:      ``,
			ifMatch:           ``,
			ifNoneMatch:       `""`,
			ifModifiedSince:   ``,
			ifUnmodifiedSince: ``,
			wantStatusCode:    http.StatusOK,
		},
		{
			label:             "Matching ETag ifModifiedSince",
			etag:              ``,
			lastModified:      `Tue, 20 Oct 2015 07:28:20 GMT`,
			ifMatch:           ``,
			ifNoneMatch:       ``,
			ifModifiedSince:   `Tue, 20 Oct 2015 07:27:20 GMT`,
			ifUnmodifiedSince: ``,
			wantStatusCode:    http.StatusOK,
		},
		{
			label:             "Non-matching ETag ifModifiedSince",
			etag:              ``,
			lastModified:      `Tue, 20 Oct 2015 07:28:20 GMT`,
			ifMatch:           ``,
			ifNoneMatch:       ``,
			ifModifiedSince:   `Tue, 20 Oct 2015 07:29:20 GMT`,
			ifUnmodifiedSince: ``,
			wantStatusCode:    http.StatusNotModified,
		},
		{
			label:             "Matching ETag ifUnmodifiedSince",
			etag:              ``,
			lastModified:      `Tue, 20 Oct 2015 07:28:20 GMT`,
			ifMatch:           ``,
			ifNoneMatch:       ``,
			ifModifiedSince:   ``,
			ifUnmodifiedSince: `Tue, 20 Oct 2015 07:27:20 GMT`,
			wantStatusCode:    http.StatusPreconditionFailed,
		},
		{
			label:             "Non-matching ETag ifUnmodifiedSince",
			etag:              ``,
			lastModified:      `Tue, 20 Oct 2015 07:28:20 GMT`,
			ifMatch:           ``,
			ifNoneMatch:       ``,
			ifModifiedSince:   ``,
			ifUnmodifiedSince: `Tue, 20 Oct 2015 07:29:20 GMT`,
			method:            "GET",
			wantStatusCode:    http.StatusOK,
		},
	}

	page := Page{StatusCode: http.StatusOK}
	for _, test := range tests {
		page.ETag = test.etag
		page.LastModified = test.lastModified
		page.Method = test.method
		new_page := applyConditional(
			page,
			test.ifMatch,
			test.ifNoneMatch,
			test.ifModifiedSince,
			test.ifUnmodifiedSince,
		)

		if new_page.StatusCode != test.wantStatusCode {
			t.Errorf("%s: got %d, expected %d", test.label, new_page.StatusCode, test.wantStatusCode)
		}
	}
}

func FuzzETagMatching(f *testing.F) {
	f.Add(`"abcdef"`, `W/"abcdef"`, true)
	f.Add(`"xyz"`, `"xyz"`, false)
	f.Add(`W/"123"`, `"123"`, false)
	f.Add(`malformed`, `""`, true)

	f.Fuzz(func(t *testing.T, header string, etag string, isStrong bool) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Panicked with header=%q, etag=%q, isStrong=%t\nPanic: %v",
					header, etag, isStrong, r)
			}
		}()

		res := etagMatches(header, etag, isStrong)

		if isStrong && res {
			weakMatch := etagMatches(header, etag, false)
			if !weakMatch {
				t.Errorf("Logical error: Matched strongly but failed weakly for header=%q, etag=%q",
					header, etag)
			}
		}

		if header == etag && etagRegex.Match([]byte(header)) && etagRegex.Match([]byte(header)) {
			if !etagMatches(header, etag, false) {
				t.Errorf("Identical strings failed weak match: %q", header)
			}
		}
	})
}
