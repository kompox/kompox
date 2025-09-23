package secret

import "github.com/kompox/kompox/domain"

// Repos groups repositories required by secret use cases.
type Repos struct {
	App      domain.AppRepository
	Service  domain.ServiceRepository
	Provider domain.ProviderRepository
	Cluster  domain.ClusterRepository
}

// UseCase provides secret management operations (env / pull overrides).
type UseCase struct {
	Repos *Repos
}
