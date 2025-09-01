package volume

import (
	"context"
	"fmt"

	"github.com/kompox/kompox/domain/model"
)

// DiskCreateInput parameters for DiskCreate use case.
type DiskCreateInput struct {
	// AppID owning application identifier.
	AppID string `json:"app_id"`
	// VolumeName logical volume name.
	VolumeName string `json:"volume_name"`
}

// DiskCreateOutput result for DiskCreate use case.
type DiskCreateOutput struct {
	// Disk is the created volume disk.
	Disk *model.VolumeDisk `json:"disk"`
}

// DiskCreate creates a new volume disk.
func (u *UseCase) DiskCreate(ctx context.Context, in *DiskCreateInput) (*DiskCreateOutput, error) {
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
	disk, err := u.VolumePort.VolumeDiskCreate(ctx, cluster, app, in.VolumeName)
	if err != nil {
		return nil, err
	}
	return &DiskCreateOutput{Disk: disk}, nil
}
