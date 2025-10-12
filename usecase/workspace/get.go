package workspace

import (
	"context"

	"github.com/kompox/kompox/domain/model"
)

// GetInput identifies the workspace to fetch.
type GetInput struct {
	// WorkspaceID is the identifier of the workspace.
	WorkspaceID string `json:"workspace_id"`
}

// GetOutput wraps the retrieved workspace.
type GetOutput struct {
	// Workspace is the fetched entity.
	Workspace *model.Workspace `json:"workspace"`
}

// Get retrieves a workspace by ID.
func (u *UseCase) Get(ctx context.Context, in *GetInput) (*GetOutput, error) {
	if in == nil || in.WorkspaceID == "" {
		return nil, model.ErrWorkspaceInvalid
	}
	s, err := u.Repos.Workspace.Get(ctx, in.WorkspaceID)
	if err != nil {
		return nil, err
	}
	return &GetOutput{Workspace: s}, nil
}
