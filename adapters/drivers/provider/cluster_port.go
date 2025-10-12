package providerdrv

import (
	"context"
	"fmt"

	"github.com/kompox/kompox/domain"
	"github.com/kompox/kompox/domain/model"
)

// clusterPortAdapter implements model.ClusterPort backed by provider drivers.
type clusterPortAdapter struct {
	workspaces domain.WorkspaceRepository
	providers  domain.ProviderRepository
}

// getDriver fetches a provider driver for the given cluster.
func (a *clusterPortAdapter) getDriver(ctx context.Context, cluster *model.Cluster) (Driver, error) {
	if cluster == nil {
		return nil, fmt.Errorf("cluster nil")
	}
	provider, err := a.providers.Get(ctx, cluster.ProviderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider %s: %w", cluster.ProviderID, err)
	}
	var workspace *model.Workspace
	if provider.WorkspaceID != "" {
		workspace, _ = a.workspaces.Get(ctx, provider.WorkspaceID)
	}
	factory, ok := GetDriverFactory(provider.Driver)
	if !ok {
		return nil, fmt.Errorf("unknown provider driver: %s", provider.Driver)
	}
	drv, err := factory(workspace, provider)
	if err != nil {
		return nil, fmt.Errorf("failed to create driver %s: %w", provider.Driver, err)
	}
	return drv, nil
}

// Status returns the current status of the specified cluster by delegating
// to the underlying provider driver implementation. It returns a *model.ClusterStatus
// describing existence, provisioning and installation state.
func (a *clusterPortAdapter) Status(ctx context.Context, cluster *model.Cluster) (*model.ClusterStatus, error) {
	drv, err := a.getDriver(ctx, cluster)
	if err != nil {
		return nil, err
	}
	return drv.ClusterStatus(ctx, cluster)
}

// Provision creates (provisions) the target Kubernetes cluster according to the
// specification contained in the provided *model.Cluster. The operation is performed
// via the provider driver.
func (a *clusterPortAdapter) Provision(ctx context.Context, cluster *model.Cluster, opts ...model.ClusterProvisionOption) error {
	drv, err := a.getDriver(ctx, cluster)
	if err != nil {
		return err
	}
	return drv.ClusterProvision(ctx, cluster, opts...)
}

// Deprovision deletes (deprovisions) the target Kubernetes cluster. Idempotent
// behavior depends on the driver; deleting a non-existent cluster should typically
// result in a no-op or a well-defined error.
func (a *clusterPortAdapter) Deprovision(ctx context.Context, cluster *model.Cluster, opts ...model.ClusterDeprovisionOption) error {
	drv, err := a.getDriver(ctx, cluster)
	if err != nil {
		return err
	}
	return drv.ClusterDeprovision(ctx, cluster, opts...)
}

// Install installs in-cluster supporting resources (e.g., ingress controller, CSI
// drivers, monitoring agents) required by Kompox. The precise set depends on the
// provider driver.
func (a *clusterPortAdapter) Install(ctx context.Context, cluster *model.Cluster, opts ...model.ClusterInstallOption) error {
	drv, err := a.getDriver(ctx, cluster)
	if err != nil {
		return err
	}
	return drv.ClusterInstall(ctx, cluster, opts...)
}

// Uninstall removes previously installed in-cluster supporting resources that were
// added by Install. Cluster itself is not deleted by this operation.
func (a *clusterPortAdapter) Uninstall(ctx context.Context, cluster *model.Cluster, opts ...model.ClusterUninstallOption) error {
	drv, err := a.getDriver(ctx, cluster)
	if err != nil {
		return err
	}
	return drv.ClusterUninstall(ctx, cluster, opts...)
}

// DNSApply delegates DNS record operations to the provider driver.
func (a *clusterPortAdapter) DNSApply(ctx context.Context, cluster *model.Cluster, rset model.DNSRecordSet, opts ...model.ClusterDNSApplyOption) error {
	drv, err := a.getDriver(ctx, cluster)
	if err != nil {
		return err
	}
	return drv.ClusterDNSApply(ctx, cluster, rset, opts...)
}

// GetClusterPort returns a model.ClusterPort implemented via provider drivers.
func GetClusterPort(workspaces domain.WorkspaceRepository, providers domain.ProviderRepository) model.ClusterPort {
	return &clusterPortAdapter{workspaces: workspaces, providers: providers}
}
