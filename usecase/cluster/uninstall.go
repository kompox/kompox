package cluster

import (
	"context"

	"github.com/yaegashi/kompoxops/domain/model"
)

// UninstallInput represents a command to uninstall cluster resources.
type UninstallInput struct {
	ID string `json:"id"`
}

// Uninstall uninstalls in-cluster resources (Ingress Controller, etc.).
func (u *UseCase) Uninstall(ctx context.Context, cmd UninstallInput) error {
	if cmd.ID == "" {
		return model.ErrClusterInvalid
	}

	// Get cluster
	c, err := u.Repos.Cluster.Get(ctx, cmd.ID)
	if err != nil {
		return err
	}

	// Use injected cluster port to uninstall
	return u.ClusterPort.Uninstall(ctx, c)
}
