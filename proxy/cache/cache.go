package cache

import (
    "database/sql"
    "fmt"
    "strings"
    _ "github.com/mattn/go-sqlite3"
)

const maxAssetBatchSize = 300
const maxRelBatchSize = 499

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
    _, err := cache.db.Exec(`
        INSERT INTO Pages(url, headers, content, hash)
        VALUES (?, ?, ?, ?)
        ON CONFLICT(url) DO UPDATE SET
            content = excluded.content,
            hash = excluded.hash
        WHERE hash != excluded.hash
    `, page.Url, page.Headers, page.Content, int64(page.Hash))
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

    var assetID int64
    err = tx.QueryRow(`
        INSERT INTO Assets(url, headers, content, hash)
        VALUES (?, ?, ?, ?)
        ON CONFLICT(url) DO UPDATE SET
            content = excluded.content,
            hash = excluded.hash
        WHERE hash != excluded.hash
        RETURNING id
    `, asset.Url, asset.Headers, asset.Content, int64(asset.Hash)).Scan(&assetID)
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

func (cache *Cache) AddAssets(page_url string, assets []Page) error {
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

    if len(assets) == 0 {
        return tx.Commit()
    }

    assetIDs := make([]int64, 0, len(assets))
    for start := 0; start < len(assets); start += maxAssetBatchSize {
        end := start + maxAssetBatchSize
        if end > len(assets) {
            end = len(assets)
        }
        chunk := assets[start:end]

        valueStrings := make([]string, len(chunk))
        valueArgs := make([]interface{}, 0, len(chunk)*4)
        for i, a := range chunk {
            valueStrings[i] = "(?, ?, ?, ?)"
            valueArgs = append(valueArgs, a.Url, a.Headers, a.Content, a.Hash)
        }

        query := fmt.Sprintf(`
            INSERT INTO Assets(url, content, hash)
            VALUES %s
            ON CONFLICT(url) DO UPDATE SET
                content = excluded.content,
                hash = excluded.hash
            WHERE hash != excluded.hash
            RETURNING id
        `, strings.Join(valueStrings, ","))

        rows, err := tx.Query(query, valueArgs...)
        if err != nil {
            return fmt.Errorf("Batch asset insert chunk %d-%d: %s", start, end, err)
        }
        defer rows.Close()

        for rows.Next() {
            var id int64
            if err := rows.Scan(&id); err != nil {
                return err
            }
            assetIDs = append(assetIDs, id)
        }
        if err = rows.Err(); err != nil {
            return err
        }
        if len(assetIDs) != end {
            return fmt.Errorf("Returned %d asset IDs after %d assets", len(assetIDs), end)
        }
    }

    if len(assetIDs) != len(assets) {
        return fmt.Errorf("Returned %d asset IDs, expected %d", len(assetIDs), len(assets))
    }

    for start := 0; start < len(assetIDs); start += maxRelBatchSize {
        end := start + maxRelBatchSize
        if end > len(assetIDs) {
            end = len(assetIDs)
        }
        chunkIDs := assetIDs[start:end]

        relStrings := make([]string, len(chunkIDs))
        relArgs := make([]interface{}, 0, len(chunkIDs)*2)
        for i, aid := range chunkIDs {
            relStrings[i] = "(?, ?)"
            relArgs = append(relArgs, pageID, aid)
        }

        relQuery := fmt.Sprintf(`
            INSERT INTO PagesAssets(page_id, asset_id)
            VALUES %s
            ON CONFLICT DO NOTHING
        `, strings.Join(relStrings, ","))

        _, err = tx.Exec(relQuery, relArgs...)
        if err != nil {
            return fmt.Errorf("Batch insert page-asset links chunk %d-%d: %s", start, end, err)
        }
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
