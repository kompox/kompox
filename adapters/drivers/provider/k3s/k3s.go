package k3s

import (
	"context"
	"fmt"

	providerdrv "github.com/kompox/kompox/adapters/drivers/provider"
	"github.com/kompox/kompox/domain/model"
)

// driver implements the K3s provider driver.
type driver struct {
	serviceName  string
	providerName string
}

// ID returns the provider identifier.
func (d *driver) ID() string { return "k3s" }

// ServiceName returns the service name associated with this driver instance.
func (d *driver) ServiceName() string { return d.serviceName }

// ProviderName returns the provider name associated with this driver instance.
func (d *driver) ProviderName() string { return d.providerName }

// ClusterProvision is not implemented for K3s provider.
func (d *driver) ClusterProvision(ctx context.Context, cluster *model.Cluster, _ ...model.ClusterProvisionOption) error {
	return fmt.Errorf("ClusterProvision is not implemented for k3s provider")
}

// ClusterDeprovision is not implemented for K3s provider.
func (d *driver) ClusterDeprovision(ctx context.Context, cluster *model.Cluster, _ ...model.ClusterDeprovisionOption) error {
	return fmt.Errorf("ClusterDeprovision is not implemented for k3s provider")
}

// ClusterStatus is not implemented for K3s provider.
func (d *driver) ClusterStatus(ctx context.Context, cluster *model.Cluster) (*model.ClusterStatus, error) {
	return nil, fmt.Errorf("ClusterStatus is not implemented for k3s provider")
}

// ClusterInstall is not implemented for K3s provider.
func (d *driver) ClusterInstall(ctx context.Context, cluster *model.Cluster, _ ...model.ClusterInstallOption) error {
	return fmt.Errorf("ClusterInstall is not implemented for k3s provider")
}

// ClusterUninstall is not implemented for K3s provider.
func (d *driver) ClusterUninstall(ctx context.Context, cluster *model.Cluster, _ ...model.ClusterUninstallOption) error {
	return fmt.Errorf("ClusterUninstall is not implemented for k3s provider")
}

// ClusterKubeconfig is not implemented for K3s provider.
func (d *driver) ClusterKubeconfig(ctx context.Context, cluster *model.Cluster) ([]byte, error) {
	return nil, fmt.Errorf("ClusterKubeconfig is not implemented for k3s provider")
}

// Volume management (not implemented for k3s)
func (d *driver) VolumeDiskList(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, _ ...model.VolumeDiskListOption) ([]*model.VolumeDisk, error) {
	return nil, fmt.Errorf("VolumeDiskList is not implemented for k3s provider")
}
func (d *driver) VolumeDiskCreate(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, _ ...model.VolumeDiskCreateOption) (*model.VolumeDisk, error) {
	return nil, fmt.Errorf("VolumeDiskCreate is not implemented for k3s provider")
}
func (d *driver) VolumeDiskDelete(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, diskName string, _ ...model.VolumeDiskDeleteOption) error {
	return fmt.Errorf("VolumeDiskDelete is not implemented for k3s provider")
}
func (d *driver) VolumeDiskAssign(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, diskName string, _ ...model.VolumeDiskAssignOption) error {
	return fmt.Errorf("VolumeDiskAssign is not implemented for k3s provider")
}

// Snapshot operations (not implemented for k3s)
func (d *driver) VolumeSnapshotList(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, _ ...model.VolumeSnapshotListOption) ([]*model.VolumeSnapshot, error) {
	return nil, fmt.Errorf("VolumeSnapshotList is not implemented for k3s provider")
}
func (d *driver) VolumeSnapshotCreate(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, diskName string, _ ...model.VolumeSnapshotCreateOption) (*model.VolumeSnapshot, error) {
	return nil, fmt.Errorf("VolumeSnapshotCreate is not implemented for k3s provider")
}
func (d *driver) VolumeSnapshotDelete(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, snapName string, _ ...model.VolumeSnapshotDeleteOption) error {
	return fmt.Errorf("VolumeSnapshotDelete is not implemented for k3s provider")
}

// VolumeClass returns empty spec (no opinion) for k3s provider.
func (d *driver) VolumeClass(ctx context.Context, cluster *model.Cluster, app *model.App, vol model.AppVolume) (model.VolumeClass, error) {
	return model.VolumeClass{}, nil
}

// init registers the K3s driver.
func init() {
	providerdrv.Register("k3s", func(service *model.Service, provider *model.Provider) (providerdrv.Driver, error) {
		// Determine ServiceName
		serviceName := "(nil)"
		if service != nil {
			serviceName = service.Name
		}

		if provider.Settings != nil && provider.Settings["disabled"] == "true" {
			return nil, fmt.Errorf("k3s provider disabled by settings")
		}
		return &driver{
			serviceName:  serviceName,
			providerName: provider.Name,
		}, nil
	})
}
