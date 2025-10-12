package domain

import (
	"context"
	"errors"

	"github.com/kompox/kompox/domain/model"
)

// WorkspaceRepository stores and retrieves Workspace aggregates.
type WorkspaceRepository interface {
	Create(ctx context.Context, s *model.Workspace) error
	Get(ctx context.Context, id string) (*model.Workspace, error)
	List(ctx context.Context) ([]*model.Workspace, error)
	Update(ctx context.Context, s *model.Workspace) error
	Delete(ctx context.Context, id string) error
}

// ProviderRepository stores and retrieves Provider aggregates.
type ProviderRepository interface {
	Create(ctx context.Context, p *model.Provider) error
	Get(ctx context.Context, id string) (*model.Provider, error)
	List(ctx context.Context) ([]*model.Provider, error)
	Update(ctx context.Context, p *model.Provider) error
	Delete(ctx context.Context, id string) error
}

// ClusterRepository stores and retrieves Cluster aggregates.
type ClusterRepository interface {
	Create(ctx context.Context, c *model.Cluster) error
	Get(ctx context.Context, id string) (*model.Cluster, error)
	List(ctx context.Context) ([]*model.Cluster, error)
	Update(ctx context.Context, c *model.Cluster) error
	Delete(ctx context.Context, id string) error
}

// AppRepository stores and retrieves App aggregates.
type AppRepository interface {
	Create(ctx context.Context, a *model.App) error
	Get(ctx context.Context, id string) (*model.App, error)
	List(ctx context.Context) ([]*model.App, error)
	Update(ctx context.Context, a *model.App) error
	Delete(ctx context.Context, id string) error
}

// UnitOfWork coordinates transactional operations.
type UnitOfWork interface {
	Do(ctx context.Context, fn func(repos *Repositories) error) error
}

// Repositories groups repository interfaces for use inside UnitOfWork.
type Repositories struct {
	Workspace WorkspaceRepository
	Provider  ProviderRepository
	Cluster   ClusterRepository
	App       AppRepository
}

var ErrUnitOfWorkNotSupported = errors.New("unit of work not supported (memory)")
