package cache

import (
	_ "embed"
	"errors"
)

//go:embed init.sql
var initStmt string

// Values for filtering pages by cacheability
var (
	CacheableMethods     = []string{"GET", "HEAD", "OPTIONS", "QUERY"}
	CacheableStatusCodes = []int{200, 203, 301, 404, 405, 410, 414, 501}
)

// Errors
var (
	ErrFailedToCreateDirForCache = errors.New("Failed to create a directory for cache")
	ErrUnknownDriver             = errors.New("Unknown SQL driver")
	ErrInvalidDBPath             = errors.New("Database path is invalid")
	ErrInitScriptFailed          = errors.New("Initialization script failed")
	ErrUncachableMethod          = errors.New("Page with uncachable method was given")
	ErrUncachableStatusCode      = errors.New("Page with uncachable status code was given")
	ErrFailedToAddPage           = errors.New("Failed to add a page")
	ErrNoDirectMatchWasFound     = errors.New("No direct match was found in cache")
	ErrNoFallbackMatchWasFound   = errors.New("No fallback match was found in cache")
	ErrFailedToDeletePage        = errors.New("Failed to delete a page")
	ErrFailedToCloseDatabase     = errors.New("Failed to close the dabase connection to a group")
)
