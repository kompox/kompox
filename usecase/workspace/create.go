package workspace

import (
	"context"
	"time"

	"github.com/kompox/kompox/domain/model"
)

// CreateInput carries input data for creation.
// CreateInput contains data to create a workspace.
type CreateInput struct {
	// Name is the workspace name.
	Name string `json:"name"`
}

// CreateOutput wraps the created workspace.
type CreateOutput struct {
	// Workspace is the newly created entity.
	Workspace *model.Workspace `json:"workspace"`
}

// Create persists a new workspace.
func (u *UseCase) Create(ctx context.Context, in *CreateInput) (*CreateOutput, error) {
	if in == nil || in.Name == "" {
		return nil, model.ErrWorkspaceInvalid
	}
	now := time.Now().UTC()
	s := &model.Workspace{ID: "", Name: in.Name, CreatedAt: now, UpdatedAt: now}
	if err := u.Repos.Workspace.Create(ctx, s); err != nil {
		return nil, err
	}
	return &CreateOutput{Workspace: s}, nil
}
