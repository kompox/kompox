package aks

import (
	"context"
	"fmt"
	"strings"

	"github.com/kompox/kompox/domain/model"
)

// resolveVolumeDriver returns the appropriate volume driver based on volume type.
// Empty type defaults to disk volume driver.
func (d *driver) resolveVolumeDriver(vol *model.AppVolume) (driverVolume, error) {
	volType := vol.Type
	if volType == "" {
		volType = model.VolumeTypeDisk
	}

	vd, ok := d.volumeDrivers[volType]
	if !ok {
		return nil, fmt.Errorf("unsupported volume type: %s", volType)
	}

	return vd, nil
}

// VolumeDiskList implements spec method.
func (d *driver) VolumeDiskList(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, opts ...model.VolumeDiskListOption) ([]*model.VolumeDisk, error) {
	if cluster == nil || app == nil {
		return nil, fmt.Errorf("cluster/app nil")
	}

	vol, err := app.FindVolume(volName)
	if err != nil {
		return nil, fmt.Errorf("find volume: %w", err)
	}

	vd, err := d.resolveVolumeDriver(vol)
	if err != nil {
		return nil, err
	}

	return vd.DiskList(ctx, cluster, app, volName, opts...)
}

// VolumeDiskCreate implements spec method.
func (d *driver) VolumeDiskCreate(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, diskName string, source string, opts ...model.VolumeDiskCreateOption) (*model.VolumeDisk, error) {
	if cluster == nil || app == nil {
		return nil, fmt.Errorf("cluster/app nil")
	}

	diskName = strings.TrimSpace(diskName)

	vol, err := app.FindVolume(volName)
	if err != nil {
		return nil, fmt.Errorf("find volume: %w", err)
	}

	vd, err := d.resolveVolumeDriver(vol)
	if err != nil {
		return nil, err
	}

	return vd.DiskCreate(ctx, cluster, app, volName, diskName, source, opts...)
}

// VolumeDiskAssign implements spec method.
func (d *driver) VolumeDiskAssign(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, diskName string, opts ...model.VolumeDiskAssignOption) error {
	if cluster == nil || app == nil {
		return fmt.Errorf("cluster/app nil")
	}

	vol, err := app.FindVolume(volName)
	if err != nil {
		return fmt.Errorf("find volume: %w", err)
	}

	vd, err := d.resolveVolumeDriver(vol)
	if err != nil {
		return err
	}

	return vd.DiskAssign(ctx, cluster, app, volName, diskName, opts...)
}

// VolumeDiskDelete implements spec method.
func (d *driver) VolumeDiskDelete(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, diskName string, opts ...model.VolumeDiskDeleteOption) error {
	if cluster == nil || app == nil {
		return fmt.Errorf("cluster/app nil")
	}

	vol, err := app.FindVolume(volName)
	if err != nil {
		return fmt.Errorf("find volume: %w", err)
	}

	vd, err := d.resolveVolumeDriver(vol)
	if err != nil {
		return err
	}

	return vd.DiskDelete(ctx, cluster, app, volName, diskName, opts...)
}

// VolumeSnapshotList implements spec method.
func (d *driver) VolumeSnapshotList(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, opts ...model.VolumeSnapshotListOption) ([]*model.VolumeSnapshot, error) {
	if cluster == nil || app == nil {
		return nil, fmt.Errorf("cluster/app nil")
	}

	vol, err := app.FindVolume(volName)
	if err != nil {
		return nil, fmt.Errorf("find volume: %w", err)
	}

	vd, err := d.resolveVolumeDriver(vol)
	if err != nil {
		return nil, err
	}

	return vd.SnapshotList(ctx, cluster, app, volName, opts...)
}

// VolumeSnapshotCreate implements spec method.
func (d *driver) VolumeSnapshotCreate(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, snapName string, source string, opts ...model.VolumeSnapshotCreateOption) (*model.VolumeSnapshot, error) {
	if cluster == nil || app == nil {
		return nil, fmt.Errorf("cluster/app nil")
	}

	vol, err := app.FindVolume(volName)
	if err != nil {
		return nil, fmt.Errorf("find volume: %w", err)
	}

	vd, err := d.resolveVolumeDriver(vol)
	if err != nil {
		return nil, err
	}

	return vd.SnapshotCreate(ctx, cluster, app, volName, snapName, source, opts...)
}

// VolumeSnapshotDelete implements spec method.
func (d *driver) VolumeSnapshotDelete(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, snapName string, opts ...model.VolumeSnapshotDeleteOption) error {
	if cluster == nil || app == nil {
		return fmt.Errorf("cluster/app nil")
	}

	vol, err := app.FindVolume(volName)
	if err != nil {
		return fmt.Errorf("find volume: %w", err)
	}

	vd, err := d.resolveVolumeDriver(vol)
	if err != nil {
		return err
	}

	return vd.SnapshotDelete(ctx, cluster, app, volName, snapName, opts...)
}

// VolumeClass implements providerdrv.Driver VolumeClass method for AKS.
// Returns opinionated defaults suitable for Azure Disk CSI or Azure Files CSI depending on volume type.
func (d *driver) VolumeClass(ctx context.Context, cluster *model.Cluster, app *model.App, vol model.AppVolume) (model.VolumeClass, error) {
	vd, err := d.resolveVolumeDriver(&vol)
	if err != nil {
		return model.VolumeClass{}, err
	}

	return vd.Class(ctx, cluster, app, vol)
}
