package app

import (
	"context"
	"time"

	"github.com/yaegashi/kompoxops/domain/model"
)

// CreateInput contains the data required to create a new App resource.
// All fields are required unless otherwise noted.
type CreateInput struct {
	// Name is the unique application name.
	Name string `json:"name"`
	// ClusterID references the cluster on which the app will run.
	ClusterID string `json:"cluster_id"`
}

// CreateOutput contains the created App.
type CreateOutput struct {
	// App is the persisted application entity.
	App *model.App `json:"app"`
}

// Create persists a new App. Returns ErrAppInvalid when mandatory fields are empty.
func (u *UseCase) Create(ctx context.Context, in *CreateInput) (*CreateOutput, error) {
	if in == nil || in.Name == "" {
		return nil, model.ErrAppInvalid
	}
	now := time.Now().UTC()
	a := &model.App{ID: "", Name: in.Name, ClusterID: in.ClusterID, CreatedAt: now, UpdatedAt: now}
	if err := u.Repos.App.Create(ctx, a); err != nil {
		return nil, err
	}
	return &CreateOutput{App: a}, nil
}
