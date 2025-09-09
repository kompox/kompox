package volume

import (
	"context"
	"fmt"
)

// SnapshotDeleteInput parameters for deleting a snapshot.
type SnapshotDeleteInput struct {
	AppID        string `json:"app_id"`
	VolumeName   string `json:"volume_name"`
	SnapshotName string `json:"snapshot_name"`
}

type SnapshotDeleteOutput struct{}

// SnapshotDelete deletes a snapshot.
func (u *UseCase) SnapshotDelete(ctx context.Context, in *SnapshotDeleteInput) (*SnapshotDeleteOutput, error) {
	if in == nil || in.AppID == "" || in.VolumeName == "" || in.SnapshotName == "" {
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
	if err := u.VolumePort.SnapshotDelete(ctx, cluster, app, in.VolumeName, in.SnapshotName); err != nil {
		return nil, err
	}
	return &SnapshotDeleteOutput{}, nil
}
