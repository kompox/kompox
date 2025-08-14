package inmem

import (
	"context"

	"github.com/yaegashi/kompoxops/domain"
	"github.com/yaegashi/kompoxops/models/cfgops"
)

// Store provides a unified interface for all in-memory repositories.
type Store struct {
	ServiceRepository  *ServiceRepository
	ProviderRepository *ProviderRepository
	ClusterRepository  *ClusterRepository
	AppRepository      *AppRepository
	ConfigRoot         *cfgops.Root
}

// NewStore creates a new in-memory store with all repositories.
func NewStore() *Store {
	return &Store{
		ServiceRepository:  NewServiceRepository(),
		ProviderRepository: NewProviderRepository(),
		ClusterRepository:  NewClusterRepository(),
		AppRepository:      NewAppRepository(),
	}
}

// LoadFromConfig loads a kompoxops.yml configuration into the memory store.
func (s *Store) LoadFromConfig(ctx context.Context, cfg *cfgops.Root) error {
	// Convert configuration to domain models
	service, provider, cluster, app, err := cfg.ToModels()
	if err != nil {
		return err
	}

	// Store models in dependency order: service → provider → cluster → app
	if err := s.ServiceRepository.Create(ctx, service); err != nil {
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
	cfg, err := cfgops.Load(path)
	if err != nil {
		return err
	}
	return s.LoadFromConfig(ctx, cfg)
}

// Compile-time assertions
var _ domain.ServiceRepository = (*ServiceRepository)(nil)
var _ domain.ProviderRepository = (*ProviderRepository)(nil)
var _ domain.ClusterRepository = (*ClusterRepository)(nil)
var _ domain.AppRepository = (*AppRepository)(nil)
