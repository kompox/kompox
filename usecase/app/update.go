package app

import (
	"context"
	"time"

	"github.com/yaegashi/kompoxops/domain/model"
)

type UpdateInput struct {
	ID        string
	Name      *string
	ClusterID *string
}

func (u *UseCase) Update(ctx context.Context, cmd UpdateInput) (*model.App, error) {
	if cmd.ID == "" {
		return nil, model.ErrAppInvalid
	}
	existing, err := u.Repos.App.Get(ctx, cmd.ID)
	if err != nil {
		return nil, err
	}
	changed := false
	if cmd.Name != nil && *cmd.Name != "" && existing.Name != *cmd.Name {
		existing.Name = *cmd.Name
		changed = true
	}
	if cmd.ClusterID != nil && existing.ClusterID != *cmd.ClusterID {
		existing.ClusterID = *cmd.ClusterID
		changed = true
	}
	if !changed {
		return existing, nil
	}
	existing.UpdatedAt = time.Now().UTC()
	if err := u.Repos.App.Update(ctx, existing); err != nil {
		return nil, err
	}
	return existing, nil
}
