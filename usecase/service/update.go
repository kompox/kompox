package service

import (
	"context"
	"time"

	"github.com/yaegashi/kompoxops/domain/model"
)

// UpdateInput specifies service fields that can be changed.
type UpdateInput struct {
	// ServiceID identifies the service.
	ServiceID string `json:"service_id"`
	// Name optionally updates the name.
	Name *string `json:"name,omitempty"`
}

// UpdateOutput wraps the updated service.
type UpdateOutput struct {
	// Service is the updated entity.
	Service *model.Service `json:"service"`
}

// Update applies provided changes to a service.
func (u *UseCase) Update(ctx context.Context, in *UpdateInput) (*UpdateOutput, error) {
	if in == nil || in.ServiceID == "" {
		return nil, model.ErrServiceInvalid
	}
	existing, err := u.Repos.Service.Get(ctx, in.ServiceID)
	if err != nil {
		return nil, err
	}
	changed := false
	if in.Name != nil && *in.Name != "" && existing.Name != *in.Name {
		existing.Name = *in.Name
		changed = true
	}
	if changed {
		existing.UpdatedAt = time.Now().UTC()
		if err := u.Repos.Service.Update(ctx, existing); err != nil {
			return nil, err
		}
	}
	return &UpdateOutput{Service: existing}, nil
}
