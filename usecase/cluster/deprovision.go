package cluster

import (
	"context"

	"github.com/yaegashi/kompoxops/domain/model"
)

// DeprovisionInput represents a command to deprovision a cluster.
type DeprovisionInput struct {
	ClusterID string `json:"cluster_id"`
}
type DeprovisionOutput struct{}

// Deprovision deprovisions a cluster.
func (u *UseCase) Deprovision(ctx context.Context, in *DeprovisionInput) (*DeprovisionOutput, error) {
	if in == nil || in.ClusterID == "" {
		return nil, model.ErrClusterInvalid
	}
	c, err := u.Repos.Cluster.Get(ctx, in.ClusterID)
	if err != nil {
		return nil, err
	}
	if err := u.ClusterPort.Deprovision(ctx, c); err != nil {
		return nil, err
	}
	return &DeprovisionOutput{}, nil
}
