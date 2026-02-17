package nodepool

import (
	"github.com/kompox/kompox/domain"
	"github.com/kompox/kompox/domain/model"
)

// Repos holds repositories required for node pool operations.
type Repos struct {
	Workspace domain.WorkspaceRepository
	Provider  domain.ProviderRepository
	Cluster   domain.ClusterRepository
}

// UseCase wires dependencies for node pool operations.
type UseCase struct {
	Repos        *Repos
	NodePoolPort model.NodePoolPort
}
