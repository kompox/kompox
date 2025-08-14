package app

import (
	"context"

	"github.com/yaegashi/kompoxops/domain/model"
)

func (u *UseCase) List(ctx context.Context) ([]*model.App, error) { return u.Repos.App.List(ctx) }
