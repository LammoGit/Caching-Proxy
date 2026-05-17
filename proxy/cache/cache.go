package cache

import (
    "gorm.io/driver/sqlite"
    "gorm.io/gorm/clause"
    "gorm.io/gorm"
    "fmt"
    "errors"
)

type Page struct {
    ID       uint    `gorm:"primaryKey"`
    Url      string  `gorm:"uniqueIndex"`
    Headers  []byte
    Content  []byte
    IsAsset  bool    `gorm:"index"`
    Parents  []*Page `gorm:"constraint:OnDelete:CASCADE;many2many:page_links;joinForeignKey:AssetID;joinReferences:ParentID;"`
}

type Cache struct {
    db      *gorm.DB
    Path    string
}

func (cache *Cache) Load(path string) error {
    db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
    if err != nil {
        return fmt.Errorf("failed to connect database: %s", err)
    }

    if err := db.AutoMigrate(&Page{}); err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

    cache.db = db
    cache.Path = path

    return nil
}

func (cache *Cache) Add(page Page) error {
	if cache.db == nil {
		return fmt.Errorf("database not initialized")
	}
	
	err := cache.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "url"}},
		UpdateAll: true,
	}).Create(&page).Error

	if err != nil {
		return fmt.Errorf("failed to add page: %w", err)
	}
	return nil
}

func (cache *Cache) Delete(url string) error {
	if cache.db == nil {
		return fmt.Errorf("database not initialized")
	}

    err := cache.db.Transaction(func(tx *gorm.DB) error {
        var page Page
        err := tx.Select("id").Where("url = ?", url).First(&page).Error
        if err != nil {
            if errors.Is(err, gorm.ErrRecordNotFound) {
                return nil
            }
            return err
        }

        var orphans []uint
        err = tx.Table("page_links").
                Where("ParentID = ?", page.ID).
                Pluck("AssetID", &orphans).Error
        if err != nil {
            return err
        }

        if err := tx.Delete(&page).Error; err != nil {
            return err
        }

        if len(orphans) > 0 {
            err = tx.Where("is_asset = ?", true).
                    Where("id IN (?)", orphans).
                    Where("id NOT IN (SELECT AssetID FROM page_links)").
                    Delete(&Page{}).Error
        }

        return nil
    })

    if err != nil {
		return fmt.Errorf("failed to safely delete page and cleanup orphans: %w", err)
	}

	return nil

}

func (cache *Cache) Get(url string) (*Page, error) {
	if cache.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var page Page
	err := cache.db.Where("url = ?", url).First(&page).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("page not found: %w", err)
		}
		return nil, fmt.Errorf("failed to get page: %w", err)
	}

	return &page, nil
}

func (cache *Cache) Close() error {
	if cache.db == nil {
		return nil
	}

	sqlDB, err := cache.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	err = sqlDB.Close()
	if err != nil {
		return fmt.Errorf("failed to close database: %w", err)
	}

	cache.db = nil
	return nil
}
