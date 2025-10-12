package workspace

import (
	"context"

	"github.com/kompox/kompox/domain/model"
)

// ListInput placeholder (add filters later).
// ListInput defines optional filters for listing workspaces.
type ListInput struct{}

// ListOutput wraps listed workspaces.
type ListOutput struct {
	// Workspaces is the collection returned.
	Workspaces []*model.Workspace `json:"workspaces"`
}

// List returns all workspaces.
func (u *UseCase) List(ctx context.Context, _ *ListInput) (*ListOutput, error) {
	items, err := u.Repos.Workspace.List(ctx)
	if err != nil {
		return nil, err
	}
	return &ListOutput{Workspaces: items}, nil
}
