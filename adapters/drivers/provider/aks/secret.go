package aks

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/yaegashi/kompoxops/adapters/kube"
	"github.com/yaegashi/kompoxops/domain/model"
	"sigs.k8s.io/yaml"
)

// ensureSecretProviderClassFromKeyVault creates a SecretProviderClass and configures sync for kubernetes.io/tls
// Secrets for each cluster.Ingress.Certificates entry whose source is an Azure Key Vault secret URL.
// Requires AKS Workload Identity labels and CSI driver to be present (installed by provider infra).
func (d *driver) ensureSecretProviderClassFromKeyVault(ctx context.Context, kc *kube.Client, cluster *model.Cluster, tenantID, clientID string) error {
	if kc == nil {
		return fmt.Errorf("kube client is not initialized")
	}
	if cluster == nil || cluster.Ingress == nil || len(cluster.Ingress.Certificates) == 0 {
		return nil
	}

	ns := kube.IngressNamespace(cluster)
	spcName := kube.SecretProviderClassName(cluster)

	type obj struct{ objectName string }
	var (
		objects      []obj
		secretObjs   []map[string]any
		keyvaultName string
	)

	for _, cert := range cluster.Ingress.Certificates {
		if cert.Name == "" || cert.Source == "" {
			continue
		}
		u, err := url.Parse(cert.Source)
		if err != nil {
			return fmt.Errorf("invalid certificate source for %s: %w", cert.Name, err)
		}
		if !strings.HasSuffix(u.Host, ".vault.azure.net") {
			return fmt.Errorf("unsupported certificate source host for %s: %s", cert.Name, u.Host)
		}
		if keyvaultName == "" {
			keyvaultName = strings.TrimSuffix(u.Host, ".vault.azure.net")
		}
		elems := strings.Split(strings.Trim(path.Clean(u.Path), "/"), "/")
		if len(elems) < 2 || elems[0] != "secrets" {
			return fmt.Errorf("unsupported key vault path for %s: %s", cert.Name, u.Path)
		}
		objectName := elems[1]
		objects = append(objects, obj{objectName: objectName})
		secretObjs = append(secretObjs, map[string]any{
			"secretName": kube.IngressTLSSecretName(cert.Name),
			"type":       "kubernetes.io/tls",
			"data": []map[string]any{
				// When contentType is application/x-pem-file, provider emits <name>.key and <name>.crt
				{"objectName": objectName + ".key", "key": "tls.key"},
				{"objectName": objectName + ".crt", "key": "tls.crt"},
			},
		})
	}
	if keyvaultName == "" || len(objects) == 0 {
		return nil
	}

	// Build objects: array: - | blocks (provider expects YAML literal without an extra leading '|')
	var b strings.Builder
	b.WriteString("array:\n")
	for _, o := range objects {
		b.WriteString("  - |\n")
		b.WriteString("    objectName: ")
		b.WriteString(o.objectName)
		b.WriteString("\n    objectType: secret\n")
		// Ensure provider splits into .key/.crt when syncing certificate secrets
		b.WriteString("    contentType: application/x-pem-file\n")
	}

	spc := map[string]any{
		"apiVersion": "secrets-store.csi.x-k8s.io/v1",
		"kind":       "SecretProviderClass",
		"metadata": map[string]any{
			"name":      spcName,
			"namespace": ns,
		},
		"spec": map[string]any{
			"provider":      "azure",
			"secretObjects": secretObjs,
			"parameters": map[string]any{
				// Use AKS Workload Identity: do not use VM Managed Identity, specify clientID instead
				"usePodIdentity":     "false",
				"useManagedIdentity": "false",
				"clientID":           clientID,
				"keyvaultName":       keyvaultName,
				"tenantId":           tenantID,
				"objects":            b.String(),
			},
		},
	}

	raw, err := yaml.Marshal(spc)
	if err != nil {
		return fmt.Errorf("marshal SecretProviderClass: %w", err)
	}
	if err := kc.ApplyYAML(ctx, raw, &kube.ApplyOptions{DefaultNamespace: ns}); err != nil {
		return fmt.Errorf("apply SecretProviderClass: %w", err)
	}

	return nil
}
