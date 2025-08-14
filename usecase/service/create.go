package service

import (
	"context"
	"time"

	"github.com/yaegashi/kompoxops/domain/model"
)

// CreateServiceCommand carries input data for creation.
type CreateServiceCommand struct {
	Name string
}

func (u *UseCase) Create(ctx context.Context, cmd CreateServiceCommand) (*model.Service, error) {
	if cmd.Name == "" {
		return nil, model.ErrServiceInvalid
	}
	now := time.Now().UTC()
	s := &model.Service{
		ID:        "", // Will be assigned by repository if empty.
		Name:      cmd.Name,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := u.Services.Create(ctx, s); err != nil {
		return nil, err
	}
	return s, nil
}
