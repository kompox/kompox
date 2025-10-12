package workspace

import "github.com/kompox/kompox/domain"

// Repos holds repositories needed for workspace use cases.
type Repos struct {
	Workspace domain.WorkspaceRepository
}

// UseCase wires repositories needed for workspace use cases.
type UseCase struct {
	Repos *Repos
}
