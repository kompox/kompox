package aks

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/keyvault/armkeyvault"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/kompox/kompox/domain/model"
	"github.com/kompox/kompox/internal/logging"
)

// assignRolesKeyVaultSecrets assigns Key Vault Secrets User role to the User Assigned Managed Identity
// for all Key Vault secrets referenced in cluster.Ingress.Certificates.
func (d *driver) ensureAzureRoleKeyVaultSecret(ctx context.Context, cluster *model.Cluster, principalID string) error {
	if cluster == nil || cluster.Ingress == nil || len(cluster.Ingress.Certificates) == 0 {
		return nil
	}

	log := logging.FromContext(ctx)

	// Extract Key Vault secret information from certificate sources
	type secretInfo struct {
		kvName     string
		objectName string
		certName   string
		sourceURL  string
	}
	var secrets []secretInfo

	for _, cert := range cluster.Ingress.Certificates {
		if cert.Source == "" {
			continue
		}
		kvName, objectName, err := d.parseKeyVaultSecretURL(cert.Source)
		if err != nil {
			log.Warn(ctx, "failed to parse Key Vault URL, skipping role assignment",
				"cert_name", cert.Name,
				"source", cert.Source,
				"error", err)
			continue
		}
		secrets = append(secrets, secretInfo{
			kvName:     kvName,
			objectName: objectName,
			certName:   cert.Name,
			sourceURL:  cert.Source,
		})
	}

	if len(secrets) == 0 {
		return nil
	}

	// Get unique Key Vault names for resource ID lookup
	keyVaultNames := make(map[string]bool)
	for _, secret := range secrets {
		keyVaultNames[secret.kvName] = true
	}

	// Get all accessible Key Vault resources to find resource IDs
	keyVaultResourceIDs, err := d.azureKeyVaultResourceIDs(ctx, keyVaultNames)
	if err != nil {
		return fmt.Errorf("failed to get Key Vault resource IDs: %w", err)
	}

	roleDefinitionID := d.azureRoleDefinitionID(roleDefIDKeyVaultSecretsUser)

	// Assign roles for each secret individually
	successCount := 0
	errorCount := 0

	for _, secret := range secrets {
		logger := logging.FromContext(ctx).With("certName", secret.certName, "kvName", secret.kvName)
		keyVaultResourceID, exists := keyVaultResourceIDs[secret.kvName]
		if !exists {
			errorCount++
			logger.Info(ctx, "AKS:EnsureKV/efail", "err", "key vault resource not found")
			continue
		}
		logger.Info(ctx, "AKS:EnsureKV/eok")

		// Create scope for the specific secret: <key_vault_resource_id>/secrets/<secret_name>
		secretScope := fmt.Sprintf("%s/secrets/%s", keyVaultResourceID, secret.objectName)

		logger = logging.FromContext(ctx).With("principalId", principalID, "scope", secretScope)
		if err := d.ensureAzureRole(ctx, secretScope, principalID, roleDefinitionID); err != nil {
			errorCount++
			logger.Info(ctx, "AKS:RoleKV/efail", "err", err)
		} else {
			successCount++
			logger.Info(ctx, "AKS:RoleKV/eok")
		}
	}

	// Return error if any role assignments failed
	if errorCount > 0 {
		return fmt.Errorf("some key vault role assignments failed")
	}

	return nil
}

// azureKeyVaultResourceIDs retrieves resource IDs for the specified Key Vault names
// using Azure Resource Graph to search across all accessible subscriptions
func (d *driver) azureKeyVaultResourceIDs(ctx context.Context, keyVaultNames map[string]bool) (map[string]string, error) {
	if len(keyVaultNames) == 0 {
		return nil, nil
	}

	// Create Azure Resource Graph client
	resourceGraphClient, err := armresourcegraph.NewClient(d.TokenCredential, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource graph client: %w", err)
	}

	// Build KQL query to find Key Vaults by name across all accessible subscriptions
	kvNamesList := make([]string, 0, len(keyVaultNames))
	for kvName := range keyVaultNames {
		kvNamesList = append(kvNamesList, fmt.Sprintf("'%s'", kvName))
	}

	// KQL query to find Key Vaults with specific names
	query := fmt.Sprintf(`
		Resources
		| where type == "microsoft.keyvault/vaults"
		| where name in (%s)
		| project name, id, subscriptionId, resourceGroup, location
	`, strings.Join(kvNamesList, ", "))

	// Execute the query
	request := armresourcegraph.QueryRequest{
		Query: to.Ptr(query),
	}

	result, err := resourceGraphClient.Resources(ctx, request, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to query resource graph: %w", err)
	}

	// Parse results
	keyVaultResourceIDs := make(map[string]string)
	if result.Data != nil {
		// result.Data is an interface{} containing a map or array of results
		switch data := result.Data.(type) {
		case map[string]interface{}:
			if rows, ok := data["rows"].([]interface{}); ok {
				for _, row := range rows {
					if rowArray, ok := row.([]interface{}); ok && len(rowArray) >= 2 {
						if name, ok := rowArray[0].(string); ok {
							if id, ok := rowArray[1].(string); ok {
								if keyVaultNames[name] {
									keyVaultResourceIDs[name] = id
								}
							}
						}
					}
				}
			}
		case []interface{}:
			// Handle case where data is directly an array
			for _, row := range data {
				if rowMap, ok := row.(map[string]interface{}); ok {
					if name, ok := rowMap["name"].(string); ok {
						if id, ok := rowMap["id"].(string); ok {
							if keyVaultNames[name] {
								keyVaultResourceIDs[name] = id
							}
						}
					}
				}
			}
		}
	}

	return keyVaultResourceIDs, nil
}

// listAzureKeyVaultsInResourceGroup retrieves the names of Key Vaults in the specified resource group.
func (d *driver) listAzureKeyVaultsInResourceGroup(ctx context.Context, resourceGroupName string) ([]string, error) {
	resourcesClient, err := armresources.NewClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create resources client: %w", err)
	}

	// Filter for Key Vault resources
	filter := "resourceType eq 'Microsoft.KeyVault/vaults'"
	pager := resourcesClient.NewListByResourceGroupPager(resourceGroupName, &armresources.ClientListByResourceGroupOptions{
		Filter: &filter,
	})

	var keyVaultNames []string
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list resources: %w", err)
		}

		for _, resource := range page.Value {
			if resource.Name != nil {
				keyVaultNames = append(keyVaultNames, *resource.Name)
			}
		}
	}

	return keyVaultNames, nil
}

// purgeAzureKeyVaults purges the specified Key Vaults to allow immediate recreation.
func (d *driver) purgeAzureKeyVaults(ctx context.Context, keyVaultNames []string) error {
	if len(keyVaultNames) == 0 {
		return nil
	}

	log := logging.FromContext(ctx)

	vaultsClient, err := armkeyvault.NewVaultsClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return fmt.Errorf("failed to create key vault client: %w", err)
	}

	for _, vaultName := range keyVaultNames {
		// Fire-and-forget: initiate purge without waiting for completion
		log.Info(ctx, "initiating key vault purge (async)", "vault_name", vaultName, "location", d.AzureLocation)
		if _, err := vaultsClient.BeginPurgeDeleted(ctx, vaultName, d.AzureLocation, nil); err != nil {
			log.Warn(ctx, "failed to start key vault purge", "error", err, "vault_name", vaultName)
		}
	}

	return nil
}
