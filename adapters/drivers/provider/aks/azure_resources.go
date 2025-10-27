package aks

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/kompox/kompox/internal/logging"
)

func azureShorterErrorString(err error) string {
	errstr := err.Error()
	var responseErr *azcore.ResponseError
	if errors.As(err, &responseErr) {
		errstr = fmt.Sprintf("%d %s (%s)", responseErr.StatusCode, http.StatusText(responseErr.StatusCode), responseErr.ErrorCode)
	}
	return errstr
}

// ensureAzureResourceGroupCreated ensures RG exists for Create path only.
func (d *driver) ensureAzureResourceGroupCreated(ctx context.Context, rg string, tags map[string]*string, principalID string) error {
	// Ensure RG exists
	groupsClient, err := armresources.NewResourceGroupsClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return err
	}

	logger := logging.FromContext(ctx).With("subscription", d.AzureSubscriptionId, "location", d.AzureLocation, "name", rg, "tags", tagsForLog(tags))
	groupRes, err := groupsClient.CreateOrUpdate(ctx, rg, armresources.ResourceGroup{Location: to.Ptr(d.AzureLocation), Tags: tags}, nil)
	if err != nil {
		logger.Info(ctx, "AKS:EnsureRG/efail", "err", err)
		return err
	}
	logger.Info(ctx, "AKS:EnsureRG/eok")
	// Ensure AKS principal has Contributor on this RG (idempotent)
	principalID = strings.TrimSpace(principalID)
	if principalID == "" {
		// Unknown principal; skip assignment silently (caller should have provided from deployment outputs).
		return nil
	}
	// Create assignment with deterministic GUID name derived from (principalID, roleDefinitionID)
	scope := *groupRes.ID
	roleDefinitionID := d.azureRoleDefinitionID(roleDefIDContributor)

	logger = logging.FromContext(ctx).With("principalId", principalID, "scope", scope)
	if err := d.ensureAzureRole(ctx, scope, principalID, roleDefinitionID); err != nil {
		logger.Info(ctx, "AKS:RoleRG/efail", "err", err)
		return err
	}
	logger.Info(ctx, "AKS:RoleRG/eok")
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
