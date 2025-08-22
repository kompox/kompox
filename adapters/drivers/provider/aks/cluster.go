package aks

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armdeploymentstacks"
	"github.com/yaegashi/kompoxops/adapters/kube"
	"github.com/yaegashi/kompoxops/domain/model"
	"github.com/yaegashi/kompoxops/internal/logging"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Constants for deployment stack output keys
const (
	StackOutputResourceGroupName = "AZURE_RESOURCE_GROUP_NAME"
	StackOutputAksClusterName    = "AZURE_AKS_CLUSTER_NAME"
)

// Deployment stack name now equals cluster resource group name for determinism.

// clusterTagValue generates the cluster tag value for resource tagging.
func (d *driver) clusterTagValue(clusterName string) string {
	return fmt.Sprintf("%s/%s/%s", d.ServiceName(), d.ProviderName(), clusterName)
}

// getDeploymentStackOutputs retrieves the outputs from the deployment stack.
func (d *driver) getDeploymentStackOutputs(ctx context.Context, cluster *model.Cluster) (map[string]any, error) {
	stacksClient, err := armdeploymentstacks.NewClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create deployment stacks client: %w", err)
	}

	deploymentStackName, err := d.clusterResourceGroupName(cluster)
	if err != nil {
		return nil, fmt.Errorf("derive deployment stack name: %w", err)
	}
	stack, err := stacksClient.GetAtSubscription(ctx, deploymentStackName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment stack: %w", err)
	}

	if stack.Properties == nil || stack.Properties.Outputs == nil {
		return nil, fmt.Errorf("deployment stack has no outputs")
	}

	// Type assert the outputs to the correct map type
	outputsMap, ok := stack.Properties.Outputs.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("deployment stack outputs has unexpected type")
	}

	outputs := make(map[string]any)
	for key, value := range outputsMap {
		if outputValue, ok := value.(map[string]any); ok {
			if val, exists := outputValue["value"]; exists {
				// ARM does not preserve alphabet case in output keys, so normalization is required
				outputs[strings.ToUpper(key)] = val
			}
		}
	}

	return outputs, nil
}

// getAKSClient creates a new AKS client and retrieves resource information from deployment stack.
func (d *driver) getAKSClient(ctx context.Context, cluster *model.Cluster) (*armcontainerservice.ManagedClustersClient, string, string, error) {
	// Get outputs from deployment stack
	outputs, err := d.getDeploymentStackOutputs(ctx, cluster)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to get deployment stack outputs: %w", err)
	}

	// Extract resource group and cluster names from outputs
	aksRGName, ok := outputs[StackOutputResourceGroupName].(string)
	if !ok {
		return nil, "", "", fmt.Errorf("%s not found in deployment stack outputs", StackOutputResourceGroupName)
	}

	aksName, ok := outputs[StackOutputAksClusterName].(string)
	if !ok {
		return nil, "", "", fmt.Errorf("%s not found in deployment stack outputs", StackOutputAksClusterName)
	}

	// Create AKS client
	aksClient, err := armcontainerservice.NewManagedClustersClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to create AKS client: %w", err)
	}

	return aksClient, aksRGName, aksName, nil
}

// ClusterProvision provisions an AKS cluster according to the cluster specification.
func (d *driver) ClusterProvision(ctx context.Context, cluster *model.Cluster) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	log := logging.FromContext(ctx)

	// Derive or override resource group name
	resourceGroupName, err := d.clusterResourceGroupName(cluster)
	if err != nil || resourceGroupName == "" {
		return fmt.Errorf("derive cluster resource group: %w", err)
	}

	// Unmarshal embedded ARM template (subscription scope)
	var template map[string]any
	if err := json.Unmarshal(mainJSON, &template); err != nil {
		return fmt.Errorf("unmarshal embedded template: %w", err)
	}

	// Prepare ARM parameters (object with value fields)
	parameters := map[string]*armdeploymentstacks.DeploymentParameter{
		"environmentName": {
			Value: cluster.Name,
		},
		"location": {
			Value: d.AzureLocation,
		},
		"resourceGroupName": {
			Value: resourceGroupName,
		},
	}

	// Create deployment stacks client
	stacksClient, err := armdeploymentstacks.NewClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return fmt.Errorf("create deployment stacks client: %w", err)
	}

	deploymentStackName, err := d.clusterResourceGroupName(cluster)
	if err != nil {
		return fmt.Errorf("derive deployment stack name: %w", err)
	}

	log.Info(ctx, "aks cluster provision begin",
		"deployment_stack", deploymentStackName,
		"resource_group", resourceGroupName,
		"cluster", cluster.Name,
		"provider", d.ProviderName(),
		"subscription", d.AzureSubscriptionId,
	)

	// If an existing successful deployment stack with same name exists, treat as done (idempotent)
	if existing, err := stacksClient.GetAtSubscription(ctx, deploymentStackName, nil); err == nil {
		if existing.Properties != nil && existing.Properties.ProvisioningState != nil &&
			*existing.Properties.ProvisioningState == armdeploymentstacks.DeploymentStackProvisioningStateSucceeded {
			return nil
		}
		// fallthrough: re-issue deployment to converge
	}

	// Create deployment stack
	deploymentStack := armdeploymentstacks.DeploymentStack{
		Location: to.Ptr(d.AzureLocation),
		Properties: &armdeploymentstacks.DeploymentStackProperties{
			Template:   template,
			Parameters: parameters,
			ActionOnUnmanage: &armdeploymentstacks.ActionOnUnmanage{
				Resources:        to.Ptr(armdeploymentstacks.DeploymentStacksDeleteDetachEnumDetach),
				ResourceGroups:   to.Ptr(armdeploymentstacks.DeploymentStacksDeleteDetachEnumDetach),
				ManagementGroups: to.Ptr(armdeploymentstacks.DeploymentStacksDeleteDetachEnumDetach),
			},
			DenySettings: &armdeploymentstacks.DenySettings{
				Mode: to.Ptr(armdeploymentstacks.DenySettingsModeNone),
			},
		},
		Tags: map[string]*string{
			"kompox-cluster": to.Ptr(d.clusterTagValue(cluster.Name)),
			"managed-by":     to.Ptr("kompox"),
		},
	}

	poller, err := stacksClient.BeginCreateOrUpdateAtSubscription(ctx, deploymentStackName, deploymentStack, nil)
	if err != nil {
		return fmt.Errorf("begin subscription deployment stack creation: %w", err)
	}
	if _, err = poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("subscription deployment stack creation failed: %w", err)
	}

	log.Info(ctx, "aks cluster provision succeeded",
		"deployment_stack", deploymentStackName,
		"resource_group", resourceGroupName,
		"cluster", cluster.Name,
		"provider", d.ProviderName(),
	)
	return nil
}

// ClusterDeprovision deprovisions an AKS cluster according to the cluster specification.
func (d *driver) ClusterDeprovision(ctx context.Context, cluster *model.Cluster) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	log := logging.FromContext(ctx)

	resourceGroupName, err := d.clusterResourceGroupName(cluster)
	if err != nil || resourceGroupName == "" {
		return fmt.Errorf("derive cluster resource group: %w", err)
	}

	// Create deployment stacks client
	stacksClient, err := armdeploymentstacks.NewClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return fmt.Errorf("failed to create deployment stacks client: %w", err)
	}

	deploymentStackName, err := d.clusterResourceGroupName(cluster)
	if err != nil {
		return fmt.Errorf("derive deployment stack name: %w", err)
	}

	// Check if deployment stack exists
	_, err = stacksClient.GetAtSubscription(ctx, deploymentStackName, nil)
	if err != nil {
		// If deployment stack doesn't exist, consider it already deprovisioned
		return nil
	}

	// Delete the deployment stack with all managed resources
	log.Info(ctx, "aks cluster deprovision begin",
		"deployment_stack", deploymentStackName,
		"resource_group", resourceGroupName,
		"cluster", cluster.Name,
		"provider", d.ProviderName(),
	)
	poller, err := stacksClient.BeginDeleteAtSubscription(ctx, deploymentStackName, &armdeploymentstacks.ClientBeginDeleteAtSubscriptionOptions{
		UnmanageActionResources:        to.Ptr(armdeploymentstacks.UnmanageActionResourceModeDelete),
		UnmanageActionResourceGroups:   to.Ptr(armdeploymentstacks.UnmanageActionResourceGroupModeDelete),
		UnmanageActionManagementGroups: to.Ptr(armdeploymentstacks.UnmanageActionManagementGroupModeDelete),
	})
	if err != nil {
		return fmt.Errorf("failed to start deployment stack deletion: %w", err)
	}

	// Wait for completion
	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to delete deployment stack %s: %w", deploymentStackName, err)
	}

	log.Info(ctx, "aks cluster deprovision succeeded",
		"deployment_stack", deploymentStackName,
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

	// Create deployment stacks client to check the stack status
	stacksClient, err := armdeploymentstacks.NewClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return status, fmt.Errorf("failed to create deployment stacks client: %w", err)
	}

	deploymentStackName, err := d.clusterResourceGroupName(cluster)
	if err != nil {
		return status, fmt.Errorf("derive deployment stack name: %w", err)
	}

	// Check deployment stack status
	stack, err := stacksClient.GetAtSubscription(ctx, deploymentStackName, nil)
	if err != nil {
		// Deployment stack doesn't exist
		return status, nil
	}

	if stack.Properties == nil || stack.Properties.ProvisioningState == nil {
		return status, nil
	}

	// Check if deployment stack is provisioned successfully
	if *stack.Properties.ProvisioningState == armdeploymentstacks.DeploymentStackProvisioningStateSucceeded {
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
