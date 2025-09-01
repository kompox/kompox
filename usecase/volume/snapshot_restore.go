package volume

import (
	"context"
	"fmt"

	"github.com/yaegashi/kompoxops/domain/model"
)

// SnapshotRestoreInput parameters for restoring a snapshot.
type SnapshotRestoreInput struct {
	AppID        string `json:"app_id"`
	VolumeName   string `json:"volume_name"`
	SnapshotName string `json:"snapshot_name"`
}

// SnapshotRestoreOutput result for restoring a snapshot.
type SnapshotRestoreOutput struct {
	Instance *model.VolumeInstance `json:"instance"`
}

// SnapshotRestore restores a snapshot into a new volume instance (or per driver semantics).
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
	inst, err := u.VolumePort.VolumeSnapshotRestore(ctx, cluster, app, in.VolumeName, in.SnapshotName)
	if err != nil {
		return nil, err
	}
	return &SnapshotRestoreOutput{Instance: inst}, nil
}
