package provider

import (
	"context"

	"github.com/kompox/kompox/domain/model"
)

// ListInput defines (future) filters for listing providers.
type ListInput struct{}

// ListOutput contains a set of providers.
type ListOutput struct {
	// Providers is the collection returned.
	Providers []*model.Provider `json:"providers"`
}

// List returns all providers.
func (u *UseCase) List(ctx context.Context, _ *ListInput) (*ListOutput, error) {
	items, err := u.Repos.Provider.List(ctx)
	if err != nil {
		return nil, err
	}
	return &ListOutput{Providers: items}, nil
}
