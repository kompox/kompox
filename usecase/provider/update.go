package provider

import (
	"context"
	"time"

	"github.com/yaegashi/kompoxops/domain/model"
)

// UpdateInput specifies mutable fields of a provider.
type UpdateInput struct {
	// ProviderID identifies the provider to update.
	ProviderID string `json:"provider_id"`
	// Name optionally updates the provider name.
	Name *string `json:"name,omitempty"`
	// Driver optionally updates the driver.
	Driver *string `json:"driver,omitempty"`
}

// UpdateOutput wraps the updated provider.
type UpdateOutput struct {
	// Provider is the updated entity.
	Provider *model.Provider `json:"provider"`
}

// Update applies changes to a provider.
func (u *UseCase) Update(ctx context.Context, in *UpdateInput) (*UpdateOutput, error) {
	if in == nil || in.ProviderID == "" {
		return nil, model.ErrProviderInvalid
	}
	existing, err := u.Repos.Provider.Get(ctx, in.ProviderID)
	if err != nil {
		return nil, err
	}
	changed := false
	if in.Name != nil && *in.Name != "" && existing.Name != *in.Name {
		existing.Name = *in.Name
		changed = true
	}
	if in.Driver != nil && *in.Driver != "" && existing.Driver != *in.Driver {
		existing.Driver = *in.Driver
		changed = true
	}
	if changed {
		existing.UpdatedAt = time.Now().UTC()
		if err := u.Repos.Provider.Update(ctx, existing); err != nil {
			return nil, err
		}
	}
	return &UpdateOutput{Provider: existing}, nil
}
