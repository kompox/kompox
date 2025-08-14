package service

import (
	"context"
	"time"

	"github.com/yaegashi/kompoxops/domain/model"
)

// CreateInput carries input data for creation.
type CreateInput struct {
	Name string
}

func (u *UseCase) Create(ctx context.Context, cmd CreateInput) (*model.Service, error) {
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
