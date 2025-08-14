package provider

import "github.com/yaegashi/kompoxops/domain"

// Repos holds repositories needed for provider use cases.
type Repos struct {
	Provider domain.ProviderRepository
}

// UseCase wires repositories needed for provider use cases.
type UseCase struct {
	Repos *Repos
}
