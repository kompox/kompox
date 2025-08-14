package service

import (
	"context"

	"github.com/yaegashi/kompoxops/domain/model"
)

// Get retrieves a service by ID.
func (u *UseCase) Get(ctx context.Context, id string) (*model.Service, error) {
	if id == "" {
		return nil, model.ErrServiceInvalid
	}
	return u.Repos.Service.Get(ctx, id)
}
