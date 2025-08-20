package volume

import (
	"context"
	"fmt"

	"github.com/yaegashi/kompoxops/domain/model"
)

// ListInput parameters for List use case.
type ListInput struct {
	AppID      string
	VolumeName string
}

// ListOutput result for List use case.
type ListOutput struct {
	Items []*model.AppVolumeInstance
}

// List returns volume instances for a logical volume.
func (u *UseCase) List(ctx context.Context, in ListInput) (*ListOutput, error) {
	if in.AppID == "" || in.VolumeName == "" {
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
	// Ensure the logical volume exists
	found := false
	for _, v := range app.Volumes {
		if v.Name == in.VolumeName {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("volume not defined: %s", in.VolumeName)
	}
	items, err := u.VolumePort.VolumeInstanceList(ctx, cluster, app, in.VolumeName)
	if err != nil {
		return nil, err
	}
	return &ListOutput{Items: items}, nil
}
