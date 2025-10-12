package dns

import (
	"github.com/kompox/kompox/domain"
	"github.com/kompox/kompox/domain/model"
)

// Repos bundles repository dependencies used by DNS use cases.
type Repos struct {
	Workspace domain.WorkspaceRepository
	Provider  domain.ProviderRepository
	Cluster   domain.ClusterRepository
	App       domain.AppRepository
}

// UseCase provides application logic for DNS operations.
type UseCase struct {
	Repos       *Repos
	ClusterPort model.ClusterPort
}
