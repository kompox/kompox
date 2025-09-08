package model

import (
	"context"
	"time"
)

// Operation-scoped options and functional option types for volume operations.
// These are placeholders for future extensions (e.g., Force, DryRun, Timeout).
// Drivers and adapters should accept and may ignore them until used.

type VolumeDiskListOptions struct{ Force bool }
type VolumeDiskCreateOptions struct{ Force bool }
type VolumeDiskDeleteOptions struct{ Force bool }
type VolumeDiskAssignOptions struct{ Force bool }

type VolumeSnapshotListOptions struct{ Force bool }
type VolumeSnapshotCreateOptions struct{ Force bool }
type VolumeSnapshotDeleteOptions struct{ Force bool }
type VolumeSnapshotRestoreOptions struct{ Force bool }

type VolumeDiskListOption func(*VolumeDiskListOptions)
type VolumeDiskCreateOption func(*VolumeDiskCreateOptions)
type VolumeDiskDeleteOption func(*VolumeDiskDeleteOptions)
type VolumeDiskAssignOption func(*VolumeDiskAssignOptions)

type VolumeSnapshotListOption func(*VolumeSnapshotListOptions)
type VolumeSnapshotCreateOption func(*VolumeSnapshotCreateOptions)
type VolumeSnapshotDeleteOption func(*VolumeSnapshotDeleteOptions)
type VolumeSnapshotRestoreOption func(*VolumeSnapshotRestoreOptions)

// Option helpers mirroring cluster options style.
func WithVolumeDiskListForce() VolumeDiskListOption {
	return func(o *VolumeDiskListOptions) { o.Force = true }
}
func WithVolumeDiskCreateForce() VolumeDiskCreateOption {
	return func(o *VolumeDiskCreateOptions) { o.Force = true }
}
func WithVolumeDiskDeleteForce() VolumeDiskDeleteOption {
	return func(o *VolumeDiskDeleteOptions) { o.Force = true }
}
func WithVolumeDiskAssignForce() VolumeDiskAssignOption {
	return func(o *VolumeDiskAssignOptions) { o.Force = true }
}

func WithVolumeSnapshotListForce() VolumeSnapshotListOption {
	return func(o *VolumeSnapshotListOptions) { o.Force = true }
}
func WithVolumeSnapshotCreateForce() VolumeSnapshotCreateOption {
	return func(o *VolumeSnapshotCreateOptions) { o.Force = true }
}
func WithVolumeSnapshotDeleteForce() VolumeSnapshotDeleteOption {
	return func(o *VolumeSnapshotDeleteOptions) { o.Force = true }
}
func WithVolumeSnapshotRestoreForce() VolumeSnapshotRestoreOption {
	return func(o *VolumeSnapshotRestoreOptions) { o.Force = true }
}

// VolumePort abstracts volume disk and snapshot operations provided by drivers.
type VolumePort interface {
	DiskList(ctx context.Context, cluster *Cluster, app *App, volName string, opts ...VolumeDiskListOption) ([]*VolumeDisk, error)
	DiskCreate(ctx context.Context, cluster *Cluster, app *App, volName string, opts ...VolumeDiskCreateOption) (*VolumeDisk, error)
	DiskDelete(ctx context.Context, cluster *Cluster, app *App, volName string, diskName string, opts ...VolumeDiskDeleteOption) error
	DiskAssign(ctx context.Context, cluster *Cluster, app *App, volName string, diskName string, opts ...VolumeDiskAssignOption) error
	SnapshotList(ctx context.Context, cluster *Cluster, app *App, volName string, opts ...VolumeSnapshotListOption) ([]*VolumeSnapshot, error)
	SnapshotCreate(ctx context.Context, cluster *Cluster, app *App, volName string, diskName string, opts ...VolumeSnapshotCreateOption) (*VolumeSnapshot, error)
	SnapshotDelete(ctx context.Context, cluster *Cluster, app *App, volName string, snapName string, opts ...VolumeSnapshotDeleteOption) error
	SnapshotRestore(ctx context.Context, cluster *Cluster, app *App, volName string, snapName string, opts ...VolumeSnapshotRestoreOption) (*VolumeDisk, error)
}

// VolumeDisk represents a specific disk of a logical volume.
type VolumeDisk struct {
	Name       string         `json:"name"`       // name of the volume disk
	VolumeName string         `json:"volumeName"` // name of the logical volume this disk belongs to
	Assigned   bool           `json:"assigned"`   // whether this disk is assigned to the logical volume
	Size       int64          `json:"size"`       // volume disk size in bytes
	Zone       string         `json:"zone"`       // availability zone where the disk is located (empty for regional)
	Options    map[string]any `json:"options"`    // provider-specific configuration options
	Handle     string         `json:"handle"`     // provider driver specific handle
	CreatedAt  time.Time      `json:"createdAt"`
	UpdatedAt  time.Time      `json:"updatedAt"`
}

// VolumeSnapshot represents a snapshot artifact belonging to a logical volume.
// Handle should carry cloud-native identifier (e.g., full resource ID/URL).
type VolumeSnapshot struct {
	Name       string    `json:"name"`       // driver-chosen unique name within the logical volume
	VolumeName string    `json:"volumeName"` // logical volume name
	Size       int64     `json:"size"`       // bytes; optional for providers that do not expose
	Handle     string    `json:"handle"`     // provider snapshot handle/ID
	CreatedAt  time.Time `json:"createdAt"`  // snapshot creation time (provider-reported if available)
	UpdatedAt  time.Time `json:"updatedAt"`  // last update time (if applicable)
}

// VolumeClass defines provider-specific parameters for persistent volumes.
// Empty fields mean no opinion; callers should omit them from manifests.
type VolumeClass struct {
	StorageClassName string            // e.g. "managed-csi"
	CSIDriver        string            // CSI driver name
	FSType           string            // Filesystem type, e.g. ext4
	Attributes       map[string]string // CSI volumeAttributes
	AccessModes      []string          // e.g. ["ReadWriteOnce"]
	ReclaimPolicy    string            // "Retain" | "Delete"
	VolumeMode       string            // "Filesystem" | "Block"
}
