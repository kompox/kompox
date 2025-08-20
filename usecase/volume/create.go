package volume

import (
	"context"
	"fmt"

	"github.com/yaegashi/kompoxops/domain/model"
)

// CreateInput parameters for Create use case.
type CreateInput struct {
	// AppID owning application identifier.
	AppID string `json:"app_id"`
	// VolumeName logical volume name.
	VolumeName string `json:"volume_name"`
}

// CreateOutput result for Create use case.
type CreateOutput struct {
	// Instance is the created volume instance.
	Instance *model.AppVolumeInstance `json:"instance"`
}

// Create creates a new volume instance.
func (u *UseCase) Create(ctx context.Context, in *CreateInput) (*CreateOutput, error) {
	if in == nil || in.AppID == "" || in.VolumeName == "" {
		return nil, fmt.Errorf("missing parameters")
	}
	app, err := u.Repos.App.Get(ctx, in.AppID)
	if err != nil {
		return nil, err
	}
	if app == nil {
		return nil, fmt.Errorf("app not found: %s", in.AppID)
	}
	cluster, err := u.Repos.Cluster.Get(ctx, app.ClusterID)
	if err != nil {
		return nil, err
	}
	if cluster == nil {
		return nil, fmt.Errorf("cluster not found: %s", app.ClusterID)
	}
	// Validate logical volume exists
	exists := false
	for _, v := range app.Volumes {
		if v.Name == in.VolumeName {
			exists = true
			break
		}
	}
	if !exists {
		return nil, fmt.Errorf("volume not defined: %s", in.VolumeName)
	}
	inst, err := u.VolumePort.VolumeInstanceCreate(ctx, cluster, app, in.VolumeName)
	if err != nil {
		return nil, err
	}
	return &CreateOutput{Instance: inst}, nil
}
