package app

import (
	"context"

	"github.com/yaegashi/kompoxops/domain/model"
)

// ListInput defines optional filters for listing Apps (reserved for future use).
type ListInput struct{}

// ListOutput contains the resulting collection of Apps.
type ListOutput struct {
	// Apps is the list of applications.
	Apps []*model.App `json:"apps"`
}

// List returns all applications. Future filter fields will be honored when added.
func (u *UseCase) List(ctx context.Context, _ *ListInput) (*ListOutput, error) {
	apps, err := u.Repos.App.List(ctx)
	if err != nil {
		return nil, err
	}
	return &ListOutput{Apps: apps}, nil
}
