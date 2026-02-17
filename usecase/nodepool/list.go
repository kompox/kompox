package nodepool

import (
	"context"
	"fmt"

	"github.com/kompox/kompox/domain/model"
)

// ListInput holds input parameters for listing node pools.
type ListInput struct {
	ClusterID string `json:"cluster_id"`
	Name      string `json:"name,omitempty"` // Optional filter by node pool name
}

// ListOutput holds the result of listing node pools.
type ListOutput struct {
	Items []*model.NodePool `json:"items"`
}

// List retrieves node pools for the specified cluster.
func (u *UseCase) List(ctx context.Context, in *ListInput) (*ListOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("input is required")
	}
	if in.ClusterID == "" {
		return nil, fmt.Errorf("cluster ID is required")
	}

	cluster, err := u.Repos.Cluster.Get(ctx, in.ClusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster: %w", err)
	}
	if cluster == nil {
		return nil, fmt.Errorf("cluster not found: %s", in.ClusterID)
	}

	var opts []model.NodePoolListOption
	if in.Name != "" {
		opts = append(opts, model.WithNodePoolListName(in.Name))
	}

	pools, err := u.NodePoolPort.NodePoolList(ctx, cluster, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to list node pools: %w", err)
	}

	return &ListOutput{Items: pools}, nil
}
