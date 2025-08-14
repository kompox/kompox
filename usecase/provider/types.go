package provider

import "github.com/yaegashi/kompoxops/domain"

// UseCase wires repositories needed for provider use cases.
type UseCase struct {
	Providers domain.ProviderRepository
}
