package model

import "context"

// VolumePort is a domain port abstraction for volume instance operations.
// Implemented by provider driver adapter layer so that use cases remain
// agnostic of concrete driver details.
type VolumePort interface {
	VolumeInstanceList(ctx context.Context, cluster *Cluster, app *App, volName string) ([]*AppVolumeInstance, error)
	VolumeInstanceCreate(ctx context.Context, cluster *Cluster, app *App, volName string) (*AppVolumeInstance, error)
	VolumeInstanceAssign(ctx context.Context, cluster *Cluster, app *App, volName string, volInstName string) error
	VolumeInstanceDelete(ctx context.Context, cluster *Cluster, app *App, volName string, volInstName string) error
}
