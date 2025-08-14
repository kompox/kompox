package service

import "github.com/yaegashi/kompoxops/domain"

// UseCase wires repositories needed for service use cases.
type UseCase struct {
	Services domain.ServiceRepository
}
