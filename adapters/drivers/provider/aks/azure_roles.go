package aks

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v2"
	"github.com/google/uuid"
)

// UUIDv5 namespace used to generate role assignment names.
// Chosen arbitrarily but kept constant to ensure stable name generation.
var roleAssignmentNamespace = uuid.MustParse("b7b0e0a0-9c4e-4c09-bc61-0f6c2f2f6d6a")

// assignRole assigns the given role definition to the specified principal at the provided scope.
func (d *driver) ensureAzureRole(ctx context.Context, scope, principalID, roleDefinitionID string) error {
	// Create a role assignments client on demand
	client, err := armauthorization.NewRoleAssignmentsClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return fmt.Errorf("failed to create role assignments client: %w", err)
	}

	// Generate a deterministic role assignment name to ensure idempotency per (principal, role)
	nameInput := principalID + "|" + roleDefinitionID
	roleAssignmentName := uuid.NewSHA1(roleAssignmentNamespace, []byte(nameInput)).String()

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
