package domain

import (
	"context"
	"errors"

	"github.com/yaegashi/kompoxops/domain/model"
)

// ServiceRepository stores and retrieves Service aggregates.
type ServiceRepository interface {
	Create(ctx context.Context, s *model.Service) error
	Get(ctx context.Context, id string) (*model.Service, error)
	List(ctx context.Context) ([]*model.Service, error)
	Update(ctx context.Context, s *model.Service) error
	Delete(ctx context.Context, id string) error
}

// UnitOfWork coordinates transactional operations.
type UnitOfWork interface {
	Do(ctx context.Context, fn func(repos *Repositories) error) error
}

// Repositories groups repository interfaces for use inside UnitOfWork.
type Repositories struct {
	Service ServiceRepository
}

var ErrUnitOfWorkNotSupported = errors.New("unit of work not supported (inmem)")
