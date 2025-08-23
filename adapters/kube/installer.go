package kube

import (
	"context"
	stdErrors "errors"
	"fmt"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/yaegashi/kompoxops/domain/model"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	helmdriver "helm.sh/helm/v3/pkg/storage/driver"
)

// Installer provides common in-cluster install/uninstall operations.
// It is intended to be called from provider drivers' ClusterInstall/ClusterUninstall.
type Installer struct {
	Client *Client
	// Kubeconfig holds raw kubeconfig bytes used for Helm operations.
	// When empty, Helm-based operations will fail with an error.
	Kubeconfig []byte
}

// NewInstaller constructs an Installer from a kube Client.
func NewInstaller(c *Client) *Installer {
	return &Installer{Client: c}
}

// NewInstallerWithKubeconfig constructs an Installer with kube client and kubeconfig bytes.
func NewInstallerWithKubeconfig(c *Client, kubeconfig []byte) *Installer {
	return &Installer{Client: c, Kubeconfig: kubeconfig}
}

// writeTempKubeconfig writes kubeconfig bytes to a temporary file and returns its path
// and a cleanup function to remove it.
func writeTempKubeconfig(kubeconfig []byte) (string, func(), error) {
	f, err := os.CreateTemp("", "kompoxops-kubeconfig-*.yaml")
	if err != nil {
		return "", func() {}, fmt.Errorf("create temp kubeconfig: %w", err)
	}
	path := f.Name()
	if _, err := f.Write(kubeconfig); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return "", func() {}, fmt.Errorf("write temp kubeconfig: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return "", func() {}, fmt.Errorf("close temp kubeconfig: %w", err)
	}
	cleanup := func() { _ = os.Remove(path) }
	return path, cleanup, nil
}

// IngressNamespace resolves the namespace to use for ingress from the cluster spec.
// Falls back to "default" when not specified.
func IngressNamespace(cluster *model.Cluster) string {
	ns := "default"
	if cluster != nil && cluster.Ingress != nil {
		if v := cluster.Ingress.Namespace; v != "" {
			ns = v
		}
	}
	return ns
}

// IngressServiceAccountName returns the canonical ServiceAccount name used by ingress workloads.
func IngressServiceAccountName(cluster *model.Cluster) string {
	// Default name when not specified
	const def = "ingress-service-account"
	if cluster != nil && cluster.Ingress != nil {
		if sa := cluster.Ingress.ServiceAccount; sa != "" {
			return sa
		}
	}
	return def
}

// EnsureIngressNamespace ensures the ingress namespace exists.
func (i *Installer) EnsureIngressNamespace(ctx context.Context, cluster *model.Cluster) error {
	ns := IngressNamespace(cluster)
	return i.EnsureNamespace(ctx, ns)
}

// DeleteIngressNamespace deletes the ingress namespace if it exists.
func (i *Installer) DeleteIngressNamespace(ctx context.Context, cluster *model.Cluster) error {
	ns := IngressNamespace(cluster)
	return i.DeleteNamespace(ctx, ns)
}

// InstallTraefik installs or upgrades a minimal Traefik ingress controller into the ingress namespace.
// This implementation uses server-side apply on in-repo generated manifests to avoid external dependencies.
// It creates the following resources:
// - ServiceAccount: traefik (namespaced)
// - ClusterRole/ClusterRoleBinding: traefik (cluster-scoped)
// - Deployment: traefik (namespaced)
// - Service: traefik (LoadBalancer, namespaced)
func (i *Installer) InstallTraefik(ctx context.Context, cluster *model.Cluster) error {
	if i == nil || i.Client == nil || i.Client.RESTConfig == nil {
		return fmt.Errorf("kube installer is not initialized")
	}
	if len(i.Kubeconfig) == 0 {
		return fmt.Errorf("kubeconfig is required for Helm operations")
	}
	ns := IngressNamespace(cluster)
	if err := i.EnsureNamespace(ctx, ns); err != nil {
		return err
	}

	// Prepare a temporary kubeconfig file for Helm SDK
	kubeconfigPath, cleanup, err := writeTempKubeconfig(i.Kubeconfig)
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

	// Try upgrade first; if the release doesn't exist, fallback to install (CLI-compatible behavior)
	up := action.NewUpgrade(cfg)
	up.Namespace = ns
	up.Atomic = true
	up.Wait = true
	up.Timeout = 5 * time.Minute

	// Locate and load the Traefik chart from official repo
	cpo := action.ChartPathOptions{RepoURL: "https://helm.traefik.io/traefik"}
	chartPath, err := cpo.LocateChart("traefik", settings)
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
		// Fall back to a placeholder if not provided; user should set a real email in cluster config.
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

	if _, err := up.Run("traefik", ch, values); err != nil {
		// If release doesn't exist, perform install instead
		if stdErrors.Is(err, helmdriver.ErrNoDeployedReleases) {
			in := action.NewInstall(cfg)
			in.Namespace = ns
			in.ReleaseName = "traefik"
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

// UninstallTraefik removes the Traefik resources created by InstallTraefik.
// It's best-effort and idempotent.
func (i *Installer) UninstallTraefik(ctx context.Context, cluster *model.Cluster) error {
	if i == nil || i.Client == nil || i.Client.RESTConfig == nil {
		return fmt.Errorf("kube installer is not initialized")
	}
	if len(i.Kubeconfig) == 0 {
		return fmt.Errorf("kubeconfig is required for Helm operations")
	}
	ns := IngressNamespace(cluster)

	kubeconfigPath, cleanup, err := writeTempKubeconfig(i.Kubeconfig)
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
	if _, err := un.Run("traefik"); err != nil {
		// When the release doesn't exist, treat as success
		if stdErrors.Is(err, helmdriver.ErrReleaseNotFound) {
			return nil
		}
		return fmt.Errorf("helm uninstall traefik: %w", err)
	}
	return nil
}

// waitForDeploymentReady polls the deployment until at least one replica is ready or context timeout.
func (i *Installer) waitForDeploymentReady(ctx context.Context, namespace, name string) error {
	if i == nil || i.Client == nil || i.Client.Clientset == nil {
		return fmt.Errorf("kube installer is not initialized")
	}
	// Lightweight poll loop without extra dependencies
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	timeout := time.NewTimer(5 * time.Minute)
	defer timeout.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout.C:
			return fmt.Errorf("timeout waiting for deployment %s/%s ready", namespace, name)
		case <-ticker.C:
			dep, err := i.Client.Clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					continue
				}
				// transient error, keep polling
				continue
			}
			if dep.Status.ReadyReplicas >= 1 {
				return nil
			}
		}
	}
}

// EnsureNamespace creates a namespace if it does not exist (idempotent).
func (i *Installer) EnsureNamespace(ctx context.Context, name string) error {
	if i == nil || i.Client == nil || i.Client.Clientset == nil {
		return fmt.Errorf("kube installer is not initialized")
	}
	if name == "" {
		return fmt.Errorf("namespace name is empty")
	}

	_, err := i.Client.Clientset.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("get namespace %s: %w", name, err)
	}

	_, err = i.Client.Clientset.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		return fmt.Errorf("create namespace %s: %w", name, err)
	}
	return nil
}

// DeleteNamespace deletes a namespace if it exists (idempotent best-effort).
func (i *Installer) DeleteNamespace(ctx context.Context, name string) error {
	if i == nil || i.Client == nil || i.Client.Clientset == nil {
		return fmt.Errorf("kube installer is not initialized")
	}
	if name == "" {
		return fmt.Errorf("namespace name is empty")
	}

	err := i.Client.Clientset.CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("delete namespace %s: %w", name, err)
	}
	return nil
}
