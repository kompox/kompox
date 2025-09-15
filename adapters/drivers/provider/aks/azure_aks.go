package aks

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/kompox/kompox/adapters/kube"
	"github.com/kompox/kompox/domain/model"
	"github.com/kompox/kompox/internal/logging"
)

// Constants for template output keys
const (
	outputTenantID                         = "AZURE_TENANT_ID"
	outputResourceGroupName                = "AZURE_RESOURCE_GROUP_NAME"
	outputAksClusterName                   = "AZURE_AKS_CLUSTER_NAME"
	outputAksPrincipalID                   = "AZURE_AKS_PRINCIPAL_ID"
	outputIngressServiceAccountNamespace   = "AZURE_INGRESS_SERVICE_ACCOUNT_NAMESPACE"
	outputIngressServiceAccountName        = "AZURE_INGRESS_SERVICE_ACCOUNT_NAME"
	outputIngressServiceAccountClientID    = "AZURE_INGRESS_SERVICE_ACCOUNT_CLIENT_ID"
	outputIngressServiceAccountPrincipalID = "AZURE_INGRESS_SERVICE_ACCOUNT_PRINCIPAL_ID"
)

// paramSettingMap maps cluster setting keys to ARM template parameter names.
var paramSettingMap = []struct{ param, setting string }{
	{"aksSystemVmSize", "AZURE_AKS_SYSTEM_VM_SIZE"},
	{"aksSystemVmDiskType", "AZURE_AKS_SYSTEM_VM_DISK_TYPE"},
	{"aksSystemVmDiskSizeGB", "AZURE_AKS_SYSTEM_VM_DISK_SIZE_GB"},
	{"aksSystemVmPriority", "AZURE_AKS_SYSTEM_VM_PRIORITY"},
	{"aksSystemVmZones", "AZURE_AKS_SYSTEM_VM_ZONES"},
	{"aksUserVmSize", "AZURE_AKS_USER_VM_SIZE"},
	{"aksUserVmDiskType", "AZURE_AKS_USER_VM_DISK_TYPE"},
	{"aksUserVmDiskSizeGB", "AZURE_AKS_USER_VM_DISK_SIZE_GB"},
	{"aksUserVmPriority", "AZURE_AKS_USER_VM_PRIORITY"},
	{"aksUserVmZones", "AZURE_AKS_USER_VM_ZONES"},
}

// azureDeploymentName generates the deployment name for the subscription-scoped deployment.
// It returns the same name as the resource group name for consistency.
func (d *driver) azureDeploymentName(cluster *model.Cluster) (string, error) {
	return d.clusterResourceGroupName(cluster)
}

// ensureAzureDeploymentCreated creates or updates the subscription-scoped deployment for the cluster.
// If an existing deployment succeeded and force is false, it returns without changes (idempotent).
func (d *driver) ensureAzureDeploymentCreated(ctx context.Context, cluster *model.Cluster, resourceGroupName string, tags map[string]*string, force bool) error {
	log := logging.FromContext(ctx)

	// Unmarshal embedded ARM template (subscription scope)
	var template map[string]any
	if err := json.Unmarshal(mainJSON, &template); err != nil {
		return fmt.Errorf("unmarshal embedded template: %w", err)
	}

	// Prepare ARM parameters for subscription-scoped deployment
	parameters := map[string]any{
		"environmentName":                map[string]any{"value": cluster.Name},
		"location":                       map[string]any{"value": d.AzureLocation},
		"resourceGroupName":              map[string]any{"value": resourceGroupName},
		"ingressServiceAccountName":      map[string]any{"value": kube.IngressServiceAccountName(cluster)},
		"ingressServiceAccountNamespace": map[string]any{"value": kube.IngressNamespace(cluster)},
		"tags":                           map[string]any{"value": tags},
	}

	// Load parameters from cluster settings if present
	for _, mapping := range paramSettingMap {
		if val, exists := cluster.Settings[mapping.setting]; exists {
			parameters[mapping.param] = map[string]any{"value": val}
		}
	}

	// Create deployments client for subscription-scoped deployment
	deploymentsClient, err := armresources.NewDeploymentsClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return fmt.Errorf("create deployments client: %w", err)
	}

	depName, err := d.azureDeploymentName(cluster)
	if err != nil {
		return fmt.Errorf("derive deployment name: %w", err)
	}

	// Check if deployment already exists and is successful (idempotent unless forced)
	if existing, err := deploymentsClient.GetAtSubscriptionScope(ctx, depName, nil); err == nil {
		if existing.Properties != nil && existing.Properties.ProvisioningState != nil &&
			*existing.Properties.ProvisioningState == "Succeeded" {
			if !force {
				log.Info(ctx, "aks cluster already provisioned",
					"resource_group", resourceGroupName,
					"cluster", cluster.Name,
					"provider", d.ProviderName(),
				)
				return nil
			}
			log.Info(ctx, "force-redeploy enabled, proceeding despite existing successful deployment",
				"deployment", depName,
				"resource_group", resourceGroupName,
				"cluster", cluster.Name,
			)
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
		Tags: tags,
	}

	poller, err := deploymentsClient.BeginCreateOrUpdateAtSubscriptionScope(ctx, depName, deployment, nil)
	if err != nil {
		return fmt.Errorf("begin subscription deployment creation: %w", err)
	}
	if _, err = poller.PollUntilDone(ctx, nil); err != nil {
		return fmt.Errorf("subscription deployment creation failed: %w", err)
	}

	return nil
}

// ensureAzureDeploymentDeleted best-effort deletes the subscription-scoped deployment for the cluster.
// All errors are logged at debug level and ignored for idempotency.
func (d *driver) ensureAzureDeploymentDeleted(ctx context.Context, cluster *model.Cluster) {
	log := logging.FromContext(ctx)
	depClient, err := armresources.NewDeploymentsClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return
	}
	depName, err := d.azureDeploymentName(cluster)
	if err != nil {
		return
	}
	log.Info(ctx, "deleting subscription-scoped deployment (best-effort)",
		"deployment", depName,
		"cluster", cluster.Name,
		"provider", d.ProviderName(),
	)
	poller, err := depClient.BeginDeleteAtSubscriptionScope(ctx, depName, nil)
	if err != nil {
		return
	}
	if _, err = poller.PollUntilDone(ctx, nil); err != nil {
		log.Debug(ctx, "deployment deletion error ignored", "deployment", depName, "error", err)
	}
}

// azureDeploymentOutputs retrieves the outputs from the subscription-scoped deployment.
func (d *driver) azureDeploymentOutputs(ctx context.Context, cluster *model.Cluster) (map[string]any, error) {
	deploymentsClient, err := armresources.NewDeploymentsClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create deployments client: %w", err)
	}

	depName, err := d.azureDeploymentName(cluster)
	if err != nil {
		return nil, fmt.Errorf("derive deployment name: %w", err)
	}
	deployment, err := deploymentsClient.GetAtSubscriptionScope(ctx, depName, nil)
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

// azureKubeconfig retrieves the admin kubeconfig for the AKS cluster resolved from deployment outputs.
func (d *driver) azureKubeconfig(ctx context.Context, cluster *model.Cluster) ([]byte, error) {
	// Get outputs from deployment to resolve resource group and cluster name
	outputs, err := d.azureDeploymentOutputs(ctx, cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment outputs: %w", err)
	}

	aksRGName, ok := outputs[outputResourceGroupName].(string)
	if !ok {
		return nil, fmt.Errorf("%s not found in deployment outputs", outputResourceGroupName)
	}
	aksName, ok := outputs[outputAksClusterName].(string)
	if !ok {
		return nil, fmt.Errorf("%s not found in deployment outputs", outputAksClusterName)
	}

	// Create AKS client and request admin credentials
	aksClient, err := armcontainerservice.NewManagedClustersClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create AKS client: %w", err)
	}

	credResult, err := aksClient.ListClusterAdminCredentials(ctx, aksRGName, aksName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster credentials: %w", err)
	}
	if len(credResult.Kubeconfigs) == 0 || len(credResult.Kubeconfigs[0].Value) == 0 {
		return nil, fmt.Errorf("no kubeconfig found for cluster")
	}
	return credResult.Kubeconfigs[0].Value, nil
}
