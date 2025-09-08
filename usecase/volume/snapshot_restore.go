package volume

import (
	"context"
	"fmt"

	"github.com/kompox/kompox/domain/model"
)

// SnapshotRestoreInput parameters for restoring a snapshot.
type SnapshotRestoreInput struct {
	AppID        string `json:"app_id"`
	VolumeName   string `json:"volume_name"`
	SnapshotName string `json:"snapshot_name"`
	// Zone overrides app.deployment.zone when specified.
	Zone string `json:"zone,omitempty"`
	// Options overrides/merges with app.volumes.options when specified.
	Options map[string]any `json:"options,omitempty"`
}

// SnapshotRestoreOutput result for restoring a snapshot.
type SnapshotRestoreOutput struct {
	Disk *model.VolumeDisk `json:"disk"`
}

// SnapshotRestore restores a snapshot into a new volume disk (or per driver semantics).
func (u *UseCase) SnapshotRestore(ctx context.Context, in *SnapshotRestoreInput) (*SnapshotRestoreOutput, error) {
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

	// Build options based on input
	var opts []model.VolumeSnapshotRestoreOption
	if in.Zone != "" {
		opts = append(opts, model.WithVolumeSnapshotRestoreZone(in.Zone))
	}
	if in.Options != nil {
		opts = append(opts, model.WithVolumeSnapshotRestoreOptions(in.Options))
	}

	disk, err := u.VolumePort.SnapshotRestore(ctx, cluster, app, in.VolumeName, in.SnapshotName, opts...)
	if err != nil {
		return nil, err
	}
	return &SnapshotRestoreOutput{Disk: disk}, nil
}
