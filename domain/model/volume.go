package model

import (
	"context"
	"time"
)

// Operation-scoped options and functional option types for volume operations.
// These are placeholders for future extensions (e.g., Force, DryRun, Timeout).
// Drivers and adapters should accept and may ignore them until used.

type VolumeInstanceListOptions struct{ Force bool }
type VolumeInstanceCreateOptions struct{ Force bool }
type VolumeInstanceDeleteOptions struct{ Force bool }
type VolumeInstanceAssignOptions struct{ Force bool }

type VolumeSnapshotListOptions struct{ Force bool }
type VolumeSnapshotCreateOptions struct{ Force bool }
type VolumeSnapshotDeleteOptions struct{ Force bool }
type VolumeSnapshotRestoreOptions struct{ Force bool }

type VolumeInstanceListOption func(*VolumeInstanceListOptions)
type VolumeInstanceCreateOption func(*VolumeInstanceCreateOptions)
type VolumeInstanceDeleteOption func(*VolumeInstanceDeleteOptions)
type VolumeInstanceAssignOption func(*VolumeInstanceAssignOptions)

type VolumeSnapshotListOption func(*VolumeSnapshotListOptions)
type VolumeSnapshotCreateOption func(*VolumeSnapshotCreateOptions)
type VolumeSnapshotDeleteOption func(*VolumeSnapshotDeleteOptions)
type VolumeSnapshotRestoreOption func(*VolumeSnapshotRestoreOptions)

// Option helpers mirroring cluster options style.
func WithVolumeInstanceListForce() VolumeInstanceListOption {
	return func(o *VolumeInstanceListOptions) { o.Force = true }
}
func WithVolumeInstanceCreateForce() VolumeInstanceCreateOption {
	return func(o *VolumeInstanceCreateOptions) { o.Force = true }
}
func WithVolumeInstanceDeleteForce() VolumeInstanceDeleteOption {
	return func(o *VolumeInstanceDeleteOptions) { o.Force = true }
}
func WithVolumeInstanceAssignForce() VolumeInstanceAssignOption {
	return func(o *VolumeInstanceAssignOptions) { o.Force = true }
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

// VolumePort abstracts volume instance operations provided by drivers.
type VolumePort interface {
	VolumeInstanceList(ctx context.Context, cluster *Cluster, app *App, volName string, opts ...VolumeInstanceListOption) ([]*VolumeInstance, error)
	VolumeInstanceCreate(ctx context.Context, cluster *Cluster, app *App, volName string, opts ...VolumeInstanceCreateOption) (*VolumeInstance, error)
	VolumeInstanceDelete(ctx context.Context, cluster *Cluster, app *App, volName string, volInstName string, opts ...VolumeInstanceDeleteOption) error
	VolumeInstanceAssign(ctx context.Context, cluster *Cluster, app *App, volName string, volInstName string, opts ...VolumeInstanceAssignOption) error
	VolumeSnapshotList(ctx context.Context, cluster *Cluster, app *App, volName string, opts ...VolumeSnapshotListOption) ([]*VolumeSnapshot, error)
	VolumeSnapshotCreate(ctx context.Context, cluster *Cluster, app *App, volName string, volInstName string, opts ...VolumeSnapshotCreateOption) (*VolumeSnapshot, error)
	VolumeSnapshotDelete(ctx context.Context, cluster *Cluster, app *App, volName string, snapName string, opts ...VolumeSnapshotDeleteOption) error
	VolumeSnapshotRestore(ctx context.Context, cluster *Cluster, app *App, volName string, snapName string, opts ...VolumeSnapshotRestoreOption) (*VolumeInstance, error)
}

// VolumeInstance represents a specific instance of a logical volume.
type VolumeInstance struct {
	Name       string    `json:"name"`       // name of the volume instance
	VolumeName string    `json:"volumeName"` // name of the volume this instance is based on
	Assigned   bool      `json:"assigned"`   // whether this instance is assigned to the volume
	Size       int64     `json:"size"`       // volume instance size in bytes
	Handle     string    `json:"handle"`     // provider driver specific handle
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
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
