package cluster

import (
	"context"

	"github.com/yaegashi/kompoxops/domain/model"
)

// UninstallInput represents a command to uninstall cluster resources.
type UninstallInput struct {
	ClusterID string `json:"cluster_id"`
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
	if err := u.ClusterPort.Uninstall(ctx, c); err != nil {
		return nil, err
	}
	return &UninstallOutput{}, nil
}
