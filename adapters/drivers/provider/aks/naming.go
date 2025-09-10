package aks

// Resource group naming helpers for AKS driver.
//
// Rules:
// Provider base RG name:
//   base = provider.settings.AZURE_RESOURCE_GROUP_BASE_NAME || "kompox_{provider.name}"
// Cluster provisioning RG name:
//   cluster.settings.AZURE_RESOURCE_GROUP_NAME || "{base}_{cluster.name}_{hash.Cluster}"
// Volume (App) RG name:
//   app.settings.AZURE_RESOURCE_GROUP_NAME || "{base}_{app.name}_{hash.AppID}"
// Hashes are generated via internal/naming.NewHashes.
// Length is truncated to Azure RG max (90 chars) preserving hash suffix.

import (
	"fmt"

	"github.com/kompox/kompox/domain/model"
	"github.com/kompox/kompox/internal/naming"
)

// Resource group related limits and setting keys.
const (
	maxResourcePrefix    = 32
	maxResourceName      = 72
	keyResourcePrefix    = "AZURE_RESOURCE_PREFIX"
	keyResourceGroupName = "AZURE_RESOURCE_GROUP_NAME"
)

// safeTruncate ensures resulting name does not exceed Azure RG length, preserving hash suffix.
// Returns an error if the hash is too long to accommodate any base characters.
func safeTruncate(base, hash string) (string, error) {
	maxBaseLen := maxResourceName - (len(hash) + 1)
	if maxBaseLen < 1 {
		return "", fmt.Errorf("hash too long: %d chars exceeds limit", len(hash))
	}
	if len(base) > maxBaseLen {
		base = base[:maxBaseLen]
	}
	return fmt.Sprintf("%s_%s", base, hash), nil
}

func (d *driver) clusterResourceGroupName(cluster *model.Cluster) (string, error) {
	if cluster == nil {
		return "", fmt.Errorf("cluster nil")
	}
	if cluster.Settings != nil {
		if v := cluster.Settings[keyResourceGroupName]; v != "" {
			return v, nil
		}
	}
	h := naming.NewHashes(d.ServiceName(), d.ProviderName(), cluster.Name, "")
	base := fmt.Sprintf("%s_cls_%s", d.resourcePrefix, cluster.Name)
	result, err := safeTruncate(base, h.Cluster)
	if err != nil {
		return "", fmt.Errorf("cluster resource group name: %w", err)
	}
	return result, nil
}

func (d *driver) appResourceGroupName(app *model.App) (string, error) {
	if app == nil {
		return "", fmt.Errorf("app nil")
	}
	if app.Settings != nil {
		if v := app.Settings[keyResourceGroupName]; v != "" {
			return v, nil
		}
	}
	h := naming.NewHashes(d.ServiceName(), d.ProviderName(), "", app.Name)
	base := fmt.Sprintf("%s_app_%s", d.resourcePrefix, app.Name)
	result, err := safeTruncate(base, h.AppID)
	if err != nil {
		return "", fmt.Errorf("app resource group name: %w", err)
	}
	return result, nil
}

func (d *driver) appDiskName(app *model.App, volName string, diskName string) (string, error) {
	h := naming.NewHashes(d.ServiceName(), d.ProviderName(), "", app.Name)
	base := fmt.Sprintf("%s_disk_%s_%s", d.resourcePrefix, volName, diskName)
	result, err := safeTruncate(base, h.AppID)
	if err != nil {
		return "", fmt.Errorf("disk name: %w", err)
	}
	return result, nil
}

func (d *driver) appSnapshotName(app *model.App, volName string, snapshotName string) (string, error) {
	h := naming.NewHashes(d.ServiceName(), d.ProviderName(), "", app.Name)
	base := fmt.Sprintf("%s_snap_%s_%s", d.resourcePrefix, volName, snapshotName)
	result, err := safeTruncate(base, h.AppID)
	if err != nil {
		return "", fmt.Errorf("snapshot name: %w", err)
	}
	return result, nil
}
