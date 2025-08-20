package service

import (
	"context"
	"time"

	"github.com/yaegashi/kompoxops/domain/model"
)

// CreateInput carries input data for creation.
// CreateInput contains data to create a service.
type CreateInput struct {
	// Name is the service name.
	Name string `json:"name"`
}

// CreateOutput wraps the created service.
type CreateOutput struct {
	// Service is the newly created entity.
	Service *model.Service `json:"service"`
}

// Create persists a new service.
func (u *UseCase) Create(ctx context.Context, in *CreateInput) (*CreateOutput, error) {
	if in == nil || in.Name == "" {
		return nil, model.ErrServiceInvalid
	}
	now := time.Now().UTC()
	s := &model.Service{ID: "", Name: in.Name, CreatedAt: now, UpdatedAt: now}
	if err := u.Repos.Service.Create(ctx, s); err != nil {
		return nil, err
	}
	return &CreateOutput{Service: s}, nil
}
