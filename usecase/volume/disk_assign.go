package volume

import (
	"context"
	"fmt"
)

// DiskAssignInput parameters.
type DiskAssignInput struct {
	// AppID owning application identifier.
	AppID string `json:"app_id"`
	// VolumeName logical volume name.
	VolumeName string `json:"volume_name"`
	// DiskName disk name to assign.
	DiskName string `json:"disk_name"`
}

type DiskAssignOutput struct{}

// DiskAssign sets the specified disk as assigned for the logical volume.
func (u *UseCase) DiskAssign(ctx context.Context, in *DiskAssignInput) (*DiskAssignOutput, error) {
	if in == nil || in.AppID == "" || in.VolumeName == "" || in.DiskName == "" {
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
	_, err = app.FindVolume(in.VolumeName)
	if err != nil {
		return nil, fmt.Errorf("volume not defined: %w", err)
	}
	if err := u.VolumePort.DiskAssign(ctx, cluster, app, in.VolumeName, in.DiskName); err != nil {
		return nil, err
	}
	return &DiskAssignOutput{}, nil
}
