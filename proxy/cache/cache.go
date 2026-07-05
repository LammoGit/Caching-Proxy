package cache

import (
    "database/sql"
	"log/slog"
	"fmt"
    _ "github.com/mattn/go-sqlite3"
)

type Page struct {
    Url      string
    Method   string
    Headers  []byte
    Content  string
}

type Cache struct {
    path  string
    db    *sql.DB
}

func (cache *Cache) Load(path string) error {
    db, err := sql.Open("sqlite3", path)
    if err != nil {
		slog.Error(fmt.Sprintf("Failed to open DB connection at path: %s", path))
        return err
    }

    cache.db = db
    cache.path = path

    stmt := `
    PRAGMA foreign_keys = ON;
    
    CREATE TABLE IF NOT EXISTS Pages (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        url TEXT NOT NULL,
        method TEXT NOT NULL,
        headers BLOB,
        content TEXT NOT NULL,
        UNIQUE(url, method)
    );
    `
    _, err = cache.db.Exec(stmt)
    if err != nil {
		slog.Error("Failed to execute initial script")
        return err
    }

    return nil
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
