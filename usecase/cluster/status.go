package cluster

import (
	"context"

	"github.com/yaegashi/kompoxops/domain/model"
)

// StatusInput represents a command to get cluster status.
type StatusInput struct {
	ID string `json:"id"`
}

// StatusOutput represents the response of cluster status.
type StatusOutput struct {
	model.ClusterStatus
	ClusterID   string `json:"cluster_id"`
	ClusterName string `json:"cluster_name"`
}

// Status returns the status of a cluster.
func (u *UseCase) Status(ctx context.Context, cmd StatusInput) (*StatusOutput, error) {
	if cmd.ID == "" {
		return nil, model.ErrClusterInvalid
	}

	// Get cluster
	c, err := u.Clusters.Get(ctx, cmd.ID)
	if err != nil {
		return nil, err
	}

	// Use injected status port directly
	cStatus, err := u.StatusPort.Status(c)
	if err != nil {
		return nil, err
	}

	return &StatusOutput{
		ClusterStatus: *cStatus,
		ClusterID:     c.ID,
		ClusterName:   c.Name,
	}, nil
}
