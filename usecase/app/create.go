package app

import (
	"context"
	"time"

	"github.com/yaegashi/kompoxops/domain/model"
)

type CreateInput struct {
	Name      string
	ClusterID string
}

func (u *UseCase) Create(ctx context.Context, cmd CreateInput) (*model.App, error) {
	if cmd.Name == "" {
		return nil, model.ErrAppInvalid
	}
	now := time.Now().UTC()
	a := &model.App{ID: "", Name: cmd.Name, ClusterID: cmd.ClusterID, CreatedAt: now, UpdatedAt: now}
	if err := u.Repos.App.Create(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}
