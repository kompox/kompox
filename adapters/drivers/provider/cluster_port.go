package providerdrv

import (
	"context"
	"fmt"

	"github.com/yaegashi/kompoxops/domain"
	"github.com/yaegashi/kompoxops/domain/model"
)

// clusterPortAdapter implements model.ClusterPort backed by provider drivers.
type clusterPortAdapter struct {
	services  domain.ServiceRepository
	providers domain.ProviderRepository
}

func (a *clusterPortAdapter) Status(ctx context.Context, cluster *model.Cluster) (*model.ClusterStatus, error) {
	// Get provider
	provider, err := a.providers.Get(ctx, cluster.ProviderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider %s: %w", cluster.ProviderID, err)
	}

	// Get service (may be nil)
	var service *model.Service
	if provider.ServiceID != "" {
		service, err = a.services.Get(ctx, provider.ServiceID)
		if err != nil {
			// Log but continue - service may be nil for testing
			service = nil
		}
	}

	// Get driver factory
	factory, exists := GetDriverFactory(provider.Driver)
	if !exists {
		return nil, fmt.Errorf("unknown provider driver: %s", provider.Driver)
	}

	// Create driver with service and provider
	driver, err := factory(service, provider)
	if err != nil {
		return nil, fmt.Errorf("failed to create driver %s: %w", provider.Driver, err)
	}

	// Get status from driver
	ds, err := driver.ClusterStatus(ctx, cluster)
	if err != nil {
		return nil, err
	}

	return &model.ClusterStatus{
		Existing:    ds.Existing,
		Provisioned: ds.Provisioned,
		Installed:   ds.Installed,
	}, nil
}

// getDriverForCluster is a helper function to get a driver for a cluster
func (a *clusterPortAdapter) getDriverForCluster(ctx context.Context, cluster *model.Cluster) (Driver, error) {
	// Get provider
	provider, err := a.providers.Get(ctx, cluster.ProviderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider %s: %w", cluster.ProviderID, err)
	}

	// Get service (may be nil)
	var service *model.Service
	if provider.ServiceID != "" {
		service, err = a.services.Get(ctx, provider.ServiceID)
		if err != nil {
			// Log but continue - service may be nil for testing
			service = nil
		}
	}

	// Get driver factory
	factory, exists := GetDriverFactory(provider.Driver)
	if !exists {
		return nil, fmt.Errorf("unknown provider driver: %s", provider.Driver)
	}

	// Create driver with service and provider
	driver, err := factory(service, provider)
	if err != nil {
		return nil, fmt.Errorf("failed to create driver %s: %w", provider.Driver, err)
	}

	return driver, nil
}

func (a *clusterPortAdapter) Provision(ctx context.Context, cluster *model.Cluster) error {
	// Get driver
	driver, err := a.getDriverForCluster(ctx, cluster)
	if err != nil {
		return err
	}

	// Provision cluster via driver
	return driver.ClusterProvision(ctx, cluster)
}

func (a *clusterPortAdapter) Deprovision(ctx context.Context, cluster *model.Cluster) error {
	// Get driver
	driver, err := a.getDriverForCluster(ctx, cluster)
	if err != nil {
		return err
	}

	// Deprovision cluster via driver
	return driver.ClusterDeprovision(ctx, cluster)
}

func (a *clusterPortAdapter) Install(ctx context.Context, cluster *model.Cluster) error {
	// Get driver
	driver, err := a.getDriverForCluster(ctx, cluster)
	if err != nil {
		return err
	}

	// Install in-cluster resources via driver
	return driver.ClusterInstall(ctx, cluster)
}

func (a *clusterPortAdapter) Uninstall(ctx context.Context, cluster *model.Cluster) error {
	// Get driver
	driver, err := a.getDriverForCluster(ctx, cluster)
	if err != nil {
		return err
	}

	// Uninstall in-cluster resources via driver
	return driver.ClusterUninstall(ctx, cluster)
}

// GetClusterPort returns a model.ClusterPort implemented via provider drivers.
func GetClusterPort(services domain.ServiceRepository, providers domain.ProviderRepository) model.ClusterPort {
	return &clusterPortAdapter{services: services, providers: providers}
}
