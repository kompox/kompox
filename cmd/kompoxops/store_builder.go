package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/yaegashi/kompoxops/adapters/store/inmem"
	"github.com/yaegashi/kompoxops/adapters/store/rdb"
	"github.com/yaegashi/kompoxops/domain"
	"github.com/yaegashi/kompoxops/models/cfgops"
)

// getDBURL extracts the db-url flag value from command hierarchy.
func getDBURL(cmd *cobra.Command) string {
	f := findFlag(cmd, "db-url")
	if f != nil && f.Value.String() != "" {
		return f.Value.String()
	}
	return "file:kompoxops.yml"
}

// buildRepositories creates repositories based on db-url.
// If db-url starts with "file:", it loads the configuration file into memory store.
func buildRepositories(cmd *cobra.Command) (domain.ServiceRepository, domain.ProviderRepository, domain.ClusterRepository, domain.AppRepository, error) {
	dbURL := getDBURL(cmd)

	switch {
	case strings.HasPrefix(dbURL, "file:"):
		// Extract file path from file: URL
		filePath := strings.TrimPrefix(dbURL, "file:")
		if filePath == "" {
			return nil, nil, nil, nil, fmt.Errorf("file path is required for file: URL")
		}

		// Load configuration from file
		cfg, err := cfgops.Load(filePath)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("failed to load config from %s: %w", filePath, err)
		}

		// Create memory store and load configuration
		store := inmem.NewStore()
		ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
		defer cancel()

		if err := store.LoadFromConfig(ctx, cfg); err != nil {
			return nil, nil, nil, nil, fmt.Errorf("failed to load config into store: %w", err)
		}

		return store.ServiceRepo, store.ProviderRepo, store.ClusterRepo, store.AppRepo, nil

	case strings.HasPrefix(dbURL, "sqlite:") || strings.HasPrefix(dbURL, "sqlite3:"):
		db, err := rdb.OpenFromURL(dbURL)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		if err := rdb.AutoMigrate(db); err != nil {
			return nil, nil, nil, nil, err
		}
		return rdb.NewServiceRepository(db), rdb.NewProviderRepository(db), rdb.NewClusterRepository(db), rdb.NewAppRepository(db), nil

	default:
		return nil, nil, nil, nil, fmt.Errorf("unsupported db scheme: %s", dbURL)
	}
}
