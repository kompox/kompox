package providerdrv

import (
	"context"
	"fmt"

	"github.com/yaegashi/kompoxops/domain"
	"github.com/yaegashi/kompoxops/domain/model"
)

// volumePortAdapter implements model.VolumePort backed by provider drivers.
type volumePortAdapter struct {
	services  domain.ServiceRepository
	providers domain.ProviderRepository
	clusters  domain.ClusterRepository
	apps      domain.AppRepository
}

// getDriver fetches driver for given cluster+app (cluster and app already looked up or retrieved inside methods).
func (a *volumePortAdapter) getDriver(ctx context.Context, cluster *model.Cluster, app *model.App) (Driver, error) {
	if cluster == nil || app == nil {
		return nil, fmt.Errorf("cluster/app nil")
	}
	provider, err := a.providers.Get(ctx, cluster.ProviderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider %s: %w", cluster.ProviderID, err)
	}
	var service *model.Service
	if provider.ServiceID != "" {
		service, _ = a.services.Get(ctx, provider.ServiceID)
	}
	factory, ok := GetDriverFactory(provider.Driver)
	if !ok {
		return nil, fmt.Errorf("unknown provider driver: %s", provider.Driver)
	}
	drv, err := factory(service, provider)
	if err != nil {
		return nil, fmt.Errorf("failed to create driver %s: %w", provider.Driver, err)
	}
	return drv, nil
}

func (a *volumePortAdapter) VolumeInstanceList(ctx context.Context, cluster *model.Cluster, app *model.App, volName string) ([]*model.AppVolumeInstance, error) {
	drv, err := a.getDriver(ctx, cluster, app)
	if err != nil {
		return nil, err
	}
	return drv.VolumeInstanceList(ctx, cluster, app, volName)
}

func (a *volumePortAdapter) VolumeInstanceCreate(ctx context.Context, cluster *model.Cluster, app *model.App, volName string) (*model.AppVolumeInstance, error) {
	drv, err := a.getDriver(ctx, cluster, app)
	if err != nil {
		return nil, err
	}
	return drv.VolumeInstanceCreate(ctx, cluster, app, volName)
}

func (a *volumePortAdapter) VolumeInstanceAssign(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, volInstName string) error {
	drv, err := a.getDriver(ctx, cluster, app)
	if err != nil {
		return err
	}
	return drv.VolumeInstanceAssign(ctx, cluster, app, volName, volInstName)
}

func (a *volumePortAdapter) VolumeInstanceDelete(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, volInstName string) error {
	drv, err := a.getDriver(ctx, cluster, app)
	if err != nil {
		return err
	}
	return drv.VolumeInstanceDelete(ctx, cluster, app, volName, volInstName)
}

// GetVolumePort returns a model.VolumePort implemented via provider drivers.
func GetVolumePort(services domain.ServiceRepository, providers domain.ProviderRepository, clusters domain.ClusterRepository, apps domain.AppRepository) model.VolumePort {
	return &volumePortAdapter{services: services, providers: providers, clusters: clusters, apps: apps}
}
