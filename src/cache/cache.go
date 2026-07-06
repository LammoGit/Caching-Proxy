package cache

import (
	_ "embed"
    "database/sql"
	"log/slog"
	"fmt"

    _ "github.com/mattn/go-sqlite3"
)

//go:embed init.sql
var initStmt string

type Page struct {
    Url      string
    Method   string
    Headers  []byte
    Content  []byte
}

type Cache struct {
    path  string
    db    *sql.DB
}

func New(path string) (cache *Cache, err error) {
    db, err := sql.Open("sqlite3", path)
    if err != nil {
		slog.Error(fmt.Sprintf("Failed to open DB connection at path: %s", path))
        return
    }

	cache = &Cache {
		db: db,
		path: path,
	}

    _, err = cache.db.Exec(initStmt)
    if err != nil {
		slog.Error("Failed to execute initial script")
        return
    }

    return
}

func (cache *Cache) AddPage(page Page) (err error) {
	_, err = cache.db.Exec(`
        INSERT INTO Pages(url, method, headers, content)
        VALUES (?, ?, ?, ?)
        ON CONFLICT(url, method) DO UPDATE SET
            headers = excluded.headers,
            content = excluded.content;
    `, page.Url, page.Method, page.Headers, page.Content)
    if err != nil {
		slog.Debug(fmt.Sprintf("Failed to insert/update page: %s %s", page.Method, page.Url))
    } else {
		slog.Debug(fmt.Sprintf("Inserted/Updated page: %s %s", page.Method, page.Url))
	}
    return
}

func (cache *Cache) GetPage(url, method string) (page Page, err error) {
	page.Url = url
	page.Method = method

    err = cache.db.QueryRow(`
        SELECT
            headers,
            content
        FROM Pages
        WHERE url = ? AND method = ?
    `, url, method).Scan(&page.Headers, &page.Content)
	if err != nil {
		slog.Debug(fmt.Sprintf("Failed to get the page: %s %s", method, url))
	} else {
		slog.Debug(fmt.Sprintf("Successfully got the page: %s %s", method, url))
	}
    return
}

func (cache *Cache) DeletePage(url, method string) (err error) {
    _, err = cache.db.Exec(`
        DELETE FROM Pages
        WHERE url = ? AND method = ?
    `, url, method)
	if err != nil {
		slog.Debug(fmt.Sprintf("Failed to delete the page: %s %s", method, url))
	} else {
		slog.Debug(fmt.Sprintf("Successfully deleted the page: %s %s", method, url))
	}
    return
}

func (cache *Cache) Close() error {
	slog.Debug("Closing DB connection")
    return cache.db.Close()
}
