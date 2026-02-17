package nodepool

import (
	"context"
	"fmt"

	"github.com/kompox/kompox/domain/model"
)

// CreateInput holds input parameters for creating a node pool.
type CreateInput struct {
	ClusterID string           `json:"cluster_id"`
	Pool      model.NodePool   `json:"pool"`
	Force     bool             `json:"force,omitempty"`
}

// CreateOutput holds the result of creating a node pool.
type CreateOutput struct {
	Pool *model.NodePool `json:"pool"`
}

// Create creates a new node pool in the specified cluster.
func (u *UseCase) Create(ctx context.Context, in *CreateInput) (*CreateOutput, error) {
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

	var opts []model.NodePoolCreateOption
	if in.Force {
		opts = append(opts, model.WithNodePoolCreateForce())
	}

	pool, err := u.NodePoolPort.NodePoolCreate(ctx, cluster, in.Pool, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create node pool: %w", err)
	}

	return &CreateOutput{Pool: pool}, nil
}
