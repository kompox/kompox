package volume

import (
	"context"
	"fmt"
)

// DeleteInput parameters.
type DeleteInput struct {
	AppID              string
	VolumeName         string
	VolumeInstanceName string
}

// Delete deletes a volume instance.
func (u *UseCase) Delete(ctx context.Context, in DeleteInput) error {
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
	return u.VolumePort.VolumeInstanceDelete(ctx, cluster, app, in.VolumeName, in.VolumeInstanceName)
}
