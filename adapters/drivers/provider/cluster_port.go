package providerdrv

import (
	"context"
	"fmt"

	"github.com/yaegashi/kompoxops/domain"
	"github.com/yaegashi/kompoxops/domain/model"
)

// clusterPortAdapter implements model.ClusterPort backed by provider drivers.
type clusterPortAdapter struct {
	providers domain.ProviderRepository
}

func (a *clusterPortAdapter) Status(ctx context.Context, cluster *model.Cluster) (*model.ClusterStatus, error) {
	// Get provider
	provider, err := a.providers.Get(ctx, cluster.ProviderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider %s: %w", cluster.ProviderID, err)
	}

	// Get driver factory
	factory, exists := GetDriverFactory(provider.Driver)
	if !exists {
		return nil, fmt.Errorf("unknown provider driver: %s", provider.Driver)
	}

	// Create driver with provider settings
	driver, err := factory(provider.Settings)
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

func (a *clusterPortAdapter) Provision(ctx context.Context, cluster *model.Cluster) error {
	// Get provider
	provider, err := a.providers.Get(ctx, cluster.ProviderID)
	if err != nil {
		return fmt.Errorf("failed to get provider %s: %w", cluster.ProviderID, err)
	}

	// Get driver factory
	factory, exists := GetDriverFactory(provider.Driver)
	if !exists {
		return fmt.Errorf("unknown provider driver: %s", provider.Driver)
	}

	// Create driver with provider settings
	driver, err := factory(provider.Settings)
	if err != nil {
		return fmt.Errorf("failed to create driver %s: %w", provider.Driver, err)
	}

	// Provision cluster via driver
	return driver.ClusterProvision(ctx, cluster)
}

func (a *clusterPortAdapter) Deprovision(ctx context.Context, cluster *model.Cluster) error {
	// Get provider
	provider, err := a.providers.Get(ctx, cluster.ProviderID)
	if err != nil {
		return fmt.Errorf("failed to get provider %s: %w", cluster.ProviderID, err)
	}

	// Get driver factory
	factory, exists := GetDriverFactory(provider.Driver)
	if !exists {
		return fmt.Errorf("unknown provider driver: %s", provider.Driver)
	}

	// Create driver with provider settings
	driver, err := factory(provider.Settings)
	if err != nil {
		return fmt.Errorf("failed to create driver %s: %w", provider.Driver, err)
	}

	// Deprovision cluster via driver
	return driver.ClusterDeprovision(ctx, cluster)
}

// GetClusterPort returns a model.ClusterPort implemented via provider drivers.
func GetClusterPort(providers domain.ProviderRepository) model.ClusterPort {
	return &clusterPortAdapter{providers: providers}
}
