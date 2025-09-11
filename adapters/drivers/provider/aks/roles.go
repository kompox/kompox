package aks

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
	"github.com/kompox/kompox/domain/model"
	"github.com/kompox/kompox/internal/logging"
)

// Key Vault Secrets User role definition ID
const keyVaultSecretsUserRoleID = "4633458b-17de-408a-b874-0445c86b69e6"

// assignRolesKeyVaultSecrets assigns Key Vault Secrets User role to the User Assigned Managed Identity
// for all Key Vault secrets referenced in cluster.Ingress.Certificates.
func (d *driver) assignRolesKeyVaultSecrets(ctx context.Context, cluster *model.Cluster, principalID string) error {
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
	keyVaultResourceIDs, err := d.getKeyVaultResourceIDs(ctx, keyVaultNames)
	if err != nil {
		return fmt.Errorf("failed to get Key Vault resource IDs: %w", err)
	}

	// Create role assignments client
	roleAssignmentsClient, err := armauthorization.NewRoleAssignmentsClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return fmt.Errorf("failed to create role assignments client: %w", err)
	}

	// Assign roles for each secret individually
	successCount := 0
	errorCount := 0

	for _, secret := range secrets {
		keyVaultResourceID, exists := keyVaultResourceIDs[secret.kvName]
		if !exists {
			errorCount++
			log.Warn(ctx, "Key Vault resource ID not found, skipping role assignment",
				"key_vault", secret.kvName,
				"cert_name", secret.certName)
			continue
		}

		// Create scope for the specific secret: <key_vault_resource_id>/secrets/<secret_name>
		secretScope := fmt.Sprintf("%s/secrets/%s", keyVaultResourceID, secret.objectName)

		if err := d.assignKeyVaultRole(ctx, roleAssignmentsClient, secretScope, principalID, secret.kvName, secret.objectName); err != nil {
			errorCount++
			log.Warn(ctx, "failed to assign Key Vault Secrets User role",
				"key_vault", secret.kvName,
				"secret_name", secret.objectName,
				"cert_name", secret.certName,
				"scope", secretScope,
				"principal_id", principalID,
				"error", err)
			// Continue with other secrets
		} else {
			successCount++
			log.Info(ctx, "successfully assigned Key Vault Secrets User role",
				"key_vault", secret.kvName,
				"secret_name", secret.objectName,
				"cert_name", secret.certName,
				"principal_id", principalID)
		}
	}

	// Log summary
	log.Info(ctx, "Key Vault role assignment summary",
		"success_count", successCount,
		"error_count", errorCount,
		"total_count", len(secrets))

	// Return error if any role assignments failed
	if errorCount > 0 {
		return fmt.Errorf("some key vault role assignments failed")
	}

	return nil
}

// getKeyVaultResourceIDs retrieves resource IDs for the specified Key Vault names
// using Azure Resource Graph to search across all accessible subscriptions
func (d *driver) getKeyVaultResourceIDs(ctx context.Context, keyVaultNames map[string]bool) (map[string]string, error) {
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

// assignKeyVaultRole assigns the Key Vault Secrets User role to the specified principal for a specific secret
func (d *driver) assignKeyVaultRole(ctx context.Context, client *armauthorization.RoleAssignmentsClient, scope, principalID, kvName, secretName string) error {
	// Create role definition ID (subscription scoped)
	roleDefinitionID := fmt.Sprintf("/subscriptions/%s/providers/Microsoft.Authorization/roleDefinitions/%s",
		d.AzureSubscriptionId, keyVaultSecretsUserRoleID)

	// Generate a deterministic role assignment name to ensure idempotency
	// Include secret name in the role assignment name for uniqueness
	roleAssignmentName := d.generateRoleAssignmentName(principalID, fmt.Sprintf("%s-%s", keyVaultSecretsUserRoleID, secretName))

	roleAssignment := armauthorization.RoleAssignmentCreateParameters{
		Properties: &armauthorization.RoleAssignmentProperties{
			RoleDefinitionID: to.Ptr(roleDefinitionID),
			PrincipalID:      to.Ptr(principalID),
			PrincipalType:    to.Ptr(armauthorization.PrincipalTypeServicePrincipal),
		},
	}

	// Try to create the role assignment (idempotent - will succeed if already exists)
	_, err := client.Create(ctx, scope, roleAssignmentName, roleAssignment, nil)
	if err != nil {
		// Check if the error is because the role assignment already exists
		if strings.Contains(strings.ToLower(err.Error()), "already exists") ||
			strings.Contains(strings.ToLower(err.Error()), "conflict") {
			// Role assignment already exists, which is fine
			return nil
		}
		return fmt.Errorf("failed to create role assignment: %w", err)
	}

	return nil
}

// generateRoleAssignmentName generates a deterministic GUID for role assignment
func (d *driver) generateRoleAssignmentName(principalID, roleID string) string {
	// Generate a deterministic UUID v5-like identifier
	// Combine principal ID, role ID to create a unique hash
	hasher := sha256.New()
	hasher.Write([]byte(principalID))
	hasher.Write([]byte(roleID))
	hash := hasher.Sum(nil)

	// Format as UUID v4 format: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
	// Take first 16 bytes of the hash and format as UUID
	return fmt.Sprintf("%08x-%04x-4%03x-%04x-%012x",
		hash[0:4],
		hash[4:6],
		(uint32(hash[6])<<8|uint32(hash[7]))&0x0fff,
		(uint32(hash[8])<<8|uint32(hash[9])&0x3fff)|0x8000,
		hash[10:16])
}
