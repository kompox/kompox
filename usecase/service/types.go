package service

import "github.com/kompox/kompox/domain"

// Repos holds repositories needed for service use cases.
type Repos struct {
	Service domain.ServiceRepository
}

// UseCase wires repositories needed for service use cases.
type UseCase struct {
	Repos *Repos
}
