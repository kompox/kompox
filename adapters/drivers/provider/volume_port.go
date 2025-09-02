package providerdrv

import (
	"context"
	"fmt"

	"github.com/kompox/kompox/domain"
	"github.com/kompox/kompox/domain/model"
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

// DiskList returns the list of disks associated with the
// logical volume identified by volName for the specified cluster/app.
func (a *volumePortAdapter) DiskList(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, opts ...model.VolumeDiskListOption) ([]*model.VolumeDisk, error) {
	drv, err := a.getDriver(ctx, cluster, app)
	if err != nil {
		return nil, err
	}
	return drv.VolumeDiskList(ctx, cluster, app, volName, opts...)
}

// DiskCreate creates (provisions) a new disk belonging to
// the logical volume identified by volName for the specified cluster/app.
func (a *volumePortAdapter) DiskCreate(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, opts ...model.VolumeDiskCreateOption) (*model.VolumeDisk, error) {
	drv, err := a.getDriver(ctx, cluster, app)
	if err != nil {
		return nil, err
	}
	return drv.VolumeDiskCreate(ctx, cluster, app, volName, opts...)
}

// DiskDelete deletes the named disk (diskName) belonging
// to the logical volume volName for the specified cluster/app.
func (a *volumePortAdapter) DiskDelete(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, diskName string, opts ...model.VolumeDiskDeleteOption) error {
	drv, err := a.getDriver(ctx, cluster, app)
	if err != nil {
		return err
	}
	return drv.VolumeDiskDelete(ctx, cluster, app, volName, diskName, opts...)
}

// DiskAssign assigns an existing disk (diskName) to the
// logical volume volName for the specified cluster/app. The exact semantics (e.g.
// attachment vs. reference update) are implemented by the provider driver.
func (a *volumePortAdapter) DiskAssign(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, diskName string, opts ...model.VolumeDiskAssignOption) error {
	drv, err := a.getDriver(ctx, cluster, app)
	if err != nil {
		return err
	}
	return drv.VolumeDiskAssign(ctx, cluster, app, volName, diskName, opts...)
}

// SnapshotList lists snapshots for the given logical volume.
func (a *volumePortAdapter) SnapshotList(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, opts ...model.VolumeSnapshotListOption) ([]*model.VolumeSnapshot, error) {
	drv, err := a.getDriver(ctx, cluster, app)
	if err != nil {
		return nil, err
	}
	return drv.VolumeSnapshotList(ctx, cluster, app, volName, opts...)
}

// SnapshotCreate creates a snapshot from a specified volume disk.
func (a *volumePortAdapter) SnapshotCreate(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, diskName string, opts ...model.VolumeSnapshotCreateOption) (*model.VolumeSnapshot, error) {
	drv, err := a.getDriver(ctx, cluster, app)
	if err != nil {
		return nil, err
	}
	return drv.VolumeSnapshotCreate(ctx, cluster, app, volName, diskName, opts...)
}

// SnapshotDelete deletes the specified snapshot.
func (a *volumePortAdapter) SnapshotDelete(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, snapName string, opts ...model.VolumeSnapshotDeleteOption) error {
	drv, err := a.getDriver(ctx, cluster, app)
	if err != nil {
		return err
	}
	return drv.VolumeSnapshotDelete(ctx, cluster, app, volName, snapName, opts...)
}

// SnapshotRestore creates a new volume disk from the specified snapshot.
func (a *volumePortAdapter) SnapshotRestore(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, snapName string, opts ...model.VolumeSnapshotRestoreOption) (*model.VolumeDisk, error) {
	drv, err := a.getDriver(ctx, cluster, app)
	if err != nil {
		return nil, err
	}
	return drv.VolumeSnapshotRestore(ctx, cluster, app, volName, snapName, opts...)
}

// GetVolumePort returns a model.VolumePort implemented via provider drivers.
func GetVolumePort(services domain.ServiceRepository, providers domain.ProviderRepository, clusters domain.ClusterRepository, apps domain.AppRepository) model.VolumePort {
	return &volumePortAdapter{services: services, providers: providers, clusters: clusters, apps: apps}
}
