package cluster

import (
	"github.com/yaegashi/kompoxops/domain"
	"github.com/yaegashi/kompoxops/domain/model"
)

// UseCase wires repositories and ports needed for cluster use cases.
type UseCase struct {
	Clusters    domain.ClusterRepository
	Providers   domain.ProviderRepository
	ClusterPort model.ClusterPort
}
