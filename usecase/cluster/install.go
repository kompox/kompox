package cluster

import (
	"context"

	"github.com/yaegashi/kompoxops/domain/model"
)

// InstallInput represents a command to install cluster resources.
type InstallInput struct {
	ID string `json:"id"`
}

// Install installs in-cluster resources (Ingress Controller, etc.).
func (u *UseCase) Install(ctx context.Context, cmd InstallInput) error {
	if cmd.ID == "" {
		return model.ErrClusterInvalid
	}

	// Get cluster
	c, err := u.Repos.Cluster.Get(ctx, cmd.ID)
	if err != nil {
		return err
	}

	// Use injected cluster port to install
	return u.ClusterPort.Install(ctx, c)
}
