package app

import (
	"context"

	"github.com/yaegashi/kompoxops/domain/model"
)

func (u *UseCase) Get(ctx context.Context, id string) (*model.App, error) {
	if id == "" {
		return nil, model.ErrAppInvalid
	}
	return u.Apps.Get(ctx, id)
}
