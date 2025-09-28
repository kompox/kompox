package volume

import (
	"context"
	"fmt"

	"github.com/kompox/kompox/domain/model"
	"github.com/kompox/kompox/internal/naming"
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
	if err := naming.ValidateVolumeName(in.VolumeName); err != nil {
		return nil, fmt.Errorf("validate volume name: %w", err)
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
	_, err = app.FindVolume(in.VolumeName)
	if err != nil {
		return nil, fmt.Errorf("volume not defined: %w", err)
	}
	items, err := u.VolumePort.SnapshotList(ctx, cluster, app, in.VolumeName)
	if err != nil {
		return nil, err
	}
	return &SnapshotListOutput{Items: items}, nil
}
