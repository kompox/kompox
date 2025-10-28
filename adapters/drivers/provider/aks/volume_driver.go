package aks

import (
	"context"

	"github.com/kompox/kompox/domain/model"
)

// driverVolume abstracts volume operations for different volume types (disk, files, etc.).
// Each volume type has its own implementation that knows how to interact with
// the corresponding Azure service (Managed Disks, Azure Files, etc.).
type driverVolume interface {
	// DiskList returns a list of disks of the specified logical volume.
	DiskList(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, opts ...model.VolumeDiskListOption) ([]*model.VolumeDisk, error)

	// DiskCreate creates a disk of the specified logical volume.
	DiskCreate(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, diskName string, source string, opts ...model.VolumeDiskCreateOption) (*model.VolumeDisk, error)

	// DiskDelete deletes a disk of the specified logical volume.
	DiskDelete(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, diskName string, opts ...model.VolumeDiskDeleteOption) error

	// DiskAssign assigns a disk to the specified logical volume.
	DiskAssign(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, diskName string, opts ...model.VolumeDiskAssignOption) error

	// SnapshotList returns a list of snapshots of the specified volume.
	SnapshotList(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, opts ...model.VolumeSnapshotListOption) ([]*model.VolumeSnapshot, error)

	// SnapshotCreate creates a snapshot of the specified volume.
	SnapshotCreate(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, snapName string, source string, opts ...model.VolumeSnapshotCreateOption) (*model.VolumeSnapshot, error)

	// SnapshotDelete deletes the specified snapshot.
	SnapshotDelete(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, snapName string, opts ...model.VolumeSnapshotDeleteOption) error

	// Class returns provider-specific volume provisioning parameters.
	Class(ctx context.Context, cluster *model.Cluster, app *model.App, vol model.AppVolume) (model.VolumeClass, error)
}
