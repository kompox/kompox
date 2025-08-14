package aks

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	providerdrv "github.com/yaegashi/kompoxops/adapters/drivers/provider"
	"github.com/yaegashi/kompoxops/domain/model"
)

// driver implements the AKS provider driver.
type driver struct {
	TokenCredential     azcore.TokenCredential
	AzureSubscriptionId string
	AzureLocation       string
}

// ID returns the provider identifier.
func (d *driver) ID() string { return "aks" }

// init registers the AKS driver.
func init() {
	providerdrv.Register("aks", func(settings map[string]string) (providerdrv.Driver, error) {
		get := func(k string) string {
			if settings == nil {
				return ""
			}
			return strings.TrimSpace(settings[k])
		}

		subscriptionID := get("AZURE_SUBSCRIPTION_ID")
		location := get("AZURE_LOCATION")
		missing := make([]string, 0, 2)
		if subscriptionID == "" {
			missing = append(missing, "AZURE_SUBSCRIPTION_ID")
		}
		if location == "" {
			missing = append(missing, "AZURE_LOCATION")
		}
		if len(missing) > 0 {
			return nil, fmt.Errorf("missing required AKS settings: %s", strings.Join(missing, ", "))
		}

		authMethod := get("AZURE_AUTH_METHOD")
		if authMethod == "" {
			return nil, fmt.Errorf("AZURE_AUTH_METHOD must be specified")
		}

		var cred azcore.TokenCredential
		var err error
		switch authMethod {
		case "client_secret":
			tenantID := get("AZURE_TENANT_ID")
			clientID := get("AZURE_CLIENT_ID")
			clientSecret := get("AZURE_CLIENT_SECRET")
			if tenantID == "" || clientID == "" || clientSecret == "" {
				return nil, fmt.Errorf("client_secret auth requires AZURE_TENANT_ID, AZURE_CLIENT_ID, AZURE_CLIENT_SECRET")
			}
			cred, err = azidentity.NewClientSecretCredential(tenantID, clientID, clientSecret, nil)
		case "client_certificate":
			return nil, fmt.Errorf("client_certificate auth is not supported in this implementation (Go SDK requires x509.Certificate and crypto.PrivateKey parsing)")
		case "managed_identity":
			clientID := get("AZURE_CLIENT_ID")
			opts := &azidentity.ManagedIdentityCredentialOptions{}
			if clientID != "" {
				opts.ID = azidentity.ClientID(clientID)
			}
			cred, err = azidentity.NewManagedIdentityCredential(opts)
		case "workload_identity":
			tenantID := get("AZURE_TENANT_ID")
			clientID := get("AZURE_CLIENT_ID")
			tokenFile := get("AZURE_FEDERATED_TOKEN_FILE")
			if tenantID == "" || clientID == "" || tokenFile == "" {
				return nil, fmt.Errorf("workload_identity auth requires AZURE_TENANT_ID, AZURE_CLIENT_ID, AZURE_FEDERATED_TOKEN_FILE")
			}
			cred, err = azidentity.NewWorkloadIdentityCredential(&azidentity.WorkloadIdentityCredentialOptions{
				TenantID:      tenantID,
				ClientID:      clientID,
				TokenFilePath: tokenFile,
			})
		case "azure_cli":
			cred, err = azidentity.NewAzureCLICredential(nil)
		case "azure_developer_cli":
			cred, err = azidentity.NewAzureDeveloperCLICredential(nil)
		default:
			return nil, fmt.Errorf("unsupported AZURE_AUTH_METHOD: %s", authMethod)
		}
		if err != nil {
			return nil, fmt.Errorf("create Azure credential: %w", err)
		}

		return &driver{
			TokenCredential:     cred,
			AzureSubscriptionId: subscriptionID,
			AzureLocation:       location,
		}, nil
	})
}

// ClusterProvision provisions an AKS cluster according to the cluster specification.
func (d *driver) ClusterProvision(ctx context.Context, cluster *model.Cluster) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	// Get resource group name from cluster settings
	resourceGroupName := cluster.Settings["AZURE_RESOURCE_GROUP_NAME"]
	if resourceGroupName == "" {
		return fmt.Errorf("AZURE_RESOURCE_GROUP_NAME is required in cluster settings")
	}

	// Create resource group client
	rgClient, err := armresources.NewResourceGroupsClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return fmt.Errorf("failed to create resource group client: %w", err)
	}

	// Create or update resource group
	rgParams := armresources.ResourceGroup{
		Location: to.Ptr(d.AzureLocation),
		Tags: map[string]*string{
			"managed-by": to.Ptr("kompoxops"),
		},
	}
	_, err = rgClient.CreateOrUpdate(ctx, resourceGroupName, rgParams, nil)
	if err != nil {
		return fmt.Errorf("failed to create resource group %s: %w", resourceGroupName, err)
	}

	// Create AKS client
	aksClient, err := armcontainerservice.NewManagedClustersClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return fmt.Errorf("failed to create AKS client: %w", err)
	}

	// Check if cluster already exists
	_, err = aksClient.Get(ctx, resourceGroupName, cluster.Name, nil)
	if err == nil {
		return fmt.Errorf("AKS cluster %s already exists in resource group %s", cluster.Name, resourceGroupName)
	}

	// Define AKS cluster parameters
	aksParams := armcontainerservice.ManagedCluster{
		Location: to.Ptr(d.AzureLocation),
		Tags: map[string]*string{
			"managed-by": to.Ptr("kompoxops"),
		},
		Identity: &armcontainerservice.ManagedClusterIdentity{
			Type: to.Ptr(armcontainerservice.ResourceIdentityTypeSystemAssigned),
		},
		Properties: &armcontainerservice.ManagedClusterProperties{
			DNSPrefix: to.Ptr(cluster.Name),
			AgentPoolProfiles: []*armcontainerservice.ManagedClusterAgentPoolProfile{
				{
					Name:    to.Ptr("nodepool1"),
					Count:   to.Ptr[int32](1),
					VMSize:  to.Ptr("Standard_DS2_v2"),
					OSType:  to.Ptr(armcontainerservice.OSTypeLinux),
					Type:    to.Ptr(armcontainerservice.AgentPoolTypeVirtualMachineScaleSets),
					Mode:    to.Ptr(armcontainerservice.AgentPoolModeSystem),
					MaxPods: to.Ptr[int32](30),
				},
			},
			ServicePrincipalProfile: &armcontainerservice.ManagedClusterServicePrincipalProfile{
				ClientID: to.Ptr("msi"),
			},
		},
	}

	// Start cluster creation
	poller, err := aksClient.BeginCreateOrUpdate(ctx, resourceGroupName, cluster.Name, aksParams, nil)
	if err != nil {
		return fmt.Errorf("failed to start AKS cluster creation: %w", err)
	}

	// Wait for completion
	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to create AKS cluster %s: %w", cluster.Name, err)
	}

	return nil
}

// ClusterDeprovision deprovisions an AKS cluster according to the cluster specification.
func (d *driver) ClusterDeprovision(ctx context.Context, cluster *model.Cluster) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	// Get resource group name from cluster settings
	resourceGroupName := cluster.Settings["AZURE_RESOURCE_GROUP_NAME"]
	if resourceGroupName == "" {
		return fmt.Errorf("AZURE_RESOURCE_GROUP_NAME is required in cluster settings")
	}

	// Create AKS client
	aksClient, err := armcontainerservice.NewManagedClustersClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return fmt.Errorf("failed to create AKS client: %w", err)
	}

	// Check if cluster exists
	_, err = aksClient.Get(ctx, resourceGroupName, cluster.Name, nil)
	if err != nil {
		// If cluster doesn't exist, consider it already deprovisioned
		return nil
	}

	// Start cluster deletion
	poller, err := aksClient.BeginDelete(ctx, resourceGroupName, cluster.Name, nil)
	if err != nil {
		return fmt.Errorf("failed to start AKS cluster deletion: %w", err)
	}

	// Wait for completion
	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to delete AKS cluster %s: %w", cluster.Name, err)
	}

	return nil
}

// ClusterStatus returns the status of an AKS cluster.
func (d *driver) ClusterStatus(ctx context.Context, cluster *model.Cluster) (*model.ClusterStatus, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	status := &model.ClusterStatus{
		Existing:    cluster.Existing,
		Provisioned: false,
		Installed:   false,
	}

	// If cluster is marked as existing, check if it actually exists
	if cluster.Existing {
		// For existing clusters, we need to verify they actually exist
		resourceGroupName := cluster.Settings["AZURE_RESOURCE_GROUP_NAME"]
		if resourceGroupName == "" {
			return status, fmt.Errorf("AZURE_RESOURCE_GROUP_NAME is required in cluster settings")
		}

		// Create AKS client
		aksClient, err := armcontainerservice.NewManagedClustersClient(d.AzureSubscriptionId, d.TokenCredential, nil)
		if err != nil {
			return status, fmt.Errorf("failed to create AKS client: %w", err)
		}

		// Check if cluster exists
		aksCluster, err := aksClient.Get(ctx, resourceGroupName, cluster.Name, nil)
		if err == nil && aksCluster.Properties != nil && aksCluster.Properties.ProvisioningState != nil {
			// Cluster exists and is provisioned if state is Succeeded
			if *aksCluster.Properties.ProvisioningState == "Succeeded" {
				status.Provisioned = true
				// TODO: Check if ingress controller and other components are installed
				// For now, we'll assume installed = provisioned for AKS
				status.Installed = true
			}
		}
	} else {
		// For non-existing clusters, check if they were provisioned by kompoxops
		resourceGroupName := cluster.Settings["AZURE_RESOURCE_GROUP_NAME"]
		if resourceGroupName == "" {
			return status, fmt.Errorf("AZURE_RESOURCE_GROUP_NAME is required in cluster settings")
		}

		// Create AKS client
		aksClient, err := armcontainerservice.NewManagedClustersClient(d.AzureSubscriptionId, d.TokenCredential, nil)
		if err != nil {
			return status, fmt.Errorf("failed to create AKS client: %w", err)
		}

		// Check if cluster exists
		aksCluster, err := aksClient.Get(ctx, resourceGroupName, cluster.Name, nil)
		if err == nil && aksCluster.Properties != nil && aksCluster.Properties.ProvisioningState != nil {
			// Cluster exists and is provisioned if state is Succeeded
			if *aksCluster.Properties.ProvisioningState == "Succeeded" {
				status.Provisioned = true
				// TODO: Check if ingress controller and other components are installed
				// For now, we'll assume installed = provisioned for AKS
				status.Installed = true
			}
		}
	}

	return status, nil
}

// ClusterInstall installs in-cluster resources (Ingress Controller, etc.) for AKS cluster.
func (d *driver) ClusterInstall(ctx context.Context, cluster *model.Cluster) error {
	// For AKS, we can install ingress controllers and other Kubernetes resources
	// This is a placeholder implementation - in practice this would:
	// 1. Connect to the AKS cluster using kubectl or Kubernetes Go client
	// 2. Install Traefik Proxy or other ingress controller via Helm or kubectl
	// 3. Set up any required namespaces, RBAC, etc.

	// TODO: Implement actual cluster resource installation
	// For now, return success to indicate the interface is satisfied
	return nil
}

// ClusterUninstall uninstalls in-cluster resources (Ingress Controller, etc.) from AKS cluster.
func (d *driver) ClusterUninstall(ctx context.Context, cluster *model.Cluster) error {
	// For AKS, we can uninstall ingress controllers and other Kubernetes resources
	// This is a placeholder implementation - in practice this would:
	// 1. Connect to the AKS cluster using kubectl or Kubernetes Go client
	// 2. Uninstall Traefik Proxy or other ingress controller
	// 3. Clean up namespaces, RBAC, etc.

	// TODO: Implement actual cluster resource uninstallation
	// For now, return success to indicate the interface is satisfied
	return nil
}
