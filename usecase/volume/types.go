package volume

import (
	"github.com/kompox/kompox/domain"
	"github.com/kompox/kompox/domain/model"
)

// Repos holds repositories required for volume operations.
type Repos struct {
	Workspace domain.WorkspaceRepository
	Provider  domain.ProviderRepository
	Cluster   domain.ClusterRepository
	App       domain.AppRepository
}

// UseCase wires dependencies for volume operations.
type UseCase struct {
	Repos      *Repos
	VolumePort model.VolumePort
}
