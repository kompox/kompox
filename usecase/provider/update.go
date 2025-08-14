package provider

import (
	"context"
	"time"

	"github.com/yaegashi/kompoxops/domain/model"
)

type UpdateInput struct {
	ID     string
	Name   *string
	Driver *string
}

func (u *UseCase) Update(ctx context.Context, cmd UpdateInput) (*model.Provider, error) {
	if cmd.ID == "" {
		return nil, model.ErrProviderInvalid
	}
	existing, err := u.Providers.Get(ctx, cmd.ID)
	if err != nil {
		return nil, err
	}
	changed := false
	if cmd.Name != nil && *cmd.Name != "" && existing.Name != *cmd.Name {
		existing.Name = *cmd.Name
		changed = true
	}
	if cmd.Driver != nil && *cmd.Driver != "" && existing.Driver != *cmd.Driver {
		existing.Driver = *cmd.Driver
		changed = true
	}
	if !changed {
		return existing, nil
	}
	existing.UpdatedAt = time.Now().UTC()
	if err := u.Providers.Update(ctx, existing); err != nil {
		return nil, err
	}
	return existing, nil
}
