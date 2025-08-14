package app

import "github.com/yaegashi/kompoxops/domain"

// UseCase wires repositories needed for app use cases.
type UseCase struct {
	Apps domain.AppRepository
}
