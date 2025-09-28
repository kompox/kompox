package volume

import (
	"context"
	"fmt"

	"github.com/kompox/kompox/domain/model"
)

// DiskCreateInput parameters for DiskCreate use case.
type DiskCreateInput struct {
	// AppID owning application identifier.
	AppID string `json:"app_id"`
	// VolumeName logical volume name.
	VolumeName string `json:"volume_name"`
	// DiskName target disk name (empty for auto-generated).
	DiskName string `json:"disk_name,omitempty"`
	// Zone overrides app.deployment.zone when specified.
	Zone string `json:"zone,omitempty"`
	// Options overrides/merges with app.volumes.options when specified.
	Options map[string]any `json:"options,omitempty"`
	// Source specifies the source for disk creation (opaque string as per K4x-ADR-003).
	// Empty means create an empty disk. The interpretation is delegated to the provider driver.
	Source string `json:"source,omitempty"`
}

// DiskCreateOutput result for DiskCreate use case.
type DiskCreateOutput struct {
	// Disk is the created volume disk.
	Disk *model.VolumeDisk `json:"disk"`
}

// DiskCreate creates a new volume disk.
func (u *UseCase) DiskCreate(ctx context.Context, in *DiskCreateInput) (*DiskCreateOutput, error) {
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

	// Build options based on input
	var opts []model.VolumeDiskCreateOption
	if in.Zone != "" {
		opts = append(opts, model.WithVolumeDiskCreateZone(in.Zone))
	}
	if in.Options != nil {
		opts = append(opts, model.WithVolumeDiskCreateOptions(in.Options))
	}
	// Source is now passed directly to Driver per K4x-ADR-003

	disk, err := u.VolumePort.DiskCreate(ctx, cluster, app, in.VolumeName, in.DiskName, in.Source, opts...)
	if err != nil {
		return nil, err
	}
	return &DiskCreateOutput{Disk: disk}, nil
}
