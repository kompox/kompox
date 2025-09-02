package volume

import (
	"context"
	"fmt"

	"github.com/kompox/kompox/domain/model"
)

// SnapshotCreateInput parameters for creating a snapshot.
type SnapshotCreateInput struct {
	AppID      string `json:"app_id"`
	VolumeName string `json:"volume_name"`
	DiskName   string `json:"disk_name"`
}

// SnapshotCreateOutput result for creating a snapshot.
type SnapshotCreateOutput struct {
	Snapshot *model.VolumeSnapshot `json:"snapshot"`
}

// SnapshotCreate creates a snapshot for a given volume disk.
func (u *UseCase) SnapshotCreate(ctx context.Context, in *SnapshotCreateInput) (*SnapshotCreateOutput, error) {
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
	snap, err := u.VolumePort.SnapshotCreate(ctx, cluster, app, in.VolumeName, in.DiskName)
	if err != nil {
		return nil, err
	}
	return &SnapshotCreateOutput{Snapshot: snap}, nil
}
