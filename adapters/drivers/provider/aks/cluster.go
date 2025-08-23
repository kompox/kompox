package aks

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/keyvault/armkeyvault"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/yaegashi/kompoxops/adapters/kube"
	"github.com/yaegashi/kompoxops/domain/model"
	"github.com/yaegashi/kompoxops/internal/logging"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Constants for template output keys
const (
	OutputResourceGroupName              = "AZURE_RESOURCE_GROUP_NAME"
	OutputAksClusterName                 = "AZURE_AKS_CLUSTER_NAME"
	OutputIngressServiceAccountNamespace = "AZURE_INGRESS_SERVICE_ACCOUNT_NAMESPACE"
	OutputIngressServiceAccountName      = "AZURE_INGRESS_SERVICE_ACCOUNT_NAME"
)

// deploymentName generates the deployment name for the subscription-scoped deployment.
// It returns the same name as the resource group name for consistency.
func (d *driver) deploymentName(cluster *model.Cluster) (string, error) {
	return d.clusterResourceGroupName(cluster)
}

// clusterTagValue generates the cluster tag value for resource tagging.
func (d *driver) clusterTagValue(clusterName string) string {
	return fmt.Sprintf("%s/%s/%s", d.ServiceName(), d.ProviderName(), clusterName)
}

// getDeploymentOutputs retrieves the outputs from the subscription-scoped deployment.
func (d *driver) getDeploymentOutputs(ctx context.Context, cluster *model.Cluster) (map[string]any, error) {
	deploymentsClient, err := armresources.NewDeploymentsClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create deployments client: %w", err)
	}

	deploymentName, err := d.deploymentName(cluster)
	if err != nil {
		return nil, fmt.Errorf("derive deployment name: %w", err)
	}
	deployment, err := deploymentsClient.GetAtSubscriptionScope(ctx, deploymentName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscription deployment: %w", err)
	}

	if deployment.Properties == nil || deployment.Properties.Outputs == nil {
		return nil, fmt.Errorf("deployment has no outputs")
	}

	// Type assert the outputs to the correct map type
	outputsMap, ok := deployment.Properties.Outputs.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("deployment outputs has unexpected type")
	}

	outputs := make(map[string]any)
	for key, value := range outputsMap {
		if outputValue, ok := value.(map[string]any); ok {
			if val, exists := outputValue["value"]; exists {
				// Normalize keys to uppercase to avoid issues from accidental casing changes
				outputs[strings.ToUpper(key)] = val
			}
		}
	}

	return outputs, nil
}

// getAKSClient creates a new AKS client and retrieves resource information from deployment outputs.
func (d *driver) getAKSClient(ctx context.Context, cluster *model.Cluster) (*armcontainerservice.ManagedClustersClient, string, string, error) {
	// Get outputs from deployment
	outputs, err := d.getDeploymentOutputs(ctx, cluster)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to get deployment outputs: %w", err)
	}

	// Extract resource group and cluster names from outputs
	aksRGName, ok := outputs[OutputResourceGroupName].(string)
	if !ok {
		return nil, "", "", fmt.Errorf("%s not found in deployment outputs", OutputResourceGroupName)
	}

	aksName, ok := outputs[OutputAksClusterName].(string)
	if !ok {
		return nil, "", "", fmt.Errorf("%s not found in deployment outputs", OutputAksClusterName)
	}

	// Create AKS client
	aksClient, err := armcontainerservice.NewManagedClustersClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to create AKS client: %w", err)
	}

	return aksClient, aksRGName, aksName, nil
}

// getKeyVaultsInResourceGroup retrieves the names of Key Vaults in the specified resource group.
func (d *driver) getKeyVaultsInResourceGroup(ctx context.Context, resourceGroupName string) ([]string, error) {
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

// purgeKeyVaults purges the specified Key Vaults to allow immediate recreation.
func (d *driver) purgeKeyVaults(ctx context.Context, keyVaultNames []string) error {
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

// ClusterProvision provisions an AKS cluster according to the cluster specification.
func (d *driver) ClusterProvision(ctx context.Context, cluster *model.Cluster) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	log := logging.FromContext(ctx)

	// Derive resource group name
	resourceGroupName, err := d.clusterResourceGroupName(cluster)
	if err != nil || resourceGroupName == "" {
		return fmt.Errorf("derive cluster resource group: %w", err)
	}

	log.Info(ctx, "aks cluster provision begin",
		"resource_group", resourceGroupName,
		"cluster", cluster.Name,
		"provider", d.ProviderName(),
		"subscription", d.AzureSubscriptionId,
	)

	// Unmarshal embedded ARM template (subscription scope)
	var template map[string]any
	if err := json.Unmarshal(mainJSON, &template); err != nil {
		return fmt.Errorf("unmarshal embedded template: %w", err)
	}

	// Prepare ARM parameters for subscription-scoped deployment
	parameters := map[string]any{
		"environmentName":   map[string]any{"value": cluster.Name},
		"location":          map[string]any{"value": d.AzureLocation},
		"resourceGroupName": map[string]any{"value": resourceGroupName},
		// Pass ingress ServiceAccount parameters required for workload identity wiring
		"ingressServiceAccountName":      map[string]any{"value": kube.IngressServiceAccountName(cluster)},
		"ingressServiceAccountNamespace": map[string]any{"value": kube.IngressNamespace(cluster)},
	}

	// Create deployments client for subscription-scoped deployment
	deploymentsClient, err := armresources.NewDeploymentsClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return fmt.Errorf("create deployments client: %w", err)
	}

	deploymentName, err := d.deploymentName(cluster)
	if err != nil {
		return fmt.Errorf("derive deployment name: %w", err)
	}

	// Check if deployment already exists and is successful (idempotent)
	if existing, err := deploymentsClient.GetAtSubscriptionScope(ctx, deploymentName, nil); err == nil {
		if existing.Properties != nil && existing.Properties.ProvisioningState != nil &&
			*existing.Properties.ProvisioningState == "Succeeded" {
			log.Info(ctx, "aks cluster already provisioned",
				"resource_group", resourceGroupName,
				"cluster", cluster.Name,
				"provider", d.ProviderName(),
			)
			return nil
		}
		// fallthrough: re-issue deployment to converge
	}

	// Create subscription-scoped deployment
	deployment := armresources.Deployment{
		Location: to.Ptr(d.AzureLocation),
		Properties: &armresources.DeploymentProperties{
			Template:   template,
			Parameters: parameters,
			Mode:       to.Ptr(armresources.DeploymentModeIncremental),
		},
		Tags: map[string]*string{
			"kompox-cluster": to.Ptr(d.clusterTagValue(cluster.Name)),
			"managed-by":     to.Ptr("kompox"),
		},
	}

	poller, err := deploymentsClient.BeginCreateOrUpdateAtSubscriptionScope(ctx, deploymentName, deployment, nil)
	if err != nil {
		return fmt.Errorf("begin subscription deployment creation: %w", err)
	}
	if _, err = poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("subscription deployment creation failed: %w", err)
	}

	log.Info(ctx, "aks cluster provision succeeded",
		"resource_group", resourceGroupName,
		"cluster", cluster.Name,
		"provider", d.ProviderName(),
	)
	return nil
}

// ClusterDeprovision deprovisions an AKS cluster by deleting the entire resource group.
func (d *driver) ClusterDeprovision(ctx context.Context, cluster *model.Cluster) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	log := logging.FromContext(ctx)

	resourceGroupName, err := d.clusterResourceGroupName(cluster)
	if err != nil || resourceGroupName == "" {
		return fmt.Errorf("derive cluster resource group: %w", err)
	}

	// Create resource groups client
	rgClient, err := armresources.NewResourceGroupsClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return fmt.Errorf("failed to create resource groups client: %w", err)
	}

	// Check if resource group exists
	_, err = rgClient.Get(ctx, resourceGroupName, nil)
	if err != nil {
		// If resource group doesn't exist, consider it already deprovisioned
		return nil
	}

	// Get Key Vaults in the resource group before deletion for later purging
	keyVaultNames, err := d.getKeyVaultsInResourceGroup(ctx, resourceGroupName)
	if err != nil {
		log.Debug(ctx, "failed to get key vaults in resource group", "error", err, "resource_group", resourceGroupName)
		// Continue with deletion even if we can't get key vault names
		keyVaultNames = []string{}
	}

	log.Info(ctx, "aks cluster deprovision begin",
		"resource_group", resourceGroupName,
		"cluster", cluster.Name,
		"provider", d.ProviderName(),
		"key_vaults_to_purge", len(keyVaultNames),
	)

	// Delete the entire resource group
	poller, err := rgClient.BeginDelete(ctx, resourceGroupName, nil)
	if err != nil {
		return fmt.Errorf("failed to start resource group deletion: %w", err)
	}

	// Wait for completion
	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to delete resource group %s: %w", resourceGroupName, err)
	}

	log.Info(ctx, "resource group deleted successfully",
		"resource_group", resourceGroupName,
		"cluster", cluster.Name,
		"provider", d.ProviderName(),
	)

	// Purge the Key Vaults that were in the deleted resource group
	if len(keyVaultNames) > 0 {
		if err := d.purgeKeyVaults(ctx, keyVaultNames); err != nil {
			// Log the error but don't fail the entire deprovision operation
			log.Debug(ctx, "failed to purge some key vaults", "error", err, "key_vaults", keyVaultNames)
		}
	}

	log.Info(ctx, "aks cluster deprovision succeeded",
		"resource_group", resourceGroupName,
		"cluster", cluster.Name,
		"provider", d.ProviderName(),
	)

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

	// Create deployments client to check subscription deployment status
	deploymentsClient, err := armresources.NewDeploymentsClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return status, fmt.Errorf("failed to create deployments client: %w", err)
	}

	deploymentName, err := d.deploymentName(cluster)
	if err != nil {
		return status, fmt.Errorf("derive deployment name: %w", err)
	}

	// Check subscription deployment status
	deployment, err := deploymentsClient.GetAtSubscriptionScope(ctx, deploymentName, nil)
	if err != nil {
		// Deployment doesn't exist
		return status, nil
	}

	if deployment.Properties == nil || deployment.Properties.ProvisioningState == nil {
		return status, nil
	}

	// Check if deployment is provisioned successfully
	if *deployment.Properties.ProvisioningState == "Succeeded" {
		status.Provisioned = true

		// Get AKS client and resource information
		aksClient, aksRGName, aksName, err := d.getAKSClient(ctx, cluster)
		if err != nil {
			return status, fmt.Errorf("failed to get AKS client and resource info: %w", err)
		}

		// Check if AKS cluster exists and is provisioned
		aksCluster, err := aksClient.Get(ctx, aksRGName, aksName, nil)
		if err == nil && aksCluster.Properties != nil && aksCluster.Properties.ProvisioningState != nil &&
			*aksCluster.Properties.ProvisioningState == "Succeeded" {

			// Check if cluster is installed by verifying ingress namespace exists
			installed, err := d.checkIngressNamespaceExists(ctx, aksRGName, aksName, cluster)
			if err != nil {
				// Log error but don't fail the entire status check
				// The cluster is provisioned even if we can't check the namespace
				return status, nil
			}
			status.Installed = installed
		}
	}

	return status, nil
}

// checkIngressNamespaceExists checks if the ingress namespace exists in the K8s cluster
func (d *driver) checkIngressNamespaceExists(ctx context.Context, resourceGroupName string, aksClusterName string, cluster *model.Cluster) (bool, error) {
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
	log := logging.FromContext(ctx)

	// Retrieve AKS resource identifiers for logging.
	if aksClient, aksRG, aksName, err := d.getAKSClient(ctx, cluster); err == nil && aksClient != nil {
		log.Info(ctx, "aks cluster install begin",
			"aks_resource_group", aksRG,
			"aks_cluster", aksName,
			"cluster", cluster.Name,
			"provider", d.ProviderName(),
		)
	} else if err != nil { // only debug log on failure to resolve prior to kubeconfig
		log.Debug(ctx, "failed to resolve aks identifiers before install", "error", err, "cluster", cluster.Name, "provider", d.ProviderName())
	}

	// Build kube client from provider-managed kubeconfig
	kubeconfig, err := d.ClusterKubeconfig(ctx, cluster)
	if err != nil {
		return fmt.Errorf("get kubeconfig: %w", err)
	}
	kc, err := kube.NewClientFromKubeconfig(ctx, kubeconfig, &kube.Options{UserAgent: "kompoxops"})
	if err != nil {
		return fmt.Errorf("new kube client: %w", err)
	}
	installer := kube.NewInstallerWithKubeconfig(kc, kubeconfig)

	// Step 1: Ensure ingress namespace exists (idempotent)
	if err := installer.EnsureIngressNamespace(ctx, cluster); err != nil {
		return err
	}

	// Step 1.5: Create ServiceAccount exactly as specified by deployment outputs
	outputs, err := d.getDeploymentOutputs(ctx, cluster)
	if err != nil {
		return fmt.Errorf("read deployment outputs: %w", err)
	}
	saNS, _ := outputs[OutputIngressServiceAccountNamespace].(string)
	saName, _ := outputs[OutputIngressServiceAccountName].(string)
	if saNS == "" || saName == "" {
		return fmt.Errorf("deployment outputs must include %s and %s", OutputIngressServiceAccountNamespace, OutputIngressServiceAccountName)
	}

	// Validate outputs match the parameters we passed in ClusterProvision
	expectedNS := kube.IngressNamespace(cluster)
	expectedName := kube.IngressServiceAccountName(cluster)
	if saNS != expectedNS || saName != expectedName {
		return fmt.Errorf("deployment outputs mismatch: expected %s/%s, got %s/%s", expectedNS, expectedName, saNS, saName)
	}

	// Create the ServiceAccount idempotently in the specified namespace
	if err := kc.CreateServiceAccount(ctx, saNS, saName); err != nil {
		return fmt.Errorf("create ingress serviceaccount %s/%s: %w", saNS, saName, err)
	}

	// Step 2: Install Traefik via manifests (idempotent)
	if err := installer.InstallTraefik(ctx, cluster); err != nil {
		return err
	}

	log.Info(ctx, "aks cluster install succeeded", "cluster", cluster.Name, "provider", d.ProviderName())
	return nil
}

// ClusterUninstall uninstalls in-cluster resources (Ingress Controller, etc.) from AKS cluster.
func (d *driver) ClusterUninstall(ctx context.Context, cluster *model.Cluster) error {
	log := logging.FromContext(ctx)

	// Retrieve AKS resource identifiers for logging.
	if aksClient, aksRG, aksName, err := d.getAKSClient(ctx, cluster); err == nil && aksClient != nil {
		log.Info(ctx, "aks cluster uninstall begin",
			"aks_resource_group", aksRG,
			"aks_cluster", aksName,
			"cluster", cluster.Name,
			"provider", d.ProviderName(),
		)
	} else if err != nil { // only debug log on failure to resolve prior to kubeconfig
		log.Debug(ctx, "failed to resolve aks identifiers before uninstall", "error", err, "cluster", cluster.Name, "provider", d.ProviderName())
	}

	// Build kube client from provider-managed kubeconfig
	kubeconfig, err := d.ClusterKubeconfig(ctx, cluster)
	if err != nil {
		return fmt.Errorf("get kubeconfig: %w", err)
	}
	kc, err := kube.NewClientFromKubeconfig(ctx, kubeconfig, &kube.Options{UserAgent: "kompoxops"})
	if err != nil {
		return fmt.Errorf("new kube client: %w", err)
	}
	installer := kube.NewInstallerWithKubeconfig(kc, kubeconfig)

	// Step 1: Uninstall Traefik (best-effort)
	if err := installer.UninstallTraefik(ctx, cluster); err != nil {
		return err
	}

	// Step 2: Delete ingress namespace (best-effort, idempotent)
	if err := installer.DeleteIngressNamespace(ctx, cluster); err != nil {
		return err
	}

	log.Info(ctx, "aks cluster uninstall succeeded", "cluster", cluster.Name, "provider", d.ProviderName())
	return nil
}

// ClusterKubeconfig returns admin kubeconfig bytes for the AKS cluster.
func (d *driver) ClusterKubeconfig(ctx context.Context, cluster *model.Cluster) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	// Get AKS client and resource information
	aksClient, aksRGName, aksName, err := d.getAKSClient(ctx, cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to get AKS client and resource info: %w", err)
	}

	credResult, err := aksClient.ListClusterAdminCredentials(ctx, aksRGName, aksName, &armcontainerservice.ManagedClustersClientListClusterAdminCredentialsOptions{ServerFqdn: nil})
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster credentials: %w", err)
	}
	if len(credResult.Kubeconfigs) == 0 || len(credResult.Kubeconfigs[0].Value) == 0 {
		return nil, fmt.Errorf("no kubeconfig found for cluster")
	}
	return credResult.Kubeconfigs[0].Value, nil
}
