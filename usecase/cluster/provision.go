package cluster

import (
	"context"

	"github.com/yaegashi/kompoxops/domain/model"
)

// ProvisionInput represents a command to provision a cluster.
type ProvisionInput struct {
	ID string `json:"id"`
}

// Provision provisions a cluster.
func (u *UseCase) Provision(ctx context.Context, cmd ProvisionInput) error {
	if cmd.ID == "" {
		return model.ErrClusterInvalid
	}

	// Get cluster
	c, err := u.Repos.Cluster.Get(ctx, cmd.ID)
	if err != nil {
		return err
	}

	// Use injected cluster port to provision
	return u.ClusterPort.Provision(ctx, c)
}
