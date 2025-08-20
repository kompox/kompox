package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/yaegashi/kompoxops/adapters/store/inmem"
	"github.com/yaegashi/kompoxops/adapters/store/rdb"
	"github.com/yaegashi/kompoxops/config/kompoxopscfg"
	"github.com/yaegashi/kompoxops/domain"
	"github.com/yaegashi/kompoxops/usecase/app"
	"github.com/yaegashi/kompoxops/usecase/cluster"
	"github.com/yaegashi/kompoxops/usecase/provider"
	"github.com/yaegashi/kompoxops/usecase/service"
	"github.com/yaegashi/kompoxops/usecase/volume"
)

// configRoot holds the loaded configuration.
var configRoot *kompoxopscfg.Root

// getDBURL extracts the db-url flag value from command hierarchy.
func getDBURL(cmd *cobra.Command) string {
	f := findFlag(cmd, "db-url")
	if f != nil && f.Value.String() != "" {
		return f.Value.String()
	}
	return "file:kompoxops.yml"
}

// buildRepos creates repositories based on db-url.
// If db-url starts with "file:", it loads the configuration file into memory store.
func buildRepos(cmd *cobra.Command) (*domain.Repositories, error) {
	dbURL := getDBURL(cmd)

	switch {
	case strings.HasPrefix(dbURL, "file:"):
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

		return &domain.Repositories{
			Service:  store.ServiceRepository,
			Provider: store.ProviderRepository,
			Cluster:  store.ClusterRepository,
			App:      store.AppRepository,
		}, nil

	case strings.HasPrefix(dbURL, "sqlite:") || strings.HasPrefix(dbURL, "sqlite3:"):
		db, err := rdb.OpenFromURL(dbURL)
		if err != nil {
			return nil, err
		}
		if err := rdb.AutoMigrate(db); err != nil {
			return nil, err
		}
		return &domain.Repositories{
			Service:  rdb.NewServiceRepository(db),
			Provider: rdb.NewProviderRepository(db),
			Cluster:  rdb.NewClusterRepository(db),
			App:      rdb.NewAppRepository(db),
		}, nil

	default:
		return nil, fmt.Errorf("unsupported db scheme: %s", dbURL)
	}
}

// buildAppRepos creates repositories needed for app use cases.
func buildAppRepos(cmd *cobra.Command) (*app.Repos, error) {
	repos, err := buildRepos(cmd)
	if err != nil {
		return nil, err
	}
	return &app.Repos{
		App:      repos.App,
		Service:  repos.Service,
		Provider: repos.Provider,
		Cluster:  repos.Cluster,
	}, nil
}

// buildClusterRepos creates repositories needed for cluster use cases.
func buildClusterRepos(cmd *cobra.Command) (*cluster.Repos, error) {
	repos, err := buildRepos(cmd)
	if err != nil {
		return nil, err
	}
	return &cluster.Repos{
		Service:  repos.Service,
		Cluster:  repos.Cluster,
		Provider: repos.Provider,
	}, nil
}

// buildProviderRepos creates repositories needed for provider use cases.
func buildProviderRepos(cmd *cobra.Command) (*provider.Repos, error) {
	repos, err := buildRepos(cmd)
	if err != nil {
		return nil, err
	}
	return &provider.Repos{
		Provider: repos.Provider,
	}, nil
}

// buildServiceRepos creates repositories needed for service use cases.
func buildServiceRepos(cmd *cobra.Command) (*service.Repos, error) {
	repos, err := buildRepos(cmd)
	if err != nil {
		return nil, err
	}
	return &service.Repos{
		Service: repos.Service,
	}, nil
}

// buildVolumeRepos creates repositories needed for volume use cases.
func buildVolumeRepos(cmd *cobra.Command) (*volume.Repos, error) {
	repos, err := buildRepos(cmd)
	if err != nil {
		return nil, err
	}
	return &volume.Repos{Service: repos.Service, Provider: repos.Provider, Cluster: repos.Cluster, App: repos.App}, nil
}
