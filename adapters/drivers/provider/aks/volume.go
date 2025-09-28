package aks

import (
	"context"

	"github.com/kompox/kompox/domain/model"
)

// Built-in role definition
// VolumeClass implements providerdrv.Driver VolumeClass method for AKS.
// Returns opinionated defaults suitable for Azure Disk CSI; callers must omit any empty field.
func (d *driver) VolumeClass(ctx context.Context, cluster *model.Cluster, app *model.App, vol model.AppVolume) (model.VolumeClass, error) {
	// For now we always return managed-csi / disk.csi.azure.com with ext4; future: derive from volume, size, perf tier.
	return model.VolumeClass{
		StorageClassName: "managed-csi",
		CSIDriver:        "disk.csi.azure.com",
		FSType:           "ext4",
		Attributes:       map[string]string{"fsType": "ext4"},
		AccessModes:      []string{"ReadWriteOnce"},
		ReclaimPolicy:    "Retain",
		VolumeMode:       "Filesystem",
	}, nil
}
