package volume

import (
	"context"
	"fmt"

	"github.com/kompox/kompox/domain/model"
)

// SnapshotCreateInput parameters for creating a snapshot.
type SnapshotCreateInput struct {
	AppID        string `json:"app_id"`
	VolumeName   string `json:"volume_name"`
	SnapshotName string `json:"snapshot_name,omitempty"` // Target snapshot name (empty for auto-generated)
	Source       string `json:"source,omitempty"`       // Source for snapshot creation (opaque, empty for default/assigned disk)
}

// SnapshotCreateOutput result for creating a snapshot.
type SnapshotCreateOutput struct {
	Snapshot *model.VolumeSnapshot `json:"snapshot"`
}

// SnapshotCreate creates a snapshot for a given volume source.
func (u *UseCase) SnapshotCreate(ctx context.Context, in *SnapshotCreateInput) (*SnapshotCreateOutput, error) {
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
	_, err = app.FindVolume(in.VolumeName)
	if err != nil {
		return nil, fmt.Errorf("volume not defined: %w", err)
	}
	// Pass snapName and source directly to driver per K4x-ADR-003
	snap, err := u.VolumePort.SnapshotCreate(ctx, cluster, app, in.VolumeName, in.SnapshotName, in.Source)
	if err != nil {
		return nil, err
	}
	return &SnapshotCreateOutput{Snapshot: snap}, nil
}
