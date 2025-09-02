package volume

import (
	"context"
	"fmt"
)

// DiskDeleteInput parameters.
type DiskDeleteInput struct {
	// AppID owning application identifier.
	AppID string `json:"app_id"`
	// VolumeName logical volume name.
	VolumeName string `json:"volume_name"`
	// DiskName target disk name.
	DiskName string `json:"disk_name"`
}

type DiskDeleteOutput struct{}

// DiskDelete deletes a volume disk.
func (u *UseCase) DiskDelete(ctx context.Context, in *DiskDeleteInput) (*DiskDeleteOutput, error) {
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
	ok := false
	for _, v := range app.Volumes {
		if v.Name == in.VolumeName {
			ok = true
			break
		}
	}
	if !ok {
		return nil, fmt.Errorf("volume not defined: %s", in.VolumeName)
	}
	if err := u.VolumePort.DiskDelete(ctx, cluster, app, in.VolumeName, in.DiskName); err != nil {
		return nil, err
	}
	return &DiskDeleteOutput{}, nil
}
