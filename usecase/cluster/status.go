package cluster

import (
	"context"

	"github.com/yaegashi/kompoxops/domain/model"
)

// StatusCommand represents a command to get cluster status.
type StatusCommand struct {
	ID string `json:"id"`
}

// StatusResponse represents the response of cluster status.
type StatusResponse struct {
	model.ClusterStatus
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

	// Use injected status port directly
	status, err := u.StatusPort.Status(cluster)
	if err != nil {
		return nil, err
	}

	return &StatusResponse{
		ClusterStatus: *status,
		ClusterID:     cluster.ID,
		ClusterName:   cluster.Name,
	}, nil
}
