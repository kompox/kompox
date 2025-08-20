package service

import (
	"context"

	"github.com/yaegashi/kompoxops/domain/model"
)

// GetInput identifies the service to fetch.
type GetInput struct {
	// ServiceID is the identifier of the service.
	ServiceID string `json:"service_id"`
}

// GetOutput wraps the retrieved service.
type GetOutput struct {
	// Service is the fetched entity.
	Service *model.Service `json:"service"`
}

// Get retrieves a service by ID.
func (u *UseCase) Get(ctx context.Context, in *GetInput) (*GetOutput, error) {
	if in == nil || in.ServiceID == "" {
		return nil, model.ErrServiceInvalid
	}
	s, err := u.Repos.Service.Get(ctx, in.ServiceID)
	if err != nil {
		return nil, err
	}
	return &GetOutput{Service: s}, nil
}
