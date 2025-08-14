package cluster

import (
	"context"
	"fmt"

	providerdrv "github.com/yaegashi/kompoxops/adapters/drivers/provider"
	"github.com/yaegashi/kompoxops/domain"
	"github.com/yaegashi/kompoxops/domain/model"
)

// StatusCommand represents a command to get cluster status.
type StatusCommand struct {
	ID string `json:"id"`
}

// StatusResponse represents the response of cluster status.
type StatusResponse struct {
	Existing    bool   `json:"existing"`    // cluster.existing の設定値
	Provisioned bool   `json:"provisioned"` // K8s クラスタが存在するとき true
	Installed   bool   `json:"installed"`   // K8s クラスタ内のリソースが存在するとき true
	ClusterID   string `json:"cluster_id"`
	ClusterName string `json:"cluster_name"`
}

// Status returns the status of a cluster.
func (u *UseCase) Status(ctx context.Context, cmd StatusCommand) (*StatusResponse, error) {
	if cmd.ID == "" {
		return nil, model.ErrClusterInvalid
	}

	// Get cluster
	cluster, err := u.Clusters.Get(ctx, cmd.ID)
	if err != nil {
		return nil, err
	}

	// Create status provider adapter
	statusProvider := &clusterStatusProviderAdapter{
		providerRepo: u.Providers,
	}

	// Get status through the cluster model
	status, err := cluster.GetStatus(statusProvider)
	if err != nil {
		return nil, err
	}

	return &StatusResponse{
		Existing:    status.Existing,
		Provisioned: status.Provisioned,
		Installed:   status.Installed,
		ClusterID:   cluster.ID,
		ClusterName: cluster.Name,
	}, nil
}

// clusterStatusProviderAdapter adapts the driver-based status checking to the model's interface.
type clusterStatusProviderAdapter struct {
	providerRepo domain.ProviderRepository
}

// GetClusterStatus implements model.ClusterStatusProvider.
func (a *clusterStatusProviderAdapter) GetClusterStatus(cluster *model.Cluster) (*model.ClusterStatus, error) {
	// Get provider
	provider, err := a.providerRepo.Get(context.Background(), cluster.ProviderID)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider %s: %w", cluster.ProviderID, err)
	}

	// Get driver factory
	factory, exists := providerdrv.GetDriverFactory(provider.Driver)
	if !exists {
		return nil, fmt.Errorf("unknown provider driver: %s", provider.Driver)
	}

	// Create driver with provider settings
	driver, err := factory(provider.Settings)
	if err != nil {
		return nil, fmt.Errorf("failed to create driver %s: %w", provider.Driver, err)
	}

	// Get status from driver
	driverStatus, err := driver.ClusterStatus(cluster)
	if err != nil {
		return nil, err
	}

	// Convert to model status
	return &model.ClusterStatus{
		Existing:    driverStatus.Existing,
		Provisioned: driverStatus.Provisioned,
		Installed:   driverStatus.Installed,
	}, nil
}
