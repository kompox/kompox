package kube

import (
	"context"
	stdErrors "errors"
	"fmt"
	"time"

	"github.com/kompox/kompox/domain/model"
	"github.com/kompox/kompox/internal/logging"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	helmdriver "helm.sh/helm/v3/pkg/storage/driver"
	"sigs.k8s.io/yaml"
)

// InstallIngressTraefik installs or upgrades a minimal Traefik ingress controller into the ingress namespace.
// This uses the Helm SDK with a temporary kubeconfig file derived from this client.
//
// A provider may pass optional mutators to customize Helm values before install/upgrade
// to support provider-specific needs (e.g., mounting SecretProviderClass via CSI driver).
func (c *Client) InstallIngressTraefik(ctx context.Context, cluster *model.Cluster, mutators ...HelmValuesMutator) error {
	if c == nil || c.RESTConfig == nil {
		return fmt.Errorf("kube client is not initialized")
	}

	logger := logging.FromContext(ctx)
	msgSym := "KubeClient:InstallIngressTraefik"

	kubeBytes := c.Kubeconfig()
	if len(kubeBytes) == 0 {
		return fmt.Errorf("kubeconfig is required for Helm operations")
	}
	ns := IngressNamespace(cluster)
	if err := c.CreateNamespace(ctx, ns); err != nil {
		return err
	}

	// Prepare a temporary kubeconfig file for Helm SDK
	kubeconfigPath, cleanup, err := tempfile(kubeBytes)
	if err != nil {
		return err
	}
	defer cleanup()

	settings := cli.New()
	settings.KubeConfig = kubeconfigPath

	cfg := new(action.Configuration)
	if err := cfg.Init(settings.RESTClientGetter(), ns, "secret", func(format string, v ...any) {}); err != nil {
		return fmt.Errorf("init helm configuration: %w", err)
	}

	// Locate and load the Traefik chart from official repo
	cpo := action.ChartPathOptions{RepoURL: "https://helm.traefik.io/traefik"}
	chartPath, err := cpo.LocateChart(TraefikReleaseName, settings)
	if err != nil {
		return fmt.Errorf("locate traefik chart: %w", err)
	}
	ch, err := loader.Load(chartPath)
	if err != nil {
		return fmt.Errorf("load traefik chart: %w", err)
	}

	// Minimal values with ACME and persistence
	saName := IngressServiceAccountName(cluster)
	values := HelmValues{
		"service": map[string]any{
			"type": "LoadBalancer",
		},
		// Use Recreate strategy to avoid deadlocks
		"updateStrategy": map[string]any{
			"type": "Recreate",
		},
		// Use the pre-created ServiceAccount for ingress/workload-identity.
		// Helm should not attempt to create another ServiceAccount.
		"serviceAccount": map[string]any{
			"name": saName,
		},
		// Enable persistence for Let's Encrypt accounts and certificates
		"persistence": map[string]any{
			"enabled":    true,
			"accessMode": "ReadWriteOnce",
			"size":       "1Gi",
			"path":       "/data",
		},
		// Enable Traefik access logs
		"logs": map[string]any{
			"access": map[string]any{
				"enabled": true,
			},
		},
		// Ensure mounted PVC at /data is group-writable for the Traefik user (65532)
		"podSecurityContext": map[string]any{
			"fsGroup":             65532,
			"fsGroupChangePolicy": "OnRootMismatch",
		},
		// Ensure AKS Workload Identity is enabled on Traefik Pods by adding the required label.
		// This relies on a pre-created ServiceAccount annotated for Azure Workload Identity.
		"deployment": map[string]any{
			"podLabels": map[string]any{
				"azure.workload.identity/use": "true",
			},
		},
		// Ensure the Traefik pods are only on nodes labeled with kompox.dev/node-pool=system.
		"nodeSelector": map[string]any{
			"kompox.dev/node-pool": "system",
		},
		// Will populate below
		"additionalArguments": []any{},
	}

	// Configure Let's Encrypt (ACME) resolvers for staging and production
	certEmail := ""
	if cluster != nil && cluster.Ingress != nil {
		certEmail = cluster.Ingress.CertEmail
	}
	if certEmail == "" {
		// Fallback placeholder; users should configure a real email in cluster config
		certEmail = "noreply@example.com"
	}
	addArgs := []any{
		"--certificatesresolvers.production.acme.tlschallenge=true",
		"--certificatesresolvers.production.acme.caserver=https://acme-v02.api.letsencrypt.org/directory",
		fmt.Sprintf("--certificatesresolvers.production.acme.email=%s", certEmail),
		"--certificatesresolvers.production.acme.storage=/data/acme-production.json",
		"--certificatesresolvers.staging.acme.tlschallenge=true",
		"--certificatesresolvers.staging.acme.caserver=https://acme-staging-v02.api.letsencrypt.org/directory",
		fmt.Sprintf("--certificatesresolvers.staging.acme.email=%s", certEmail),
		"--certificatesresolvers.staging.acme.storage=/data/acme-staging.json",
		"--providers.file.directory=/config/traefik",
		"--providers.file.watch=true",
	}
	values["additionalArguments"] = addArgs

	dep, _ := values["deployment"].(map[string]any)
	if dep == nil {
		dep = map[string]any{}
		values["deployment"] = dep
	}
	vol := map[string]any{
		"name":      "traefik",
		"configMap": map[string]any{"name": "traefik"},
	}
	if av, ok := dep["additionalVolumes"].([]any); ok {
		dep["additionalVolumes"] = append(av, vol)
	} else {
		dep["additionalVolumes"] = []any{vol}
	}
	vm := map[string]any{
		"name":      "traefik",
		"mountPath": "/config/traefik",
		"readOnly":  true,
	}
	if avm, ok := values["additionalVolumeMounts"].([]any); ok {
		values["additionalVolumeMounts"] = append(avm, vm)
	} else {
		values["additionalVolumeMounts"] = []any{vm}
	}

	// Apply provider-specific value customizations, if any.
	for _, m := range mutators {
		if m != nil {
			m(ctx, cluster, TraefikReleaseName, values)
		}
	}

	// Build ConfigMap data for file provider.
	cmData := map[string]any{}
	// Provider extension point: all config files supplied by providers via values["additionalConfigFiles"].
	if ext, ok := values["additionalConfigFiles"].(map[string]string); ok {
		for k, v := range ext {
			if k == "" {
				continue
			}
			cmData[k] = v
		}
		// remove from Helm values after transferring to ConfigMap
		delete(values, "additionalConfigFiles")
	}
	// If no files were supplied, keep an empty object to retain valid YAML
	if len(cmData) == 0 {
		cmData = map[string]any{}
	}
	cm := map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]any{
			"name":      "traefik",
			"namespace": ns,
		},
		"data": cmData,
	}

	// Debug log output
	if b, err := yaml.Marshal(cmData); err == nil {
		logger.Debugf(ctx, msgSym+":ConfigMapData\n%s", string(b))
	}
	if b, err := yaml.Marshal(values); err == nil {
		logger.Debugf(ctx, msgSym+":HelmValues\n%s", string(b))
	}

	// Apply ConfigMap
	raw, err := yaml.Marshal(cm)
	if err != nil {
		return fmt.Errorf("marshal traefik file provider configmap: %w", err)
	}
	if err := c.ApplyYAML(ctx, raw, &ApplyOptions{DefaultNamespace: ns}); err != nil {
		return fmt.Errorf("apply traefik file provider configmap: %w", err)
	}

	// Try upgrade first; if the release doesn't exist, fallback to install
	up := action.NewUpgrade(cfg)
	up.Namespace = ns
	up.Atomic = true
	up.Wait = true
	up.Timeout = 5 * time.Minute
	upLogger := logger.With("ns", ns, "release", TraefikReleaseName)
	upLogger.Info(ctx, msgSym+":Upgrade/s")
	_, err = up.Run(TraefikReleaseName, ch, values)
	if err == nil {
		upLogger.Info(ctx, msgSym+":Upgrade/eok")
		return nil
	}
	upLogger.Info(ctx, msgSym+":Upgrade/efail", "err", err)
	if !stdErrors.Is(err, helmdriver.ErrNoDeployedReleases) {
		return fmt.Errorf("helm upgrade traefik: %w", err)
	}

	// If release doesn't exist, perform install instead
	in := action.NewInstall(cfg)
	in.Namespace = ns
	in.ReleaseName = TraefikReleaseName
	in.Atomic = true
	in.Wait = true
	in.Timeout = 5 * time.Minute
	inLogger := logger.With("ns", ns, "release", TraefikReleaseName)
	inLogger.Info(ctx, msgSym+":Install/s")
	_, err = in.Run(ch, values)
	if err == nil {
		inLogger.Info(ctx, msgSym+":Install/eok")
		return nil
	}
	inLogger.Info(ctx, msgSym+":Install/efail", "err", err)
	return fmt.Errorf("helm install traefik: %w", err)
}

// UninstallIngressTraefik removes the Traefik release. Best-effort and idempotent.
func (c *Client) UninstallIngressTraefik(ctx context.Context, cluster *model.Cluster) error {
	if c == nil || c.RESTConfig == nil {
		return fmt.Errorf("kube client is not initialized")
	}

	logger := logging.FromContext(ctx)
	msgSym := "KubeClient:InstallIngressTraefik"

	kubeBytes := c.Kubeconfig()
	if len(kubeBytes) == 0 {
		return fmt.Errorf("kubeconfig is required for Helm operations")
	}
	ns := IngressNamespace(cluster)

	kubeconfigPath, cleanup, err := tempfile(kubeBytes)
	if err != nil {
		return err
	}
	defer cleanup()

	settings := cli.New()
	settings.KubeConfig = kubeconfigPath

	cfg := new(action.Configuration)
	if err := cfg.Init(settings.RESTClientGetter(), ns, "secret", func(format string, v ...any) {}); err != nil {
		return fmt.Errorf("init helm configuration: %w", err)
	}
	un := action.NewUninstall(cfg)
	unLogger := logger.With("ns", ns, "release", TraefikReleaseName)
	unLogger.Info(ctx, msgSym+":Uninstall/s")
	if _, err := un.Run(TraefikReleaseName); err != nil {
		unLogger.Info(ctx, msgSym+":Uninstall/efail", "err", err)
		if stdErrors.Is(err, helmdriver.ErrReleaseNotFound) {
			return nil
		}
		return fmt.Errorf("helm uninstall traefik: %w", err)
	}
	unLogger.Info(ctx, msgSym+":Uninstall/eok")
	return nil
}
