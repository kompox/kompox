package cluster

import (
	"context"

	"github.com/yaegashi/kompoxops/domain/model"
)

// DeprovisionInput represents a command to deprovision a cluster.
type DeprovisionInput struct {
	ID string `json:"id"`
}

// Deprovision deprovisions a cluster.
func (u *UseCase) Deprovision(ctx context.Context, cmd DeprovisionInput) error {
	if cmd.ID == "" {
		return model.ErrClusterInvalid
	}

	// Get cluster
	c, err := u.Clusters.Get(ctx, cmd.ID)
	if err != nil {
		return err
	}

	// Use injected cluster port to deprovision
	return u.ClusterPort.Deprovision(ctx, c)
}
