package cache

import (
    "database/sql"
    "bytes"
    "compress/gzip"
    "fmt"
    "io"
    _ "github.com/mattn/go-sqlite3"
)

const gzipLevel = 9

type Page struct {
    Url      string
    Headers  []byte
    Content  []byte
    Hash     uint64
}

type Cache struct {
    path  string
    db    *sql.DB
}

func Compress(content []byte) ([]byte, error) {
    var buffer bytes.Buffer
    gz, err := gzip.NewWriterLevel(&buffer, gzipLevel)
    if err != nil {
        return []byte{}, err
    }

    if _, err = gz.Write(content); err != nil {
        return []byte{}, err
    }

    if err = gz.Close(); err != nil {
        return []byte{}, err
    }

    return buffer.Bytes(), nil
}

func Decompress(compressed []byte) ([]byte, error) {
    buffer := bytes.NewReader(compressed)
    gz, err := gzip.NewReader(buffer)
    if err != nil {
        return []byte{}, err
    }
    defer gz.Close()

    content, err := io.ReadAll(gz)
    if err != nil {
        return []byte{}, err
    }

    return content, nil
}

func (cache *Cache) Load(path string) error {
    db, err := sql.Open("sqlite3", path)
    if err != nil {
        return err
    }

    cache.db = db
    cache.path = path

    stmt := `
    PRAGMA foreign_keys = ON;
    
    CREATE TABLE IF NOT EXISTS Pages (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        url TEXT UNIQUE NOT NULL,
        headers BLOB,
        content BLOB,
        hash INTEGER
    );

    CREATE TABLE IF NOT EXISTS Assets (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        url TEXT UNIQUE NOT NULL,
        headers BLOB,
        content BLOB,
        hash INTEGER,
        ref_count INTEGER DEFAULT 0
    );
    
    CREATE TABLE IF NOT EXISTS PagesAssets (
        page_id INTEGER REFERENCES Pages(id) ON DELETE CASCADE,
        asset_id INTEGER REFERENCES Assets(id),
        PRIMARY KEY(page_id, asset_id)
    );

    CREATE TRIGGER IF NOT EXISTS increment_asset_ref
    AFTER INSERT ON PagesAssets
    BEGIN
        UPDATE Assets SET ref_count = ref_count + 1 WHERE id = NEW.asset_id;
    END;

    CREATE TRIGGER IF NOT EXISTS decrement_and_cleanup_assets
    AFTER DELETE ON PagesAssets
    BEGIN
        UPDATE Assets SET ref_count = ref_count - 1 WHERE id = OLD.asset_id;
        DELETE FROM Assets WHERE id = OLD.asset_id AND ref_count <= 0;
    END;`

    _, err = cache.db.Exec(stmt)
    if err != nil {
        return err
    }

    return nil
}

func (cache *Cache) AddPage(page Page) error {
    url := page.Url
    headers, err := Compress(page.Headers)
    if err != nil {
        return err
    }
    content, err := Compress(page.Content)
    if err != nil {
        return err
    }

    _, err = cache.db.Exec(`
        INSERT INTO Pages(url, headers, content, hash)
        VALUES (?, ?, ?, ?)
        ON CONFLICT(url) DO UPDATE SET
            content = excluded.content,
            hash = excluded.hash
        WHERE hash != excluded.hash
    `, url, headers, content, int64(page.Hash))
    if err != nil {
        return fmt.Errorf("Insert/Update page: %s", err)
    }

    return nil
}

func (cache *Cache) AddAsset(page_url string, asset Page) error {
    tx, err := cache.db.Begin()
    if err != nil {
        return err
    }
    defer tx.Rollback()

    var pageID int64
    err = tx.QueryRow(`SELECT id FROM Pages WHERE url=?`, page_url).Scan(&pageID)
    if err != nil {
        return fmt.Errorf("Select page_id by url - %s: %s", page_url, err)
    }

    url := asset.Url
    headers, err := Compress(asset.Headers)
    if err != nil {
        return err
    }
    content, err := Compress(asset.Content)
    if err != nil {
        return err
    }
    var assetID int64
    err = tx.QueryRow(`
        INSERT INTO Assets(url, headers, content, hash)
        VALUES (?, ?, ?, ?)
        ON CONFLICT(url) DO UPDATE SET
            content = excluded.content,
            hash = excluded.hash
        WHERE hash != excluded.hash
        RETURNING id
    `, url, headers, content, int64(asset.Hash)).Scan(&assetID)
    if err != nil {
        return fmt.Errorf("Asset insert: %s", err)
    }

    _, err = tx.Exec(`
        INSERT INTO PagesAssets(page_id, asset_id)
        VALUES (?, ?)
        ON CONFLICT DO NOTHING
    `, pageID, assetID)
    if err != nil {
        return fmt.Errorf("Insert page-asset links: %s", err)
    }

    return tx.Commit()
}

func (cache *Cache) GetPage(url string) (Page, error) {
    var (
        headers  []byte
        content  []byte
        hash     int64
    )

    err := cache.db.QueryRow(`
        SELECT headers, content, hash FROM Pages WHERE url = ?
    `, url).Scan(&headers, &content, &hash)
    if err != nil {
        return Page {}, err
    }

    headers, err = Decompress(headers)
    if err != nil {
        return Page{}, err
    }
    content, err = Decompress(content)
    if err != nil {
        return Page{}, err
    }

    return Page {
        Url: url,
        Headers: headers,
        Content: content,
        Hash: uint64(hash),
    }, err
}

func (cache *Cache) GetAsset(url string) (Page, error) {
    var (
        headers  []byte
        content  []byte
        hash     int64
    )

    err := cache.db.QueryRow(`
        SELECT headers, content, hash FROM Assets WHERE url = ?
    `, url).Scan(&headers, &content, &hash)
    if err != nil {
        return Page {}, err
    }

    headers, err = Decompress(headers)
    if err != nil {
        return Page{}, err
    }
    content, err = Decompress(content)
    if err != nil {
        return Page{}, err
    }

    return Page {
        Url: url,
        Headers: headers,
        Content: content,
        Hash: uint64(hash),
    }, err
}

func (cache *Cache) DeletePage(url string) error {
    _, err := cache.db.Exec(`DELETE FROM Pages WHERE url = ?`, url)
    return err
}

func (cache *Cache) Close() error {
    return cache.db.Close()
}
