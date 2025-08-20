package service

import (
	"context"

	"github.com/yaegashi/kompoxops/domain/model"
)

// ListInput placeholder (add filters later).
// ListInput defines optional filters for listing services.
type ListInput struct{}

// ListOutput wraps listed services.
type ListOutput struct {
	// Services is the collection returned.
	Services []*model.Service `json:"services"`
}

// List returns all services.
func (u *UseCase) List(ctx context.Context, _ *ListInput) (*ListOutput, error) {
	items, err := u.Repos.Service.List(ctx)
	if err != nil {
		return nil, err
	}
	return &ListOutput{Services: items}, nil
}
