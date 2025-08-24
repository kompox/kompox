package model

import "context"

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

// VolumePort abstracts volume instance operations provided by drivers.
type VolumePort interface {
	VolumeInstanceList(ctx context.Context, cluster *Cluster, app *App, volName string) ([]*AppVolumeInstance, error)
	VolumeInstanceCreate(ctx context.Context, cluster *Cluster, app *App, volName string) (*AppVolumeInstance, error)
	VolumeInstanceAssign(ctx context.Context, cluster *Cluster, app *App, volName string, volInstName string) error
	VolumeInstanceDelete(ctx context.Context, cluster *Cluster, app *App, volName string, volInstName string) error
}
