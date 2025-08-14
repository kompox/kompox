package app

import "github.com/yaegashi/kompoxops/domain"

// Repos holds repositories needed for app use cases.
type Repos struct {
	App domain.AppRepository
}

// UseCase wires repositories needed for app use cases.
type UseCase struct {
	Repos *Repos
}
