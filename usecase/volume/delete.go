package volume

import (
	"context"
	"fmt"
)

// DeleteInput parameters.
type DeleteInput struct {
	// AppID owning application identifier.
	AppID string `json:"app_id"`
	// VolumeName logical volume name.
	VolumeName string `json:"volume_name"`
	// VolumeInstanceName target instance name.
	VolumeInstanceName string `json:"volume_instance_name"`
}
type DeleteOutput struct{}

// Delete deletes a volume instance.
func (u *UseCase) Delete(ctx context.Context, in *DeleteInput) (*DeleteOutput, error) {
	if in == nil || in.AppID == "" || in.VolumeName == "" || in.VolumeInstanceName == "" {
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
	if err := u.VolumePort.VolumeInstanceDelete(ctx, cluster, app, in.VolumeName, in.VolumeInstanceName); err != nil {
		return nil, err
	}
	return &DeleteOutput{}, nil
}
