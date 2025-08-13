package service

import (
	"context"

	"github.com/yaegashi/kompoxops/domain/model"
)

// ListServicesQuery placeholder (add filters later).
type ListServicesQuery struct{}

func (u *ServiceUseCase) List(ctx context.Context, _ ListServicesQuery) ([]*model.Service, error) {
	return u.Services.List(ctx)
}
