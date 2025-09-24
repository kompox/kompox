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

// assignRole assigns the given role definition to the specified principal at the provided scope.
func (d *driver) ensureAzureRole(ctx context.Context, scope, principalID, roleDefinitionID string) error {
	// Create a role assignments client on demand
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
