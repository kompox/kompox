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
	maxResourceGroupBaseName = 40
	maxResourceGroupName     = 80
	keyResourceGroupBaseName = "AZURE_RESOURCE_GROUP_BASE_NAME"
	keyResourceGroupName     = "AZURE_RESOURCE_GROUP_NAME"
)

// safeTruncate ensures resulting name does not exceed Azure RG length, preserving hash suffix.
func safeTruncate(base, mid, hash string) string {
	name := fmt.Sprintf("%s_%s_%s", base, mid, hash)
	if len(name) <= maxResourceGroupName { // Azure RG allowed chars are ASCII subset, byte length == char length
		return name
	}
	allowMid := max(maxResourceGroupName-(len(base)+2+len(hash)), 1)
	if len(mid) > allowMid {
		mid = mid[:allowMid]
	}
	return fmt.Sprintf("%s_%s_%s", base, mid, hash)
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
	return safeTruncate(d.resourceGroupBaseName, cluster.Name, h.Cluster), nil
}

func (d *driver) volumeResourceGroupName(app *model.App) (string, error) {
	if app == nil {
		return "", fmt.Errorf("app nil")
	}
	if app.Settings != nil {
		if v := app.Settings[keyResourceGroupName]; v != "" {
			return v, nil
		}
	}
	h := naming.NewHashes(d.ServiceName(), d.ProviderName(), "", app.Name)
	return safeTruncate(d.resourceGroupBaseName, app.Name, h.AppID), nil
}
