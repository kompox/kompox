package aks

import (
	"context"
	"fmt"
	"time"

	"github.com/kompox/kompox/adapters/kube"
	"github.com/kompox/kompox/domain/model"
	"github.com/kompox/kompox/internal/logging"
	"sigs.k8s.io/yaml"
)

// kubeClient returns a Kubernetes client for the target cluster.
func (d *driver) kubeClient(ctx context.Context, cluster *model.Cluster) (*kube.Client, error) {
	kubeconfig, err := d.azureKubeconfig(ctx, cluster)
	if err != nil {
		return nil, fmt.Errorf("get kubeconfig: %w", err)
	}
	kc, err := kube.NewClientFromKubeconfig(ctx, kubeconfig, &kube.Options{UserAgent: "kompoxops"})
	if err != nil {
		return nil, fmt.Errorf("new kube client: %w", err)
	}
	return kc, nil
}

// ClusterProvision provisions an AKS cluster according to the cluster specification.
func (d *driver) ClusterProvision(ctx context.Context, cluster *model.Cluster, opts ...model.ClusterProvisionOption) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	log := logging.FromContext(ctx)

	// Derive resource group name
	rgName, err := d.clusterResourceGroupName(cluster)
	if err != nil || rgName == "" {
		return fmt.Errorf("derive cluster resource group: %w", err)
	}

	tags := d.clusterResourceTags(cluster.Name)

	log.Info(ctx, "aks cluster provision begin",
		"subscription", d.AzureSubscriptionId,
		"resource_group", rgName,
		"tags", tagsForLog(tags),
	)

	// Resolve options
	var o model.ClusterProvisionOptions
	for _, fn := range opts {
		if fn != nil {
			fn(&o)
		}
	}
	// Ensure subscription-scoped deployment (idempotent; respects force)
	if err := d.ensureAzureDeploymentCreated(ctx, cluster, rgName, tags, o.Force); err != nil {
		return err
	}

	log.Info(ctx, "aks cluster provision succeeded",
		"subscription", d.AzureSubscriptionId,
		"resource_group", rgName,
	)
	return nil
}

// ClusterDeprovision deprovisions an AKS cluster by deleting the entire resource group.
func (d *driver) ClusterDeprovision(ctx context.Context, cluster *model.Cluster, _ ...model.ClusterDeprovisionOption) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	log := logging.FromContext(ctx)

	resourceGroupName, err := d.clusterResourceGroupName(cluster)
	if err != nil || resourceGroupName == "" {
		return fmt.Errorf("derive cluster resource group: %w", err)
	}

	// First, delete the subscription-scoped deployment if it exists (idempotent, best-effort)
	d.ensureAzureDeploymentDeleted(ctx, cluster)

	log.Info(ctx, "aks cluster deprovision begin",
		"resource_group", resourceGroupName,
		"cluster", cluster.Name,
		"provider", d.ProviderName(),
	)

	// Delete the entire resource group via shared helper (handles KV detection & purge)
	if err := d.ensureAzureResourceGroupDeleted(ctx, resourceGroupName); err != nil {
		return err
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

	// Check if the AKS cluster exists by attempting to get the kube client
	kc, err := d.kubeClient(ctx, cluster)
	if err == nil {
		status.Provisioned = true

		// Retrieve ingress endpoint (global IP/FQDN) when installed
		ip, host, err := kc.IngressEndpoint(ctx, cluster)
		if err == nil {
			status.Installed = true
			status.IngressGlobalIP = ip
			status.IngressFQDN = host
		}
	}

	return status, nil
}

// ClusterInstall installs in-cluster resources (Ingress Controller, etc.) for AKS cluster.
func (d *driver) ClusterInstall(ctx context.Context, cluster *model.Cluster, _ ...model.ClusterInstallOption) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	log := logging.FromContext(ctx)
	log.Info(ctx, "aks cluster install begin", "cluster", cluster.Name, "provider", d.ProviderName())

	// Build kube client from provider-managed kubeconfig
	kc, err := d.kubeClient(ctx, cluster)
	if err != nil {
		return err
	}

	// Step 1: Ensure ingress namespace exists (idempotent)
	if err := kc.CreateNamespace(ctx, kube.IngressNamespace(cluster)); err != nil {
		return err
	}

	// Step 2: Create ServiceAccount exactly as specified by deployment outputs
	outputs, err := d.azureDeploymentOutputs(ctx, cluster)
	if err != nil {
		return fmt.Errorf("read deployment outputs: %w", err)
	}
	saNS, _ := outputs[outputIngressServiceAccountNamespace].(string)
	saName, _ := outputs[outputIngressServiceAccountName].(string)
	if saNS == "" || saName == "" {
		return fmt.Errorf("deployment outputs must include %s and %s", outputIngressServiceAccountNamespace, outputIngressServiceAccountName)
	}

	// Validate outputs match the parameters we passed in ClusterProvision
	expectedNS := kube.IngressNamespace(cluster)
	expectedName := kube.IngressServiceAccountName(cluster)
	if saNS != expectedNS || saName != expectedName {
		return fmt.Errorf("deployment outputs mismatch: expected %s/%s, got %s/%s", expectedNS, expectedName, saNS, saName)
	}

	// Fetch workload identity tenant/client ID from deployment outputs and annotate the ServiceAccount
	tenantID, _ := outputs[outputTenantID].(string)
	if tenantID == "" {
		return fmt.Errorf("deployment outputs must include non-empty %s", outputTenantID)
	}
	clientID, _ := outputs[outputIngressServiceAccountClientID].(string)
	if clientID == "" {
		return fmt.Errorf("deployment outputs must include non-empty %s", outputIngressServiceAccountClientID)
	}
	annotations := map[string]string{
		"azure.workload.identity/tenant-id": tenantID,
		"azure.workload.identity/client-id": clientID,
	}

	// Create or update the ServiceAccount idempotently with annotations
	if err := kc.CreateServiceAccount(ctx, saNS, saName, annotations); err != nil {
		return fmt.Errorf("create ingress serviceaccount %s/%s: %w", saNS, saName, err)
	}

	// Step 3: If static certificates are configured, ensure SecretProviderClass and TLS Secrets from Key Vault
	var mounts []map[string]string // {name, mountPath}
	var configs []map[string]any
	if cluster.Ingress != nil && len(cluster.Ingress.Certificates) > 0 {
		// Create SPCs and get mounts + cert files list to generate certs.yaml
		mounts, configs, err = d.ensureSecretProviderClassFromKeyVault(ctx, kc, cluster, tenantID, clientID)
		if err != nil {
			return fmt.Errorf("ensure ingress TLS from key vault: %w", err)
		}
	}

	// Step 4: Install Traefik via Helm (idempotent)
	// Mount SecretProviderClass volumes created in ensureSecretProviderClassFromKeyVault.
	// For multiple Key Vaults, mount one CSI volume per SPC with distinct mount paths.
	mutator := func(ctx context.Context, _ *model.Cluster, _ string, values kube.HelmValues) {
		// Ensure the nested map at values["deployment"] exists first.
		dep, _ := values["deployment"].(map[string]any)
		if dep == nil {
			dep = map[string]any{}
			values["deployment"] = dep
		}
		// Ensure pod label for Workload Identity remains.
		if pl, ok := dep["podLabels"].(map[string]any); ok {
			pl["azure.workload.identity/use"] = "true"
		} else {
			dep["podLabels"] = map[string]any{"azure.workload.identity/use": "true"}
		}
		// Add one volume per SPC mount (captured from outer scope)
		if len(mounts) > 0 {
			for i, m := range mounts {
				vol := map[string]any{
					"name": fmt.Sprintf("secrets-store-inline-%d", i),
					"csi": map[string]any{
						"driver":   "secrets-store.csi.k8s.io",
						"readOnly": true,
						"volumeAttributes": map[string]any{
							"secretProviderClass": m["name"],
						},
					},
				}
				if av, ok := dep["additionalVolumes"].([]any); ok {
					dep["additionalVolumes"] = append(av, vol)
				} else {
					dep["additionalVolumes"] = []any{vol}
				}

				vm := map[string]any{
					"name":      fmt.Sprintf("secrets-store-inline-%d", i),
					"mountPath": m["mountPath"],
					"readOnly":  true,
				}
				if avm, ok := values["additionalVolumeMounts"].([]any); ok {
					values["additionalVolumeMounts"] = append(avm, vm)
				} else {
					values["additionalVolumeMounts"] = []any{vm}
				}
			}
		}
		// Inject certs.yaml if provided by driver
		if len(configs) > 0 {
			m := map[string]any{"tls": map[string]any{"certificates": configs}}
			if b, err := yaml.Marshal(m); err == nil {
				add, _ := values["additionalConfigFiles"].(map[string]string)
				if add == nil {
					add = map[string]string{}
				}
				add["certs.yaml"] = string(b)
				values["additionalConfigFiles"] = add
			}
		}
	}
	if cluster.Ingress != nil && len(cluster.Ingress.Certificates) > 0 {
		if err := kc.InstallIngressTraefik(ctx, cluster, mutator); err != nil {
			return err
		}
	} else {
		if err := kc.InstallIngressTraefik(ctx, cluster); err != nil {
			return err
		}
	}

	log.Info(ctx, "aks cluster install succeeded", "cluster", cluster.Name, "provider", d.ProviderName())
	return nil
}

// ClusterUninstall uninstalls in-cluster resources (Ingress Controller, etc.) from AKS cluster.
func (d *driver) ClusterUninstall(ctx context.Context, cluster *model.Cluster, _ ...model.ClusterUninstallOption) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	log := logging.FromContext(ctx)
	log.Info(ctx, "aks cluster uninstall begin", "cluster", cluster.Name, "provider", d.ProviderName())

	// Build kube client from provider-managed kubeconfig
	kc, err := d.kubeClient(ctx, cluster)
	if err != nil {
		return err
	}

	// Step 1: Uninstall Traefik (best-effort)
	if err := kc.UninstallIngressTraefik(ctx, cluster); err != nil {
		return err
	}

	// Step 2: Delete ingress namespace (best-effort, idempotent)
	if err := kc.DeleteNamespace(ctx, kube.IngressNamespace(cluster)); err != nil {
		return err
	}

	log.Info(ctx, "aks cluster uninstall succeeded", "cluster", cluster.Name, "provider", d.ProviderName())
	return nil
}

// ClusterKubeconfig returns admin kubeconfig bytes for the AKS cluster.
func (d *driver) ClusterKubeconfig(ctx context.Context, cluster *model.Cluster) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	return d.azureKubeconfig(ctx, cluster)
}
