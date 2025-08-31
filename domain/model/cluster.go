package model

import (
	"context"
	"time"
)

// Cluster represents a Kubernetes cluster resource.
type Cluster struct {
	ID         string
	Name       string
	ProviderID string // references Provider
	Existing   bool
	Domain     string
	Ingress    *ClusterIngress
	Settings   map[string]string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// ClusterIngress defines cluster-level ingress configuration.
type ClusterIngress struct {
	Namespace      string
	Controller     string
	ServiceAccount string
	// CertResolver selects the Traefik ACME resolver (e.g., "staging", "production").
	CertResolver string
	// CertEmail is the email address used for ACME account registration.
	CertEmail string
	// Certificates are static TLS certificates to be made available to the ingress controller.
	Certificates []ClusterIngressCertificate
}

// ClusterIngressCertificate represents a static certificate reference.
// Name is an arbitrary identifier; it determines the Kubernetes TLS Secret name as "tls-" + Name.
// Source is a provider-specific locator. For AKS, a Key Vault secret URL is supported.
type ClusterIngressCertificate struct {
	Name   string
	Source string
}

// Operation-scoped options and functional option types.
type ClusterProvisionOptions struct{ Force bool }
type ClusterDeprovisionOptions struct{ Force bool }
type ClusterInstallOptions struct{ Force bool }
type ClusterUninstallOptions struct{ Force bool }

type ClusterProvisionOption func(*ClusterProvisionOptions)
type ClusterDeprovisionOption func(*ClusterDeprovisionOptions)
type ClusterInstallOption func(*ClusterInstallOptions)
type ClusterUninstallOption func(*ClusterUninstallOptions)

// Option helpers
func WithClusterProvisionForce() ClusterProvisionOption {
	return func(o *ClusterProvisionOptions) { o.Force = true }
}
func WithClusterDeprovisionForce() ClusterDeprovisionOption {
	return func(o *ClusterDeprovisionOptions) { o.Force = true }
}
func WithClusterInstallForce() ClusterInstallOption {
	return func(o *ClusterInstallOptions) { o.Force = true }
}
func WithClusterUninstallForce() ClusterUninstallOption {
	return func(o *ClusterUninstallOptions) { o.Force = true }
}

// ClusterPort is an interface (domain port) for cluster operations.
type ClusterPort interface {
	Status(ctx context.Context, cluster *Cluster) (*ClusterStatus, error)
	Provision(ctx context.Context, cluster *Cluster, opts ...ClusterProvisionOption) error
	Deprovision(ctx context.Context, cluster *Cluster, opts ...ClusterDeprovisionOption) error
	Install(ctx context.Context, cluster *Cluster, opts ...ClusterInstallOption) error
	Uninstall(ctx context.Context, cluster *Cluster, opts ...ClusterUninstallOption) error
}

// ClusterStatus represents the status of a cluster.
type ClusterStatus struct {
	Existing        bool   `json:"existing"`                  // Value of cluster.existing configuration
	Provisioned     bool   `json:"provisioned"`               // True when the Kubernetes cluster exists
	Installed       bool   `json:"installed"`                 // True when in-cluster resources are installed
	IngressGlobalIP string `json:"ingressGlobalIP,omitempty"` // Ingress global IP address
	IngressFQDN     string `json:"ingressFQDN,omitempty"`     // Ingress FQDN (if available)
}

// Note: Status retrieval should be invoked from use cases through a ClusterPort.
