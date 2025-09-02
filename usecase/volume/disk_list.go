package volume

import (
	"context"
	"fmt"

	"github.com/kompox/kompox/domain/model"
)

// DiskListInput parameters for DiskList use case.
type DiskListInput struct {
	// AppID owning application identifier.
	AppID string `json:"app_id"`
	// VolumeName logical volume name within the app.
	VolumeName string `json:"volume_name"`
}

// DiskListOutput result for DiskList use case.
type DiskListOutput struct {
	// Items is the collection of volume disks.
	Items []*model.VolumeDisk `json:"items"`
}

// DiskList returns volume disks for a logical volume.
func (u *UseCase) DiskList(ctx context.Context, in *DiskListInput) (*DiskListOutput, error) {
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
	items, err := u.VolumePort.DiskList(ctx, cluster, app, in.VolumeName)
	if err != nil {
		return nil, err
	}
	return &DiskListOutput{Items: items}, nil
}
