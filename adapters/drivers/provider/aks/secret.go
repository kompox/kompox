package aks

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/yaegashi/kompoxops/adapters/kube"
	"github.com/yaegashi/kompoxops/domain/model"
	"sigs.k8s.io/yaml"
)

// Base mount path for TLS materials consumed by Traefik file provider
const baseTLSMountPath = "/config/tls"

// parseKeyVaultSecretURL parses an Azure Key Vault secret URL and returns (keyvaultName, objectName).
// Supports URLs of the form: https://<vault>.vault.azure.net/secrets/<name>[/<version>]
func (d *driver) parseKeyVaultSecretURL(raw string) (string, string, error) {
	if raw == "" {
		return "", "", fmt.Errorf("empty key vault url")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", fmt.Errorf("invalid key vault url: %w", err)
	}
	if !strings.HasSuffix(u.Host, ".vault.azure.net") {
		return "", "", fmt.Errorf("unsupported host: %s", u.Host)
	}
	kvName := strings.TrimSuffix(u.Host, ".vault.azure.net")
	elems := strings.Split(strings.Trim(path.Clean(u.Path), "/"), "/")
	if len(elems) < 2 || elems[0] != "secrets" {
		return "", "", fmt.Errorf("unsupported path: %s", u.Path)
	}
	objectName := elems[1]
	return kvName, objectName, nil
}

// spcNameForVault returns the SecretProviderClass name for a given Key Vault.
// When multiple vaults are present, the name will be suffixed with the sanitized vault name.
func (d *driver) spcNameForVault(vault string) string {
	base := kube.TraefikReleaseName + "-kv"
	lower := strings.ToLower(vault)
	re := regexp.MustCompile("[^a-z0-9-]")
	return fmt.Sprintf("%s-%s", base, re.ReplaceAllString(lower, "-"))
}

// ensureSecretProviderClassFromKeyVault creates a SecretProviderClass and configures sync for kubernetes.io/tls
// Secrets for each cluster.Ingress.Certificates entry whose source is an Azure Key Vault secret URL.
// Requires AKS Workload Identity labels and CSI driver to be present (installed by provider infra).
// ensureSecretProviderClassFromKeyVault creates SPC resources per Key Vault and returns:
// - mounts: list of {name, mountPath} to be mounted into Pods
// - certs: list of {certFile, keyFile} entries suitable for Traefik file provider (certs.yaml)
func (d *driver) ensureSecretProviderClassFromKeyVault(ctx context.Context, kc *kube.Client, cluster *model.Cluster, tenantID, clientID string) (mounts []map[string]string, certs []map[string]any, err error) {
	if kc == nil {
		return nil, nil, fmt.Errorf("kube client is not initialized")
	}
	if cluster == nil || cluster.Ingress == nil || len(cluster.Ingress.Certificates) == 0 {
		return nil, nil, nil
	}

	ns := kube.IngressNamespace(cluster)
	// Group certificates by Key Vault name
	type obj struct {
		objectName  string
		objectAlias string
	}
	type group struct {
		keyvaultName string
		objects      []obj
		secretObjs   []map[string]any
	}
	groups := map[string]*group{}

	for _, cert := range cluster.Ingress.Certificates {
		if cert.Name == "" || cert.Source == "" {
			continue
		}
		kvName, objectName, err := d.parseKeyVaultSecretURL(cert.Source)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid certificate source for %s: %w", cert.Name, err)
		}
		g := groups[kvName]
		if g == nil {
			g = &group{keyvaultName: kvName}
			groups[kvName] = g
		}
		g.objects = append(g.objects, obj{objectName: objectName, objectAlias: cert.Name})
		g.secretObjs = append(g.secretObjs, map[string]any{
			"secretName": kube.IngressTLSSecretName(cert.Name),
			"type":       "kubernetes.io/tls",
			"data": []map[string]any{
				// When contentType is application/x-pem-file, provider emits <name>.key and <name>.crt
				{"objectName": objectName + ".key", "key": "tls.key"},
				{"objectName": objectName + ".crt", "key": "tls.crt"},
			},
		})
	}

	if len(groups) == 0 {
		return nil, nil, nil
	}

	// deterministic iteration for idempotency
	kvNames := make([]string, 0, len(groups))
	for name := range groups {
		kvNames = append(kvNames, name)
	}
	sort.Strings(kvNames)

	for _, kvName := range kvNames {
		g := groups[kvName]
		// Build objects literal expected by provider
		var b strings.Builder
		b.WriteString("array:\n")
		for _, o := range g.objects {
			b.WriteString("  - |\n")
			b.WriteString("    objectName: ")
			b.WriteString(o.objectName)
			b.WriteString("\n    objectType: secret\n")
			b.WriteString("    contentType: application/x-pem-file\n")
			if o.objectAlias != "" {
				b.WriteString("    objectAlias: ")
				b.WriteString(o.objectAlias)
				b.WriteString("\n")
			}
		}

		spcName := d.spcNameForVault(kvName)

		spc := map[string]any{
			"apiVersion": "secrets-store.csi.x-k8s.io/v1",
			"kind":       "SecretProviderClass",
			"metadata": map[string]any{
				"name":      spcName,
				"namespace": ns,
			},
			"spec": map[string]any{
				"provider":      "azure",
				"secretObjects": g.secretObjs,
				"parameters": map[string]any{
					// Use AKS Workload Identity: do not use VM Managed Identity, specify clientID instead
					"usePodIdentity":     "false",
					"useManagedIdentity": "false",
					"clientID":           clientID,
					"keyvaultName":       g.keyvaultName,
					"tenantId":           tenantID,
					"objects":            b.String(),
				},
			},
		}

		raw, mErr := yaml.Marshal(spc)
		if mErr != nil {
			return nil, nil, fmt.Errorf("marshal SecretProviderClass: %w", mErr)
		}
		if aErr := kc.ApplyYAML(ctx, raw, &kube.ApplyOptions{DefaultNamespace: ns}); aErr != nil {
			return nil, nil, fmt.Errorf("apply SecretProviderClass: %w", aErr)
		}

		// Derive mount path subdir from SPC name suffix and list files for certs.yaml
		base := kube.TraefikReleaseName + "-kv-"
		dir := strings.TrimPrefix(spcName, base)
		if dir == spcName { // fallback when no suffix
			dir = strings.ToLower(kvName)
		}
		mountPath := path.Join(baseTLSMountPath, dir)
		mounts = append(mounts, map[string]string{"name": spcName, "mountPath": mountPath})
		for _, o := range g.objects {
			if o.objectAlias == "" {
				continue
			}
			certs = append(certs, map[string]any{
				"certFile": path.Join(mountPath, o.objectAlias+".crt"),
				"keyFile":  path.Join(mountPath, o.objectAlias+".key"),
			})
		}
	}

	return mounts, certs, nil
}
