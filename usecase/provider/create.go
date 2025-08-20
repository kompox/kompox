package provider

import (
	"context"
	"time"

	"github.com/yaegashi/kompoxops/domain/model"
)

// CreateInput contains the data to create a provider.
type CreateInput struct {
	// Name is the provider name.
	Name string `json:"name"`
	// Driver indicates the driver implementation key.
	Driver string `json:"driver"`
}

// CreateOutput wraps the created provider.
type CreateOutput struct {
	// Provider is the newly created provider entity.
	Provider *model.Provider `json:"provider"`
}

// Create persists a new provider.
func (u *UseCase) Create(ctx context.Context, in *CreateInput) (*CreateOutput, error) {
	if in == nil || in.Name == "" || in.Driver == "" {
		return nil, model.ErrProviderInvalid
	}
	now := time.Now().UTC()
	p := &model.Provider{ID: "", Name: in.Name, Driver: in.Driver, CreatedAt: now, UpdatedAt: now}
	if err := u.Repos.Provider.Create(ctx, p); err != nil {
		return nil, err
	}
	return &CreateOutput{Provider: p}, nil
}
