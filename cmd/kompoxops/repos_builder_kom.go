package main

import (
	"context"

	"github.com/kompox/kompox/adapters/store/inmem"
	komv1 "github.com/kompox/kompox/config/crd/ops/v1alpha1"
	"github.com/kompox/kompox/domain"
)

// buildReposFromKOM creates repositories from KOM sink.
func buildReposFromKOM(ctx context.Context, sink *komv1.Sink) (*domain.Repositories, error) {
	// Create in-memory store (domain repositories)
	store := inmem.NewStore()

	// Prepare repositories for conversion
	repos := komv1.Repositories{
		Workspace: store.WorkspaceRepository,
		Provider:  store.ProviderRepository,
		Cluster:   store.ClusterRepository,
		App:       store.AppRepository,
	}

	// Convert KOM sink to domain models
	if err := sink.ToModels(ctx, repos); err != nil {
		return nil, err
	}

	// Return domain repositories
	return &domain.Repositories{
		Workspace: store.WorkspaceRepository,
		Provider:  store.ProviderRepository,
		Cluster:   store.ClusterRepository,
		App:       store.AppRepository,
	}, nil
}
