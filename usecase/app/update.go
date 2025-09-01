package app

import (
	"context"
	"time"

	"github.com/kompox/kompox/domain/model"
)

// UpdateInput specifies mutable fields for an App.
type UpdateInput struct {
	// AppID identifies the application to update.
	AppID string `json:"app_id"`
	// Name optionally updates the application name.
	Name *string `json:"name,omitempty"`
	// ClusterID optionally updates the cluster association.
	ClusterID *string `json:"cluster_id,omitempty"`
}

// UpdateOutput contains the updated App.
type UpdateOutput struct {
	// App is the post-update application entity.
	App *model.App `json:"app"`
}

// Update applies provided changes to the App identified by AppID.
func (u *UseCase) Update(ctx context.Context, in *UpdateInput) (*UpdateOutput, error) {
	if in == nil || in.AppID == "" {
		return nil, model.ErrAppInvalid
	}
	existing, err := u.Repos.App.Get(ctx, in.AppID)
	if err != nil {
		return nil, err
	}
	changed := false
	if in.Name != nil && *in.Name != "" && existing.Name != *in.Name {
		existing.Name = *in.Name
		changed = true
	}
	if in.ClusterID != nil && existing.ClusterID != *in.ClusterID {
		existing.ClusterID = *in.ClusterID
		changed = true
	}
	if changed {
		existing.UpdatedAt = time.Now().UTC()
		if err := u.Repos.App.Update(ctx, existing); err != nil {
			return nil, err
		}
	}
	return &UpdateOutput{App: existing}, nil
}
