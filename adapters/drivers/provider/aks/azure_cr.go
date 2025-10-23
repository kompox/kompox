package aks

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/kompox/kompox/domain/model"
	"github.com/kompox/kompox/internal/logging"
)

const (
	// Cluster settings key for ACR Resource IDs
	settingAzureAKSContainerRegistryResourceIDs = "AZURE_AKS_CONTAINER_REGISTRY_RESOURCE_IDS"
)

// azureContainerRegistryInfo represents parsed Azure Container Registry resource information.
type azureContainerRegistryInfo struct {
	ResourceID string // Full resource ID
	Name       string // Registry name
}

// parseAzureContainerRegistryID parses an Azure Container Registry resource ID using Azure SDK's parser.
// Expected format: /subscriptions/{sub}/resourceGroups/{rg}/providers/Microsoft.ContainerRegistry/registries/{name}
func parseAzureContainerRegistryID(resourceID string) (*azureContainerRegistryInfo, error) {
	rid, err := arm.ParseResourceID(resourceID)
	if err != nil {
		return nil, fmt.Errorf("parse Azure Container Registry resource ID: %w", err)
	}

	// Validate resource type
	if !strings.EqualFold(rid.ResourceType.Namespace, "Microsoft.ContainerRegistry") ||
		!strings.EqualFold(rid.ResourceType.Type, "registries") {
		return nil, fmt.Errorf("invalid resource type for Container Registry: expected Microsoft.ContainerRegistry/registries, got %s/%s",
			rid.ResourceType.Namespace, rid.ResourceType.Type)
	}

	return &azureContainerRegistryInfo{
		ResourceID: resourceID,
		Name:       rid.Name,
	}, nil
}

// collectAzureContainerRegistryIDs retrieves and parses ACR resource IDs from cluster settings.
func (d *driver) collectAzureContainerRegistryIDs(cluster *model.Cluster) ([]*azureContainerRegistryInfo, error) {
	if cluster == nil || cluster.Settings == nil {
		return nil, nil
	}

	registryIDsRaw := strings.TrimSpace(cluster.Settings[settingAzureAKSContainerRegistryResourceIDs])
	if registryIDsRaw == "" {
		return nil, nil
	}

	// Split by comma or space
	var registryIDStrs []string
	for _, id := range strings.FieldsFunc(registryIDsRaw, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n'
	}) {
		id = strings.TrimSpace(id)
		if id != "" {
			registryIDStrs = append(registryIDStrs, id)
		}
	}

	registries := make([]*azureContainerRegistryInfo, 0, len(registryIDStrs))
	for _, id := range registryIDStrs {
		info, err := parseAzureContainerRegistryID(id)
		if err != nil {
			return nil, fmt.Errorf("parse Container Registry resource ID: %w", err)
		}
		registries = append(registries, info)
	}

	return registries, nil
}

// ensureAzureContainerRegistryRoles assigns AcrPull role to the given principal for all specified ACR resources.
// This is a best-effort operation: failures are logged as warnings and do not block cluster installation.
func (d *driver) ensureAzureContainerRegistryRoles(ctx context.Context, principalID string, registries []*azureContainerRegistryInfo) {
	log := logging.FromContext(ctx)

	if len(registries) == 0 {
		return
	}

	// Build full role definition ID for AcrPull
	roleDefID := d.azureRoleDefinitionID(roleDefIDAcrPull)

	for _, reg := range registries {
		// Assign AcrPull role at registry scope (best-effort)
		if err := d.ensureAzureRole(ctx, reg.ResourceID, principalID, roleDefID); err != nil {
			log.Warn(ctx, "failed to assign AcrPull role to AKS principal (best-effort, continuing)",
				"registry", reg.Name,
				"registry_resource_id", reg.ResourceID,
				"principal_id", principalID,
				"error", err.Error(),
			)
		} else {
			log.Info(ctx, "assigned AcrPull role to AKS principal",
				"registry", reg.Name,
				"registry_resource_id", reg.ResourceID,
				"principal_id", principalID,
			)
		}
	}
}
