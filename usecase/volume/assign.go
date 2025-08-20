package volume

import (
	"context"
	"fmt"
)

// AssignInput parameters.
type AssignInput struct {
	AppID              string
	VolumeName         string
	VolumeInstanceName string
}

// Assign sets the specified instance as assigned for the logical volume.
func (u *UseCase) Assign(ctx context.Context, in AssignInput) error {
	if in.AppID == "" || in.VolumeName == "" || in.VolumeInstanceName == "" {
		return fmt.Errorf("missing parameters")
	}
	app, err := u.Repos.App.Get(ctx, in.AppID)
	if err != nil {
		return err
	}
	if app == nil {
		return fmt.Errorf("app not found: %s", in.AppID)
	}
	cluster, err := u.Repos.Cluster.Get(ctx, app.ClusterID)
	if err != nil {
		return err
	}
	if cluster == nil {
		return fmt.Errorf("cluster not found: %s", app.ClusterID)
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
		return fmt.Errorf("volume not defined: %s", in.VolumeName)
	}
	return u.VolumePort.VolumeInstanceAssign(ctx, cluster, app, in.VolumeName, in.VolumeInstanceName)
}
