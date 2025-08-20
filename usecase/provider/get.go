package provider

import (
	"context"

	"github.com/yaegashi/kompoxops/domain/model"
)

// GetInput identifies the Provider to fetch.
type GetInput struct {
	// ProviderID is the identifier of the provider.
	ProviderID string `json:"provider_id"`
}

// GetOutput wraps a provider returned by Get.
type GetOutput struct {
	// Provider is the fetched provider entity.
	Provider *model.Provider `json:"provider"`
}

// Get retrieves a provider by ID.
func (u *UseCase) Get(ctx context.Context, in *GetInput) (*GetOutput, error) {
	if in == nil || in.ProviderID == "" {
		return nil, model.ErrProviderInvalid
	}
	p, err := u.Repos.Provider.Get(ctx, in.ProviderID)
	if err != nil {
		return nil, err
	}
	return &GetOutput{Provider: p}, nil
}
