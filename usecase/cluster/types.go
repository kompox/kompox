package cluster

import (
	"github.com/yaegashi/kompoxops/domain"
	"github.com/yaegashi/kompoxops/domain/model"
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
