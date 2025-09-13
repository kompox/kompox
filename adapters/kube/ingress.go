package kube

import (
	"github.com/kompox/kompox/domain/model"
)

// Default values for ingress-related resources.
const (
	// DefaultIngressNamespace is the default namespace where the ingress controller is installed
	// when not explicitly specified by the cluster spec.
	DefaultIngressNamespace = "traefik"

	// TraefikReleaseName is the default Helm release name for Traefik and is also used as
	// the default Service/Deployment name created by the chart.
	TraefikReleaseName = "traefik"

	// DefaultIngressServiceAccount is the default ServiceAccount name used by ingress workloads
	// when the cluster spec does not specify a ServiceAccount.
	DefaultIngressServiceAccount = "ingress-service-account"
)

// IngressNamespace resolves the namespace to use for ingress from the cluster spec.
// Falls back to IngressDefaultNamespace when not specified.
func IngressNamespace(cluster *model.Cluster) string {
	if cluster != nil && cluster.Ingress != nil {
		if v := cluster.Ingress.Namespace; v != "" {
			return v
		}
	}
	return DefaultIngressNamespace
}

// IngressServiceName returns the Service name used by the ingress controller.
// For now this maps directly to the Traefik Helm release name.
func IngressServiceName(cluster *model.Cluster) string {
	return TraefikReleaseName
}

// IngressServiceAccountName returns the canonical ServiceAccount name used by ingress workloads.
func IngressServiceAccountName(cluster *model.Cluster) string {
	if cluster != nil && cluster.Ingress != nil {
		if sa := cluster.Ingress.ServiceAccount; sa != "" {
			return sa
		}
	}
	return DefaultIngressServiceAccount
}

// IngressTLSSecretName returns the Kubernetes TLS Secret name for a static certificate entry.
// It is derived as "tls-" + certificate.Name.
func IngressTLSSecretName(certName string) string {
	if certName == "" {
		return ""
	}
	return "tls-" + certName
}
