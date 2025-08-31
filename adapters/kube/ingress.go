package kube

import "github.com/yaegashi/kompoxops/domain/model"

// TraefikReleaseName is the Helm release name for Traefik and also used as the default Service/Deployment name.
const TraefikReleaseName = "traefik"

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

// IngressServiceName returns the Service name used by the ingress controller.
// For now this maps directly to the Traefik Helm release name.
func IngressServiceName(cluster *model.Cluster) string {
	return TraefikReleaseName
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

// IngressTLSSecretName returns the Kubernetes TLS Secret name for a static certificate entry.
// It is derived as "tls-" + certificate.Name.
func IngressTLSSecretName(certName string) string {
	if certName == "" {
		return ""
	}
	return "tls-" + certName
}

// SecretProviderClassName returns a stable name for the SecretProviderClass used by ingress.
func SecretProviderClassName(cluster *model.Cluster) string {
	// One SPC per cluster ingress namespace is sufficient; name it after the release.
	return TraefikReleaseName + "-kv"
}
