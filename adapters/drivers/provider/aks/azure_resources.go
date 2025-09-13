package aks

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/kompox/kompox/internal/logging"
)

// ensureAzureResourceGroupCreated ensures RG exists for Create path only.
func (d *driver) ensureAzureResourceGroupCreated(ctx context.Context, rg string, principalID string) error {
	log := logging.FromContext(ctx)
	// Ensure RG exists
	groupsClient, err := armresources.NewResourceGroupsClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return err
	}
	log.Info(ctx, "ensuring resource group", "resource_group", rg, "subscription", d.AzureSubscriptionId, "location", d.AzureLocation)
	groupRes, err := groupsClient.CreateOrUpdate(ctx, rg, armresources.ResourceGroup{Location: to.Ptr(d.AzureLocation)}, nil)
	if err != nil {
		return err
	}
	// Ensure AKS principal has Contributor on this RG (idempotent)
	principalID = strings.TrimSpace(principalID)
	if principalID == "" {
		// Unknown principal; skip assignment silently (caller should have provided from deployment outputs).
		return nil
	}
	// Create assignment with deterministic GUID name derived from (principalID, roleDefinitionID)
	scope := *groupRes.ID
	roleDefinitionID := fmt.Sprintf("/subscriptions/%s/providers/Microsoft.Authorization/roleDefinitions/%s", d.AzureSubscriptionId, contributorRoleDefinitionID)
	log.Info(ctx, "ensuring role assignment", "scope", scope, "principal_id", principalID, "role_definition_id", contributorRoleDefinitionID)
	if err := d.ensureAzureRole(ctx, scope, principalID, roleDefinitionID); err != nil {
		return err
	}
	return nil
}

// ensureAzureResourceGroupDeleted deletes the specified Azure Resource Group idempotently.
// - If the RG does not exist, it returns nil.
// - If there are Key Vault resources in the RG, it will purge them after RG deletion.
func (d *driver) ensureAzureResourceGroupDeleted(ctx context.Context, rg string) error {
	log := logging.FromContext(ctx)

	groupsClient, err := armresources.NewResourceGroupsClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return fmt.Errorf("failed to create resource groups client: %w", err)
	}

	// Check if resource group exists first; treat not-found as already deleted.
	if _, err := groupsClient.Get(ctx, rg, nil); err != nil {
		return nil
	}

	// Collect Key Vaults for post-delete purge.
	keyVaultNames, err := d.listAzureKeyVaultsInResourceGroup(ctx, rg)
	if err != nil {
		log.Debug(ctx, "failed to get key vaults in resource group", "error", err, "resource_group", rg)
		keyVaultNames = []string{}
	}

	log.Info(ctx, "deleting resource group", "resource_group", rg, "subscription", d.AzureSubscriptionId, "key_vaults_to_purge", len(keyVaultNames))
	poller, err := groupsClient.BeginDelete(ctx, rg, nil)
	if err != nil {
		return fmt.Errorf("failed to start resource group deletion: %w", err)
	}

	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("failed to delete resource group %s: %w", rg, err)
	}

	log.Info(ctx, "resource group deleted successfully", "resource_group", rg)

	// Purge Key Vaults that were in the deleted RG (best-effort).
	if len(keyVaultNames) > 0 {
		if err := d.purgeAzureKeyVaults(ctx, keyVaultNames); err != nil {
			log.Debug(ctx, "failed to purge some key vaults", "error", err, "key_vaults", keyVaultNames)
		}
	}

	return nil
}
