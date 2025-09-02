package tool

import (
	"github.com/kompox/kompox/domain"
	"github.com/kompox/kompox/domain/model"
)

// Repos holds repositories required for tool (maintenance runner) operations.
// It mirrors other usecase packages and wires the resources needed to
// locate the target App and its environment.
type Repos struct {
	Service  domain.ServiceRepository
	Provider domain.ProviderRepository
	Cluster  domain.ClusterRepository
	App      domain.AppRepository
}

// UseCase wires dependencies for tool operations.
// VolumePort is used to resolve disk handles and related metadata
// when preparing PV/PVC bindings for maintenance containers.
type UseCase struct {
	Repos      *Repos
	VolumePort model.VolumePort
}
