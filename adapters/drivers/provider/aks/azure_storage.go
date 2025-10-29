package aks

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"github.com/kompox/kompox/domain/model"
	"github.com/kompox/kompox/internal/logging"
)

// ensureStorageAccountCreated creates the storage account if it doesn't exist.
// This is called during the first disk creation for Type="files".
func (d *driver) ensureStorageAccountCreated(ctx context.Context, cluster *model.Cluster, app *model.App, sku string) error {
	log := logging.FromContext(ctx)

	rg, err := d.appResourceGroupName(app)
	if err != nil {
		return fmt.Errorf("app RG: %w", err)
	}

	accountName, err := d.appStorageAccountName(app)
	if err != nil {
		return fmt.Errorf("storage account name: %w", err)
	}

	// Ensure resource group exists with proper role assignments
	// Get Kubelet Identity principal ID for role assignment (optional for non-AKS scenarios)
	// Kubelet Identity is used by Azure Files CSI driver to retrieve storage account keys
	principalID := ""
	outputs, err := d.azureDeploymentOutputs(ctx, cluster)
	if err == nil {
		if v, ok := outputs[outputAksKubeletPrincipalID].(string); ok {
			principalID = v
		}
	}
	// Ignore errors: principalID remains empty if outputs unavailable (e.g., aks-e2e-volume tests)
	// ensureAzureResourceGroupCreated will skip role assignment when principalID is empty
	err = d.ensureAzureResourceGroupCreated(ctx, rg, d.appResourceTags(app.Name), principalID)
	if err != nil {
		return fmt.Errorf("ensure RG for storage account: %w", err)
	}

	// Create storage accounts client
	accountsClient, err := armstorage.NewAccountsClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return fmt.Errorf("new storage accounts client: %w", err)
	}

	// Check if account exists
	_, err = accountsClient.GetProperties(ctx, rg, accountName, nil)
	if err == nil {
		// Account already exists
		return nil
	}

	// Account doesn't exist, create it
	log.Info(ctx, "Creating storage account", "account", accountName, "resource_group", rg)

	// Parse SKU
	var skuName armstorage.SKUName
	switch sku {
	case "Standard_LRS":
		skuName = armstorage.SKUNameStandardLRS
	case "Standard_GRS":
		skuName = armstorage.SKUNameStandardGRS
	case "Standard_RAGRS":
		skuName = armstorage.SKUNameStandardRAGRS
	case "Standard_ZRS":
		skuName = armstorage.SKUNameStandardZRS
	case "Premium_LRS":
		skuName = armstorage.SKUNamePremiumLRS
	case "Premium_ZRS":
		skuName = armstorage.SKUNamePremiumZRS
	default:
		skuName = armstorage.SKUNameStandardLRS
	}

	// Create storage account
	params := armstorage.AccountCreateParameters{
		SKU: &armstorage.SKU{
			Name: to.Ptr(skuName),
		},
		Kind:     to.Ptr(armstorage.KindStorageV2),
		Location: to.Ptr(d.AzureLocation),
		Tags:     d.appResourceTags(app.Name),
		Properties: &armstorage.AccountPropertiesCreateParameters{
			AllowBlobPublicAccess:        to.Ptr(false),
			AllowSharedKeyAccess:         to.Ptr(true),
			MinimumTLSVersion:            to.Ptr(armstorage.MinimumTLSVersionTLS12),
			PublicNetworkAccess:          to.Ptr(armstorage.PublicNetworkAccessEnabled),
			EnableHTTPSTrafficOnly:       to.Ptr(true),
			DefaultToOAuthAuthentication: to.Ptr(false),
		},
	}

	poller, err := accountsClient.BeginCreate(ctx, rg, accountName, params, nil)
	if err != nil {
		return fmt.Errorf("begin create storage account: %w", err)
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("create storage account: %w", err)
	}

	log.Info(ctx, "Storage account created", "account", accountName)
	return nil
}
