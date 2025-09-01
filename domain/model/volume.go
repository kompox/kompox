package model

import (
	"context"
	"time"
)

// VolumePort abstracts volume instance operations provided by drivers.
type VolumePort interface {
	VolumeInstanceList(ctx context.Context, cluster *Cluster, app *App, volName string) ([]*VolumeInstance, error)
	VolumeInstanceCreate(ctx context.Context, cluster *Cluster, app *App, volName string) (*VolumeInstance, error)
	VolumeInstanceDelete(ctx context.Context, cluster *Cluster, app *App, volName string, volInstName string) error
	VolumeInstanceAssign(ctx context.Context, cluster *Cluster, app *App, volName string, volInstName string) error
	VolumeSnapshotList(ctx context.Context, cluster *Cluster, app *App, volName string) ([]*VolumeSnapshot, error)
	VolumeSnapshotCreate(ctx context.Context, cluster *Cluster, app *App, volName string, volInstName string) (*VolumeSnapshot, error)
	VolumeSnapshotDelete(ctx context.Context, cluster *Cluster, app *App, volName string, snapName string) error
	VolumeSnapshotRestore(ctx context.Context, cluster *Cluster, app *App, volName string, snapName string) (*VolumeInstance, error)
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
