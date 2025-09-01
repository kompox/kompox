package cluster

import (
	"github.com/kompox/kompox/domain"
	"github.com/kompox/kompox/domain/model"
)

// Repos holds repositories needed for cluster use cases.
type Repos struct {
	Service  domain.ServiceRepository
	Cluster  domain.ClusterRepository
	Provider domain.ProviderRepository
}

// UseCase wires repositories and ports needed for cluster use cases.
type UseCase struct {
	Repos       *Repos
	ClusterPort model.ClusterPort
}
