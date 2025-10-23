package cluster

import (
	"context"

	"github.com/kompox/kompox/domain/model"
)

// UninstallInput represents a command to uninstall cluster resources.
type UninstallInput struct {
	ClusterID string `json:"cluster_id"`
	Force     bool   `json:"force,omitempty"`
}
type UninstallOutput struct{}

// Uninstall uninstalls in-cluster resources (Ingress Controller, etc.).
func (u *UseCase) Uninstall(ctx context.Context, in *UninstallInput) (*UninstallOutput, error) {
	if in == nil || in.ClusterID == "" {
		return nil, model.ErrClusterInvalid
	}
	c, err := u.Repos.Cluster.Get(ctx, in.ClusterID)
	if err != nil {
		return nil, err
	}

	// Check protection policy (ignore Force flag as per ADR-013)
	if err := c.CheckInstallationProtection(model.OpDelete); err != nil {
		return nil, err
	}

	var opts []model.ClusterUninstallOption
	if in.Force {
		opts = append(opts, model.WithClusterUninstallForce())
	}
	if err := u.ClusterPort.Uninstall(ctx, c, opts...); err != nil {
		return nil, err
	}
	return &UninstallOutput{}, nil
}
