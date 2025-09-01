package cluster

import (
	"context"

	"github.com/kompox/kompox/domain/model"
)

// GetInput identifies the cluster to fetch.
type GetInput struct {
	// ClusterID is the target cluster identifier.
	ClusterID string `json:"cluster_id"`
}

// GetOutput wraps the retrieved cluster.
type GetOutput struct {
	// Cluster is the fetched cluster entity.
	Cluster *model.Cluster `json:"cluster"`
}

// Get retrieves a cluster by ID.
func (u *UseCase) Get(ctx context.Context, in *GetInput) (*GetOutput, error) {
	if in == nil || in.ClusterID == "" {
		return nil, model.ErrClusterInvalid
	}
	c, err := u.Repos.Cluster.Get(ctx, in.ClusterID)
	if err != nil {
		return nil, err
	}
	return &GetOutput{Cluster: c}, nil
}
