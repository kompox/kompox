package cluster

import (
	"context"
	"time"

	"github.com/yaegashi/kompoxops/domain/model"
)

type UpdateInput struct {
	ID         string
	Name       *string
	ProviderID *string
}

func (u *UseCase) Update(ctx context.Context, cmd UpdateInput) (*model.Cluster, error) {
	if cmd.ID == "" {
		return nil, model.ErrClusterInvalid
	}
	existing, err := u.Repos.Cluster.Get(ctx, cmd.ID)
	if err != nil {
		return nil, err
	}
	changed := false
	if cmd.Name != nil && *cmd.Name != "" && existing.Name != *cmd.Name {
		existing.Name = *cmd.Name
		changed = true
	}
	if cmd.ProviderID != nil && existing.ProviderID != *cmd.ProviderID {
		existing.ProviderID = *cmd.ProviderID
		changed = true
	}
	if !changed {
		return existing, nil
	}
	existing.UpdatedAt = time.Now().UTC()
	if err := u.Repos.Cluster.Update(ctx, existing); err != nil {
		return nil, err
	}
	return existing, nil
}
