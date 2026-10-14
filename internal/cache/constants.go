package cache

import (
	_ "embed"
	"errors"
)

//go:embed init.sql
var initStmt string

var (
	CacheableMethods     = []string{"GET", "HEAD", "OPTIONS", "QUERY"}
	CacheableStatusCodes = []int{200, 203, 301, 404, 405, 410, 414, 501}
)

var (
	ErrUnknownDriver        = errors.New("unknown SQL driver")
	ErrInvalidDBPath        = errors.New("database path is invalid")
	ErrInitScriptFailed     = errors.New("initialization script failed")
	ErrUncachableMethod     = errors.New("page with uncachable method was given")
	ErrUncachableStatusCode = errors.New("page with uncachable status code was given")
)
