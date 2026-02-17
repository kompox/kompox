package nodepool

import (
	"context"
	"fmt"

	"github.com/kompox/kompox/domain/model"
)

// UpdateInput holds input parameters for updating a node pool.
type UpdateInput struct {
	ClusterID string         `json:"cluster_id"`
	Pool      model.NodePool `json:"pool"`
	Force     bool           `json:"force,omitempty"`
}

// UpdateOutput holds the result of updating a node pool.
type UpdateOutput struct {
	Pool *model.NodePool `json:"pool"`
}

// Update updates mutable fields of an existing node pool.
func (u *UseCase) Update(ctx context.Context, in *UpdateInput) (*UpdateOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("input is required")
	}
	if in.ClusterID == "" {
		return nil, fmt.Errorf("cluster ID is required")
	}
	if in.Pool.Name == nil || *in.Pool.Name == "" {
		return nil, fmt.Errorf("node pool name is required")
	}

	cluster, err := u.Repos.Cluster.Get(ctx, in.ClusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster: %w", err)
	}
	if cluster == nil {
		return nil, fmt.Errorf("cluster not found: %s", in.ClusterID)
	}

	var opts []model.NodePoolUpdateOption
	if in.Force {
		opts = append(opts, model.WithNodePoolUpdateForce())
	}

	pool, err := u.NodePoolPort.NodePoolUpdate(ctx, cluster, in.Pool, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to update node pool: %w", err)
	}

	return &UpdateOutput{Pool: pool}, nil
}
