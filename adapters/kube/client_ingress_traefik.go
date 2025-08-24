package kube

import (
	"context"
	stdErrors "errors"
	"fmt"
	"time"

	"github.com/yaegashi/kompoxops/domain/model"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	helmdriver "helm.sh/helm/v3/pkg/storage/driver"
)

// InstallIngressTraefik installs or upgrades a minimal Traefik ingress controller into the ingress namespace.
// This uses the Helm SDK with a temporary kubeconfig file derived from this client.
func (c *Client) InstallIngressTraefik(ctx context.Context, cluster *model.Cluster) error {
	if c == nil || c.RESTConfig == nil {
		return fmt.Errorf("kube client is not initialized")
	}
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

	// Try upgrade first; if the release doesn't exist, fallback to install
	up := action.NewUpgrade(cfg)
	up.Namespace = ns
	up.Atomic = true
	up.Wait = true
	up.Timeout = 5 * time.Minute

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
	values := map[string]any{
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
		// Will populate below
		"additionalArguments": []string{},
		// Ensure mounted PVC at /data is group-writable for the Traefik user (65532)
		"podSecurityContext": map[string]any{
			"fsGroup":             65532,
			"fsGroupChangePolicy": "OnRootMismatch",
		},
	}

	// Configure Let's Encrypt (ACME) resolvers for staging and production
	certEmail := ""
	preferredResolver := ""
	if cluster != nil && cluster.Ingress != nil {
		certEmail = cluster.Ingress.CertEmail
		preferredResolver = cluster.Ingress.CertResolver
	}
	if certEmail == "" {
		// Fallback placeholder; users should configure a real email in cluster config
		certEmail = "noreply@example.com"
	}
	addArgs := []string{
		"--certificatesresolvers.production.acme.tlschallenge=true",
		"--certificatesresolvers.production.acme.caserver=https://acme-v02.api.letsencrypt.org/directory",
		fmt.Sprintf("--certificatesresolvers.production.acme.email=%s", certEmail),
		"--certificatesresolvers.production.acme.storage=/data/acme-production.json",
		"--certificatesresolvers.staging.acme.tlschallenge=true",
		"--certificatesresolvers.staging.acme.caserver=https://acme-staging-v02.api.letsencrypt.org/directory",
		fmt.Sprintf("--certificatesresolvers.staging.acme.email=%s", certEmail),
		"--certificatesresolvers.staging.acme.storage=/data/acme-staging.json",
	}
	if preferredResolver == "production" || preferredResolver == "staging" {
		addArgs = append(addArgs, fmt.Sprintf("--entrypoints.websecure.http.tls.certresolver=%s", preferredResolver))
	}
	values["additionalArguments"] = addArgs

	if _, err := up.Run(TraefikReleaseName, ch, values); err != nil {
		// If release doesn't exist, perform install instead
		if stdErrors.Is(err, helmdriver.ErrNoDeployedReleases) {
			in := action.NewInstall(cfg)
			in.Namespace = ns
			in.ReleaseName = TraefikReleaseName
			in.Atomic = true
			in.Wait = true
			in.Timeout = 5 * time.Minute
			if _, ierr := in.Run(ch, values); ierr != nil {
				return fmt.Errorf("helm install traefik: %w", ierr)
			}
			return nil
		}
		return fmt.Errorf("helm upgrade traefik: %w", err)
	}
	return nil
}

// UninstallIngressTraefik removes the Traefik release. Best-effort and idempotent.
func (c *Client) UninstallIngressTraefik(ctx context.Context, cluster *model.Cluster) error {
	if c == nil || c.RESTConfig == nil {
		return fmt.Errorf("kube client is not initialized")
	}
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
	if _, err := un.Run(TraefikReleaseName); err != nil {
		if stdErrors.Is(err, helmdriver.ErrReleaseNotFound) {
			return nil
		}
		return fmt.Errorf("helm uninstall traefik: %w", err)
	}
	return nil
}
