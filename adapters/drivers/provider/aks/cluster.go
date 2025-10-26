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
func (d *driver) ClusterProvision(ctx context.Context, cluster *model.Cluster, opts ...model.ClusterProvisionOption) (err error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	ctx, cleanup := d.withMethodLogger(ctx, "ClusterProvision")
	defer func() { cleanup(err) }()

	// Derive resource group name
	rgName, err := d.clusterResourceGroupName(cluster)
	if err != nil || rgName == "" {
		return fmt.Errorf("derive cluster resource group: %w", err)
	}

	tags := d.clusterResourceTags(cluster.Name)

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

	return nil
}

// ClusterDeprovision deprovisions an AKS cluster by deleting the entire resource group.
func (d *driver) ClusterDeprovision(ctx context.Context, cluster *model.Cluster, _ ...model.ClusterDeprovisionOption) (err error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	ctx, cleanup := d.withMethodLogger(ctx, "ClusterDeprovision")
	defer func() { cleanup(err) }()

	resourceGroupName, err := d.clusterResourceGroupName(cluster)
	if err != nil || resourceGroupName == "" {
		return fmt.Errorf("derive cluster resource group: %w", err)
	}

	// First, delete the subscription-scoped deployment if it exists (idempotent, best-effort)
	d.ensureAzureDeploymentDeleted(ctx, cluster)

	// Delete the entire resource group via shared helper (handles KV detection & purge)
	if err := d.ensureAzureResourceGroupDeleted(ctx, resourceGroupName); err != nil {
		return err
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
func (d *driver) ClusterInstall(ctx context.Context, cluster *model.Cluster, _ ...model.ClusterInstallOption) (err error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	ctx, cleanup := d.withMethodLogger(ctx, "ClusterInstall")
	defer func() { cleanup(err) }()

	log := logging.FromContext(ctx)

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

	// Step 4: Assign DNS Zone Contributor role for configured DNS zones (best-effort)
	// This allows AKS managed identity to update DNS records in Azure DNS zones.
	aksPrincipalID, _ := outputs[outputAksPrincipalID].(string)
	if aksPrincipalID != "" {
		zones, err := d.collectDNSZoneIDs(cluster)
		if err != nil {
			log.Warn(ctx, "failed to collect DNS zone IDs for role assignment (best-effort)", "error", err.Error())
		} else if len(zones) > 0 {
			d.ensureAzureDNSZoneRoles(ctx, aksPrincipalID, zones)
		}
	} else {
		log.Warn(ctx, "AKS principal ID not available, skipping DNS zone role assignments")
	}

	// Step 5: Assign AcrPull role for configured ACR resources (best-effort)
	// This allows AKS kubelet managed identity to pull images from Azure Container Registry.
	if aksPrincipalID != "" {
		registries, err := d.collectAzureContainerRegistryIDs(cluster)
		if err != nil {
			log.Warn(ctx, "failed to collect ACR resource IDs for role assignment (best-effort)", "error", err.Error())
		} else if len(registries) > 0 {
			d.ensureAzureContainerRegistryRoles(ctx, aksPrincipalID, registries)
		}
	} else {
		log.Warn(ctx, "AKS principal ID not available, skipping ACR role assignments")
	}

	// Step 6: Install Traefik via Helm (idempotent)
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

	return nil
}

// ClusterUninstall uninstalls in-cluster resources (Ingress Controller, etc.) from AKS cluster.
func (d *driver) ClusterUninstall(ctx context.Context, cluster *model.Cluster, _ ...model.ClusterUninstallOption) (err error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	ctx, cleanup := d.withMethodLogger(ctx, "ClusterUninstall")
	defer func() { cleanup(err) }()

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

	return nil
}

// ClusterKubeconfig returns admin kubeconfig bytes for the AKS cluster.
func (d *driver) ClusterKubeconfig(ctx context.Context, cluster *model.Cluster) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	return d.azureKubeconfig(ctx, cluster)
}

// ClusterDNSApply applies a DNS record set in Azure DNS zones.
// Implements zone resolution via ZoneHint or longest-match heuristics,
// validates input, and supports DryRun and Strict modes.
func (d *driver) ClusterDNSApply(ctx context.Context, cluster *model.Cluster, rset model.DNSRecordSet, opts ...model.ClusterDNSApplyOption) error {
	settings := &model.ClusterDNSApplyOptions{}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(settings)
	}

	log := logging.FromContext(ctx)
	clusterName := "(nil)"
	if cluster != nil {
		clusterName = cluster.Name
	}

	// Collect DNS zones from cluster settings
	zones, err := d.collectDNSZoneIDs(cluster)
	if err != nil {
		if settings.Strict {
			return fmt.Errorf("collect DNS zone IDs: %w", err)
		}
		log.Warn(ctx, "ClusterDNSApply: failed to collect DNS zone IDs", "error", err.Error())
		return nil
	}

	if len(zones) == 0 {
		err := fmt.Errorf("no DNS zones configured")
		if settings.Strict {
			return err
		}
		log.Warn(ctx, "ClusterDNSApply: no DNS zones configured")
		return nil
	}

	// Normalize and validate input
	if err := d.normalizeDNSRecordSet(&rset); err != nil {
		if settings.Strict {
			return fmt.Errorf("validate DNS record set: %w", err)
		}
		log.Warn(ctx, "ClusterDNSApply: invalid input", "error", err.Error())
		return nil
	}

	// Select DNS zone (ZoneHint or longest-match)
	zone, err := d.selectDNSZone(ctx, rset.FQDN, zones, settings.ZoneHint)
	if err != nil {
		if settings.Strict {
			return fmt.Errorf("select DNS zone: %w", err)
		}
		log.Warn(ctx, "ClusterDNSApply: zone resolution failed", "fqdn", rset.FQDN, "error", err.Error())
		return nil
	}

	// DryRun mode: show what would be done
	if settings.DryRun {
		action := "create/update"
		if len(rset.RData) == 0 {
			action = "delete"
		}
		log.Info(ctx, "ClusterDNSApply: dry-run",
			"action", action,
			"cluster", clusterName,
			"zone", zone.Name,
			"zone_id", zone.ResourceID,
			"fqdn", rset.FQDN,
			"type", rset.Type,
			"ttl", rset.TTL,
			"rdata", rset.RData,
			"strict", settings.Strict,
		)
		return nil
	}

	// Apply DNS record (upsert or delete)
	if len(rset.RData) == 0 {
		err = d.deleteAzureDNSRecord(ctx, zone, rset)
		if err != nil {
			if settings.Strict {
				return fmt.Errorf("delete DNS record: %w", err)
			}
			log.Warn(ctx, "ClusterDNSApply: failed to delete DNS record (best-effort)", "error", err.Error())
			return nil
		}
		log.Info(ctx, "ClusterDNSApply: deleted DNS record", "zone", zone.Name, "fqdn", rset.FQDN, "type", rset.Type)
	} else {
		err = d.upsertAzureDNSRecord(ctx, zone, rset)
		if err != nil {
			if settings.Strict {
				return fmt.Errorf("upsert DNS record: %w", err)
			}
			log.Warn(ctx, "ClusterDNSApply: failed to upsert DNS record (best-effort)", "error", err.Error())
			return nil
		}
		log.Info(ctx, "ClusterDNSApply: upserted DNS record", "zone", zone.Name, "fqdn", rset.FQDN, "type", rset.Type)
	}

	return nil
}
