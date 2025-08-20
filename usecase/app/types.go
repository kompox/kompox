package app

import (
	"github.com/yaegashi/kompoxops/domain"
	"github.com/yaegashi/kompoxops/domain/model"
)

// Repos holds repositories needed for app use cases.
type Repos struct {
	App      domain.AppRepository
	Service  domain.ServiceRepository
	Provider domain.ProviderRepository
	Cluster  domain.ClusterRepository
}

// UseCase wires repositories needed for app use cases.
type UseCase struct {
	Repos      *Repos
	VolumePort model.VolumePort
}
