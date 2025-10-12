package workspace

import (
	"context"
)

// DeleteInput identifies the workspace to delete.
type DeleteInput struct {
	// WorkspaceID is the workspace identifier.
	WorkspaceID string `json:"workspace_id"`
}

// DeleteOutput is empty because delete has no return entity.
type DeleteOutput struct{}

// Delete removes a workspace; empty ID is a no-op.
func (u *UseCase) Delete(ctx context.Context, in *DeleteInput) (*DeleteOutput, error) {
	if in == nil || in.WorkspaceID == "" { // idempotent no-op
		return &DeleteOutput{}, nil
	}
	if err := u.Repos.Workspace.Delete(ctx, in.WorkspaceID); err != nil {
		return nil, err
	}
	return &DeleteOutput{}, nil
}
