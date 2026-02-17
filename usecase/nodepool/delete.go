package nodepool

import (
	"context"
	"fmt"

	"github.com/kompox/kompox/domain/model"
)

// DeleteInput holds input parameters for deleting a node pool.
type DeleteInput struct {
	ClusterID string
	Name      string
	Force     bool
}

// DeleteOutput holds the result of deleting a node pool.
type DeleteOutput struct{}

// Delete removes the specified node pool from the cluster.
func (u *UseCase) Delete(ctx context.Context, in *DeleteInput) (*DeleteOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("input is required")
	}
	if in.ClusterID == "" {
		return nil, fmt.Errorf("cluster ID is required")
	}
	if in.Name == "" {
		return nil, fmt.Errorf("node pool name is required")
	}

	cluster, err := u.Repos.Cluster.Get(ctx, in.ClusterID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster: %w", err)
	}
	if cluster == nil {
		return nil, fmt.Errorf("cluster not found: %s", in.ClusterID)
	}

	var opts []model.NodePoolDeleteOption
	if in.Force {
		opts = append(opts, model.WithNodePoolDeleteForce())
	}

	if err := u.NodePoolPort.NodePoolDelete(ctx, cluster, in.Name, opts...); err != nil {
		return nil, fmt.Errorf("failed to delete node pool: %w", err)
	}

	return &DeleteOutput{}, nil
}
