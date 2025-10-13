package main

import (
	"context"

	"github.com/kompox/kompox/adapters/store/inmem"
	crdv1 "github.com/kompox/kompox/config/crd/ops/v1alpha1"
	"github.com/kompox/kompox/domain"
)

// buildReposFromCRD creates repositories from CRD sink.
func buildReposFromCRD(ctx context.Context, sink *crdv1.Sink) (*domain.Repositories, error) {
	// Create inmem store
	store := inmem.NewStore()

	// Prepare repositories for conversion
	repos := crdv1.Repositories{
		Workspace: store.WorkspaceRepository,
		Provider:  store.ProviderRepository,
		Cluster:   store.ClusterRepository,
		App:       store.AppRepository,
	}

	// Convert CRD sink to domain models
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
