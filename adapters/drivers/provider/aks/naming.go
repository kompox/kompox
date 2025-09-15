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

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/kompox/kompox/domain/model"
	"github.com/kompox/kompox/internal/naming"
)

// Resource tag names
const (
	tagServiceName  = "kompox-service-name"
	tagProviderName = "kompox-provider-name"
	tagClusterName  = "kompox-cluster-name"
	tagClusterHash  = "kompox-cluster-hash"
	tagAppName      = "kompox-app-name"
	tagAppIDHash    = "kompox-app-id-hash"
	tagVolumeName   = "kompox-volume"        // volume name
	tagDiskName     = "kompox-disk-name"     // disk name (CompactID)
	tagDiskAssigned = "kompox-disk-assigned" // true/false
	tagSnapshotName = "kompox-snapshot-name" // snapshot name (CompactID)
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

// tagsForLog converts map[string]*string tags into map[string]string for logging.
// Nil values are skipped.
func tagsForLog(tags map[string]*string) map[string]string {
	out := make(map[string]string, len(tags))
	for k, v := range tags {
		if v != nil {
			out[k] = *v
		}
	}
	return out
}

// clusterResourceTags generates key-values for tagging cluster-scoped Azure resources.
func (d *driver) clusterResourceTags(clusterName string) map[string]*string {
	h := naming.NewHashes(d.ServiceName(), d.ProviderName(), clusterName, "")
	return map[string]*string{
		tagServiceName:  to.Ptr(d.ServiceName()),
		tagProviderName: to.Ptr(d.ProviderName()),
		tagClusterName:  to.Ptr(clusterName),
		tagClusterHash:  to.Ptr(h.Cluster),
		"managed-by":    to.Ptr("kompox"),
	}
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

// appResourceTags generates key-values for tagging app-scoped Azure resources.
func (d *driver) appResourceTags(appName string) map[string]*string {
	h := naming.NewHashes(d.ServiceName(), d.ProviderName(), "", appName)
	return map[string]*string{
		tagServiceName:  to.Ptr(d.ServiceName()),
		tagProviderName: to.Ptr(d.ProviderName()),
		tagAppName:      to.Ptr(appName),
		tagAppIDHash:    to.Ptr(h.AppID),
		"managed-by":    to.Ptr("kompox"),
	}
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
