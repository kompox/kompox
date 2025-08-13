package provider

import (
	"context"

	"github.com/yaegashi/kompoxops/domain/model"
)

func (u *UseCase) Get(ctx context.Context, id string) (*model.Provider, error) {
	if id == "" {
		return nil, model.ErrProviderInvalid
	}
	return u.Providers.Get(ctx, id)
}
