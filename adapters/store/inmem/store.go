package inmem

import (
	"context"

	"github.com/kompox/kompox/config/kompoxopscfg"
	"github.com/kompox/kompox/domain"
)

// Store provides a unified interface for all in-memory repositories.
type Store struct {
	WorkspaceRepository *WorkspaceRepository
	ProviderRepository  *ProviderRepository
	ClusterRepository   *ClusterRepository
	AppRepository       *AppRepository
	ConfigRoot          *kompoxopscfg.Root
}

// NewStore creates a new in-memory store with all repositories.
func NewStore() *Store {
	return &Store{
		WorkspaceRepository: NewWorkspaceRepository(),
		ProviderRepository:  NewProviderRepository(),
		ClusterRepository:   NewClusterRepository(),
		AppRepository:       NewAppRepository(),
	}
}

// LoadFromConfig loads a kompoxops.yml configuration into the memory store.
func (s *Store) LoadFromConfig(ctx context.Context, cfg *kompoxopscfg.Root) error {
	// Convert configuration to domain models
	workspace, provider, cluster, app, err := cfg.ToModels()
	if err != nil {
		return err
	}

	// Store models in dependency order: workspace → provider → cluster → app
	if err := s.WorkspaceRepository.Create(ctx, workspace); err != nil {
		return err
	}

	if err := s.ProviderRepository.Create(ctx, provider); err != nil {
		return err
	}

	if err := s.ClusterRepository.Create(ctx, cluster); err != nil {
		return err
	}

	if err := s.AppRepository.Create(ctx, app); err != nil {
		return err
	}

	s.ConfigRoot = cfg

	return nil
}

// LoadFromFile loads a kompoxops.yml file into the memory store.
func (s *Store) LoadFromFile(ctx context.Context, path string) error {
	cfg, err := kompoxopscfg.Load(path)
	if err != nil {
		return err
	}
	return s.LoadFromConfig(ctx, cfg)
}

// Compile-time assertions
var _ domain.WorkspaceRepository = (*WorkspaceRepository)(nil)
var _ domain.ProviderRepository = (*ProviderRepository)(nil)
var _ domain.ClusterRepository = (*ClusterRepository)(nil)
var _ domain.AppRepository = (*AppRepository)(nil)
