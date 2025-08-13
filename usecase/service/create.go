package service

import (
	"context"
	"time"

	"github.com/yaegashi/kompoxops/domain"
	"github.com/yaegashi/kompoxops/domain/model"
)

// ServiceUseCase wires repositories needed for service use cases.
type ServiceUseCase struct {
	Services domain.ServiceRepository
}

// CreateServiceCommand carries input data for creation.
type CreateServiceCommand struct {
	Name       string
	ProviderID string
}

func (u *ServiceUseCase) Create(ctx context.Context, cmd CreateServiceCommand) (*model.Service, error) {
	if cmd.Name == "" {
		return nil, model.ErrServiceInvalid
	}
	now := time.Now().UTC()
	s := &model.Service{
		ID:         "", // Will be assigned by repository if empty.
		Name:       cmd.Name,
		ProviderID: cmd.ProviderID,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := u.Services.Create(ctx, s); err != nil {
		return nil, err
	}
	return s, nil
}
