package cluster

import (
	"context"

	"github.com/yaegashi/kompoxops/domain/model"
)

// ProvisionInput represents a command to provision a cluster.
type ProvisionInput struct {
	ClusterID string `json:"cluster_id"`
	Force     bool   `json:"force,omitempty"`
}
type ProvisionOutput struct{}

// Provision provisions a cluster.
func (u *UseCase) Provision(ctx context.Context, in *ProvisionInput) (*ProvisionOutput, error) {
	if in == nil || in.ClusterID == "" {
		return nil, model.ErrClusterInvalid
	}
	c, err := u.Repos.Cluster.Get(ctx, in.ClusterID)
	if err != nil {
		return nil, err
	}
	var opts []model.ClusterProvisionOption
	if in.Force {
		opts = append(opts, model.WithClusterProvisionForce())
	}
	if err := u.ClusterPort.Provision(ctx, c, opts...); err != nil {
		return nil, err
	}
	return &ProvisionOutput{}, nil
}
