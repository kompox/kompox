package aks

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v2"
	"github.com/google/uuid"
)

// Built-in Azure role definition IDs (UUIDs)
// These are tenant-agnostic and consistent across all Azure subscriptions.
const (
	// Contributor - Full management access to all resources
	roleDefIDContributor = "b24988ac-6180-42a0-ab88-20f7382dd24c"

	// DNS Zone Contributor - Manage DNS zones and record sets
	roleDefIDDNSZoneContributor = "befefa01-2a29-4197-83a8-272ff33ce314"

	// Key Vault Secrets User - Read secret contents
	roleDefIDKeyVaultSecretsUser = "4633458b-17de-408a-b874-0445c86b69e6"

	// AcrPull - Pull images from Azure Container Registry
	roleDefIDAcrPull = "7f951dda-4ed3-4680-a7ca-43fe172d538d"
)

// azureRoleDefinitionID builds the full role definition ID for the subscription scope.
// roleID is the UUID of the built-in or custom role.
func (d *driver) azureRoleDefinitionID(roleDefID string) string {
	return fmt.Sprintf("/subscriptions/%s/providers/Microsoft.Authorization/roleDefinitions/%s",
		d.AzureSubscriptionId, roleDefID)
}

// ensureAzureRole assigns the given role definition to the specified principal at the provided scope.
func (d *driver) ensureAzureRole(ctx context.Context, scope, principalID, roleDefinitionID string) error {
	client, err := armauthorization.NewRoleAssignmentsClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return fmt.Errorf("failed to create role assignments client: %w", err)
	}

	// Generate a random UUIDv4 role assignment name, matching Azure CLI behavior
	roleAssignmentName := uuid.New().String()

	roleAssignment := armauthorization.RoleAssignmentCreateParameters{
		Properties: &armauthorization.RoleAssignmentProperties{
			RoleDefinitionID: to.Ptr(roleDefinitionID),
			PrincipalID:      to.Ptr(principalID),
			PrincipalType:    to.Ptr(armauthorization.PrincipalTypeServicePrincipal),
		},
	}

	// Try to create the role assignment (idempotent - will succeed if already exists)
	_, err = client.Create(ctx, scope, roleAssignmentName, roleAssignment, nil)
	if err != nil {
		var responseErr *azcore.ResponseError
		if errors.As(err, &responseErr) {
			if responseErr.ErrorCode == "RoleAssignmentExists" || responseErr.StatusCode == http.StatusConflict {
				return nil
			}
		}
		return fmt.Errorf("failed to create role assignment: %w", err)
	}

	return nil
}
