package k3s

import (
	"context"
	"fmt"

	providerdrv "github.com/yaegashi/kompoxops/adapters/drivers/provider"
	"github.com/yaegashi/kompoxops/domain/model"
)

// driver implements the K3s provider driver.
type driver struct{}

// ID returns the provider identifier.
func (d *driver) ID() string { return "k3s" }

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

// init registers the K3s driver.
func init() {
	providerdrv.Register("k3s", func(settings map[string]string) (providerdrv.Driver, error) {
		if settings != nil && settings["disabled"] == "true" {
			return nil, fmt.Errorf("k3s provider disabled by settings")
		}
		return &driver{}, nil
	})
}
