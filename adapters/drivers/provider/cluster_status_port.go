package providerdrv

import (
	"context"
	"fmt"

	"github.com/yaegashi/kompoxops/domain"
	"github.com/yaegashi/kompoxops/domain/model"
)

// clusterStatusPortAdapter implements model.ClusterStatusPort backed by provider drivers.
type clusterStatusPortAdapter struct {
	providers domain.ProviderRepository
}

func (a *clusterStatusPortAdapter) Status(ctx context.Context, cluster *model.Cluster) (*model.ClusterStatus, error) {
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

// GetClusterStatusPort returns a model.ClusterStatusPort implemented via provider drivers.
func GetClusterStatusPort(providers domain.ProviderRepository) model.ClusterStatusPort {
	return &clusterStatusPortAdapter{providers: providers}
}
