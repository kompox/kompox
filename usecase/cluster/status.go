package cluster

import (
	"context"

	"github.com/kompox/kompox/domain/model"
)

// StatusInput represents a command to get cluster status.
type StatusInput struct {
	// ClusterID identifies the cluster.
	ClusterID string `json:"cluster_id"`
}

// StatusOutput represents the response of cluster status.
type StatusOutput struct {
	model.ClusterStatus
	ClusterID   string `json:"cluster_id"`
	ClusterName string `json:"cluster_name"`
}

// Status returns the status of a cluster.
func (u *UseCase) Status(ctx context.Context, in *StatusInput) (*StatusOutput, error) {
	if in == nil || in.ClusterID == "" {
		return nil, model.ErrClusterInvalid
	}

	// Get cluster
	c, err := u.Repos.Cluster.Get(ctx, in.ClusterID)
	if err != nil {
		return nil, err
	}

	// Use injected cluster port directly
	cStatus, err := u.ClusterPort.Status(ctx, c)
	if err != nil {
		return nil, err
	}

	return &StatusOutput{
		ClusterStatus: *cStatus,
		ClusterID:     c.ID,
		ClusterName:   c.Name,
	}, nil
}
