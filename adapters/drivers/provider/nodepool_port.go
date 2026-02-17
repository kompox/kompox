package providerdrv

import (
	"context"
	"fmt"

	"github.com/kompox/kompox/domain"
	"github.com/kompox/kompox/domain/model"
)

// nodePoolPortAdapter implements model.NodePoolPort backed by provider drivers.
type nodePoolPortAdapter struct {
	workspaces domain.WorkspaceRepository
	providers  domain.ProviderRepository
	clusters   domain.ClusterRepository
}

// getDriver fetches driver for given cluster.
func (a *nodePoolPortAdapter) getDriver(ctx context.Context, cluster *model.Cluster) (Driver, error) {
	if cluster == nil {
		return nil, fmt.Errorf("cluster is nil")
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

// NodePoolList returns a list of node pools for the specified cluster.
func (a *nodePoolPortAdapter) NodePoolList(ctx context.Context, cluster *model.Cluster, opts ...model.NodePoolListOption) ([]*model.NodePool, error) {
	drv, err := a.getDriver(ctx, cluster)
	if err != nil {
		return nil, err
	}
	return drv.NodePoolList(ctx, cluster, opts...)
}

// NodePoolCreate creates a new node pool in the cluster.
func (a *nodePoolPortAdapter) NodePoolCreate(ctx context.Context, cluster *model.Cluster, pool model.NodePool, opts ...model.NodePoolCreateOption) (*model.NodePool, error) {
	drv, err := a.getDriver(ctx, cluster)
	if err != nil {
		return nil, err
	}
	return drv.NodePoolCreate(ctx, cluster, pool, opts...)
}

// NodePoolUpdate updates mutable fields of an existing node pool.
func (a *nodePoolPortAdapter) NodePoolUpdate(ctx context.Context, cluster *model.Cluster, pool model.NodePool, opts ...model.NodePoolUpdateOption) (*model.NodePool, error) {
	drv, err := a.getDriver(ctx, cluster)
	if err != nil {
		return nil, err
	}
	return drv.NodePoolUpdate(ctx, cluster, pool, opts...)
}

// NodePoolDelete deletes the specified node pool from the cluster.
func (a *nodePoolPortAdapter) NodePoolDelete(ctx context.Context, cluster *model.Cluster, poolName string, opts ...model.NodePoolDeleteOption) error {
	drv, err := a.getDriver(ctx, cluster)
	if err != nil {
		return err
	}
	return drv.NodePoolDelete(ctx, cluster, poolName, opts...)
}

// GetNodePoolPort returns a model.NodePoolPort implemented via provider drivers.
func GetNodePoolPort(workspaces domain.WorkspaceRepository, providers domain.ProviderRepository, clusters domain.ClusterRepository) model.NodePoolPort {
	return &nodePoolPortAdapter{workspaces: workspaces, providers: providers, clusters: clusters}
}
