package rdb

import (
	"fmt"
	"strings"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// OpenFromURL opens a GORM DB based on a simple db-url string.
// Supported:
//   - sqlite:<dsn>   e.g., sqlite:./kompoxops.db or sqlite::memory:
//   - sqlite3:<dsn>  alias of sqlite
func OpenFromURL(dbURL string) (*gorm.DB, error) {
	switch {
	case strings.HasPrefix(dbURL, "sqlite:"):
		dsn := strings.TrimPrefix(dbURL, "sqlite:")
		if dsn == "" {
			dsn = "./kompoxops.db"
		}
		return gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	case strings.HasPrefix(dbURL, "sqlite3:"):
		dsn := strings.TrimPrefix(dbURL, "sqlite3:")
		if dsn == "" {
			dsn = "./kompoxops.db"
		}
		return gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	default:
		return nil, fmt.Errorf("unsupported db scheme: %s", dbURL)
	}
}

// AutoMigrate applies schema migrations for all RDB models.
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&WorkspaceRecord{}, &ProviderRecord{}, &ClusterRecord{}, &AppRecord{}, &BoxRecord{})
}
