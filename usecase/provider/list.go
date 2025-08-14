package provider

import (
	"context"

	"github.com/yaegashi/kompoxops/domain/model"
)

func (u *UseCase) List(ctx context.Context) ([]*model.Provider, error) {
	return u.Repos.Provider.List(ctx)
}
