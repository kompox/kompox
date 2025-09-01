package volume

import (
	"context"
	"fmt"

	"github.com/kompox/kompox/domain/model"
)

// SnapshotListInput parameters for listing snapshots.
type SnapshotListInput struct {
	AppID      string `json:"app_id"`
	VolumeName string `json:"volume_name"`
}

// SnapshotListOutput result of listing snapshots.
type SnapshotListOutput struct {
	Items []*model.VolumeSnapshot `json:"items"`
}

// SnapshotList returns snapshots for a logical volume.
func (u *UseCase) SnapshotList(ctx context.Context, in *SnapshotListInput) (*SnapshotListOutput, error) {
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
	items, err := u.VolumePort.VolumeSnapshotList(ctx, cluster, app, in.VolumeName)
	if err != nil {
		return nil, err
	}
	return &SnapshotListOutput{Items: items}, nil
}
