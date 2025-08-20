package volume

import (
	"github.com/yaegashi/kompoxops/domain"
	"github.com/yaegashi/kompoxops/domain/model"
)

// Repos holds repositories required for volume operations.
type Repos struct {
	Service  domain.ServiceRepository
	Provider domain.ProviderRepository
	Cluster  domain.ClusterRepository
	App      domain.AppRepository
}

// UseCase wires dependencies for volume operations.
type UseCase struct {
	Repos      *Repos
	VolumePort model.VolumePort
}
