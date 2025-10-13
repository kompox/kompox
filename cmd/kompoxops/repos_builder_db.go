package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/kompox/kompox/adapters/store/inmem"
	"github.com/kompox/kompox/adapters/store/rdb"
	"github.com/kompox/kompox/config/kompoxopscfg"
	"github.com/kompox/kompox/domain"
	"github.com/spf13/cobra"
)

// configRoot holds the loaded configuration.
var configRoot *kompoxopscfg.Root

// reposCache caches repositories for a given db-url (currently only for file: scheme)
// to ensure consistent in-memory IDs across multiple use case builders inside the
// same process lifetime. This avoids needing composite builders in most cases.
// NOTE: With file: scheme the underlying in-memory store is not persisted. Making
// this a process-wide singleton subtly changes semantics: mutations (e.g. disk
// creation) persist for subsequent commands in the same process. If isolation per
// command execution is required in the future, introduce a reset mechanism or
// scope cache by invocation.
var (
	reposCache   = map[string]*domain.Repositories{}
	reposCacheMu sync.Mutex
)

// getDBURL extracts the db-url flag value from command hierarchy.
func getDBURL(cmd *cobra.Command) string {
	f := findFlag(cmd, "db-url")
	if f != nil && f.Value.String() != "" {
		return f.Value.String()
	}
	return "file:kompoxops.yml"
}

// buildReposFromDB creates repositories from db-url.
func buildReposFromDB(cmd *cobra.Command) (*domain.Repositories, error) {
	dbURL := getDBURL(cmd)

	switch {
	case strings.HasPrefix(dbURL, "file:"):
		// Fast path: return cached repositories if present.
		reposCacheMu.Lock()
		if cached, ok := reposCache[dbURL]; ok && cached != nil {
			reposCacheMu.Unlock()
			return cached, nil
		}
		reposCacheMu.Unlock()
		// Extract file path from file: URL
		filePath := strings.TrimPrefix(dbURL, "file:")
		if filePath == "" {
			return nil, fmt.Errorf("file path is required for file: URL")
		}

		// Load configuration from file
		cfg, err := kompoxopscfg.Load(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to load config from %s: %w", filePath, err)
		}

		// Create memory store and load configuration
		store := inmem.NewStore()
		ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
		defer cancel()

		if err := store.LoadFromConfig(ctx, cfg); err != nil {
			return nil, fmt.Errorf("failed to load config into store: %w", err)
		}

		configRoot = store.ConfigRoot

		repos := &domain.Repositories{
			Workspace: store.WorkspaceRepository,
			Provider:  store.ProviderRepository,
			Cluster:   store.ClusterRepository,
			App:       store.AppRepository,
		}
		reposCacheMu.Lock()
		reposCache[dbURL] = repos
		reposCacheMu.Unlock()
		return repos, nil

	case strings.HasPrefix(dbURL, "sqlite:") || strings.HasPrefix(dbURL, "sqlite3:"):
		db, err := rdb.OpenFromURL(dbURL)
		if err != nil {
			return nil, err
		}
		if err := rdb.AutoMigrate(db); err != nil {
			return nil, err
		}
		return &domain.Repositories{
			Workspace: rdb.NewWorkspaceRepository(db),
			Provider:  rdb.NewProviderRepository(db),
			Cluster:   rdb.NewClusterRepository(db),
			App:       rdb.NewAppRepository(db),
		}, nil

	default:
		return nil, fmt.Errorf("unsupported db scheme: %s", dbURL)
	}
}
