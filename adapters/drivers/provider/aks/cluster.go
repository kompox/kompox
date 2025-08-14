package aks

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/yaegashi/kompoxops/adapters/kube"
	"github.com/yaegashi/kompoxops/domain/model"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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

	resourceGroupName := cluster.Settings["AZURE_RESOURCE_GROUP_NAME"]
	if resourceGroupName == "" {
		return status, fmt.Errorf("AZURE_RESOURCE_GROUP_NAME is required in cluster settings")
	}

	// Create AKS client
	aksClient, err := armcontainerservice.NewManagedClustersClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return status, fmt.Errorf("failed to create AKS client: %w", err)
	}

	// Check if cluster exists and is provisioned
	aksCluster, err := aksClient.Get(ctx, resourceGroupName, cluster.Name, nil)
	if err != nil {
		// Cluster doesn't exist
		return status, nil
	}

	if aksCluster.Properties == nil || aksCluster.Properties.ProvisioningState == nil {
		return status, nil
	}

	// Check if cluster is provisioned
	if *aksCluster.Properties.ProvisioningState == "Succeeded" {
		status.Provisioned = true

		// Check if cluster is installed by verifying ingress namespace exists
		installed, err := d.checkIngressNamespaceExists(ctx, resourceGroupName, cluster)
		if err != nil {
			// Log error but don't fail the entire status check
			// The cluster is provisioned even if we can't check the namespace
			return status, nil
		}
		status.Installed = installed
	}

	return status, nil
}

// checkIngressNamespaceExists checks if the ingress namespace exists in the K8s cluster
func (d *driver) checkIngressNamespaceExists(ctx context.Context, resourceGroupName string, cluster *model.Cluster) (bool, error) {
	// Acquire kubeconfig via the driver's unified method
	kubeconfig, err := d.ClusterKubeconfig(ctx, cluster)
	if err != nil {
		return false, fmt.Errorf("failed to get cluster kubeconfig: %w", err)
	}

	// Build a shared kube client
	cli, err := kube.NewClientFromKubeconfig(ctx, kubeconfig, &kube.Options{UserAgent: "kompoxops"})
	if err != nil {
		return false, fmt.Errorf("failed to create kube client: %w", err)
	}

	// Determine namespace to check
	ns := "default"
	if cluster.Ingress != nil {
		if namespace, ok := cluster.Ingress["namespace"].(string); ok && namespace != "" {
			ns = namespace
		}
	}

	// Query namespace existence via API
	_, err = cli.Clientset.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		// On other errors (e.g., auth/conn), surface as non-fatal for status
		return false, fmt.Errorf("failed to get namespace %s: %w", ns, err)
	}
	return true, nil
}

// ClusterInstall installs in-cluster resources (Ingress Controller, etc.) for AKS cluster.
func (d *driver) ClusterInstall(ctx context.Context, cluster *model.Cluster) error {
	// Build kube client from provider-managed kubeconfig
	kubeconfig, err := d.ClusterKubeconfig(ctx, cluster)
	if err != nil {
		return fmt.Errorf("get kubeconfig: %w", err)
	}
	kc, err := kube.NewClientFromKubeconfig(ctx, kubeconfig, &kube.Options{UserAgent: "kompoxops"})
	if err != nil {
		return fmt.Errorf("new kube client: %w", err)
	}
	installer := kube.NewInstaller(kc)

	// Step 1: Ensure ingress namespace exists (idempotent)
	if err := installer.EnsureIngressNamespace(ctx, cluster); err != nil {
		return err
	}
	return nil
}

// ClusterUninstall uninstalls in-cluster resources (Ingress Controller, etc.) from AKS cluster.
func (d *driver) ClusterUninstall(ctx context.Context, cluster *model.Cluster) error {
	// Build kube client from provider-managed kubeconfig
	kubeconfig, err := d.ClusterKubeconfig(ctx, cluster)
	if err != nil {
		return fmt.Errorf("get kubeconfig: %w", err)
	}
	kc, err := kube.NewClientFromKubeconfig(ctx, kubeconfig, &kube.Options{UserAgent: "kompoxops"})
	if err != nil {
		return fmt.Errorf("new kube client: %w", err)
	}
	installer := kube.NewInstaller(kc)

	// Step 1: Delete ingress namespace (best-effort, idempotent)
	if err := installer.DeleteIngressNamespace(ctx, cluster); err != nil {
		return err
	}
	return nil
}

// ClusterKubeconfig returns admin kubeconfig bytes for the AKS cluster.
func (d *driver) ClusterKubeconfig(ctx context.Context, cluster *model.Cluster) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	resourceGroupName := cluster.Settings["AZURE_RESOURCE_GROUP_NAME"]
	if resourceGroupName == "" {
		return nil, fmt.Errorf("AZURE_RESOURCE_GROUP_NAME is required in cluster settings")
	}

	aksClient, err := armcontainerservice.NewManagedClustersClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create AKS client: %w", err)
	}
	credResult, err := aksClient.ListClusterAdminCredentials(ctx, resourceGroupName, cluster.Name, &armcontainerservice.ManagedClustersClientListClusterAdminCredentialsOptions{ServerFqdn: nil})
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster credentials: %w", err)
	}
	if len(credResult.Kubeconfigs) == 0 || len(credResult.Kubeconfigs[0].Value) == 0 {
		return nil, fmt.Errorf("no kubeconfig found for cluster")
	}
	return credResult.Kubeconfigs[0].Value, nil
}
