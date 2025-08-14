package provider

import (
	"context"
	"time"

	"github.com/yaegashi/kompoxops/domain/model"
)

type CreateCommand struct {
	Name   string
	Driver string
}

func (u *UseCase) Create(ctx context.Context, cmd CreateCommand) (*model.Provider, error) {
	if cmd.Name == "" || cmd.Driver == "" {
		return nil, model.ErrProviderInvalid
	}
	now := time.Now().UTC()
	p := &model.Provider{ID: "", Name: cmd.Name, Driver: cmd.Driver, CreatedAt: now, UpdatedAt: now}
	if err := u.Providers.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}
