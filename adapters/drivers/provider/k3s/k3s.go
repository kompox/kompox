package k3s

import (
	"context"
	"fmt"

	providerdrv "github.com/yaegashi/kompoxops/adapters/drivers/provider"
	"github.com/yaegashi/kompoxops/domain/model"
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
func (d *driver) ClusterProvision(ctx context.Context, cluster *model.Cluster) error {
	return fmt.Errorf("ClusterProvision is not implemented for k3s provider")
}

// ClusterDeprovision is not implemented for K3s provider.
func (d *driver) ClusterDeprovision(ctx context.Context, cluster *model.Cluster) error {
	return fmt.Errorf("ClusterDeprovision is not implemented for k3s provider")
}

// ClusterStatus is not implemented for K3s provider.
func (d *driver) ClusterStatus(ctx context.Context, cluster *model.Cluster) (*model.ClusterStatus, error) {
	return nil, fmt.Errorf("ClusterStatus is not implemented for k3s provider")
}

// ClusterInstall is not implemented for K3s provider.
func (d *driver) ClusterInstall(ctx context.Context, cluster *model.Cluster) error {
	return fmt.Errorf("ClusterInstall is not implemented for k3s provider")
}

// ClusterUninstall is not implemented for K3s provider.
func (d *driver) ClusterUninstall(ctx context.Context, cluster *model.Cluster) error {
	return fmt.Errorf("ClusterUninstall is not implemented for k3s provider")
}

// ClusterKubeconfig is not implemented for K3s provider.
func (d *driver) ClusterKubeconfig(ctx context.Context, cluster *model.Cluster) ([]byte, error) {
	return nil, fmt.Errorf("ClusterKubeconfig is not implemented for k3s provider")
}

// Volume management (not implemented for k3s)
func (d *driver) VolumeInstanceList(ctx context.Context, cluster *model.Cluster, app *model.App, volName string) ([]*model.AppVolumeInstance, error) {
	return nil, fmt.Errorf("VolumeInstanceList is not implemented for k3s provider")
}
func (d *driver) VolumeInstanceCreate(ctx context.Context, cluster *model.Cluster, app *model.App, volName string) (*model.AppVolumeInstance, error) {
	return nil, fmt.Errorf("VolumeInstanceCreate is not implemented for k3s provider")
}
func (d *driver) VolumeInstanceAssign(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, volInstName string) error {
	return fmt.Errorf("VolumeInstanceAssign is not implemented for k3s provider")
}
func (d *driver) VolumeInstanceDelete(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, volInstName string) error {
	return fmt.Errorf("VolumeInstanceDelete is not implemented for k3s provider")
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
