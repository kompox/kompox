package workspace

import (
	"context"
	"time"

	"github.com/kompox/kompox/domain/model"
)

// UpdateInput specifies workspace fields that can be changed.
type UpdateInput struct {
	// WorkspaceID identifies the workspace.
	WorkspaceID string `json:"workspace_id"`
	// Name optionally updates the name.
	Name *string `json:"name,omitempty"`
}

// UpdateOutput wraps the updated workspace.
type UpdateOutput struct {
	// Workspace is the updated entity.
	Workspace *model.Workspace `json:"workspace"`
}

// Update applies provided changes to a workspace.
func (u *UseCase) Update(ctx context.Context, in *UpdateInput) (*UpdateOutput, error) {
	if in == nil || in.WorkspaceID == "" {
		return nil, model.ErrWorkspaceInvalid
	}
	existing, err := u.Repos.Workspace.Get(ctx, in.WorkspaceID)
	if err != nil {
		return nil, err
	}
	changed := false
	if in.Name != nil && *in.Name != "" && existing.Name != *in.Name {
		existing.Name = *in.Name
		changed = true
	}
	if changed {
		existing.UpdatedAt = time.Now().UTC()
		if err := u.Repos.Workspace.Update(ctx, existing); err != nil {
			return nil, err
		}
	}
	return &UpdateOutput{Workspace: existing}, nil
}
