package model

import "context"

// Operation-scoped options and functional option types.
type ClusterProvisionOptions struct{ Force bool }
type ClusterDeprovisionOptions struct{ Force bool }
type ClusterInstallOptions struct{ Force bool }
type ClusterUninstallOptions struct{ Force bool }
type ClusterDNSApplyOptions struct {
	ZoneHint string
	Strict   bool
	DryRun   bool
}

type ClusterProvisionOption func(*ClusterProvisionOptions)
type ClusterDeprovisionOption func(*ClusterDeprovisionOptions)
type ClusterInstallOption func(*ClusterInstallOptions)
type ClusterUninstallOption func(*ClusterUninstallOptions)
type ClusterDNSApplyOption func(*ClusterDNSApplyOptions)

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

// WithClusterDNSApplyZoneHint hints the target zone for ClusterDNSApply.
func WithClusterDNSApplyZoneHint(zone string) ClusterDNSApplyOption {
	return func(o *ClusterDNSApplyOptions) { o.ZoneHint = zone }
}

// WithClusterDNSApplyStrict surfaces provider write failures as errors.
func WithClusterDNSApplyStrict() ClusterDNSApplyOption {
	return func(o *ClusterDNSApplyOptions) { o.Strict = true }
}

// WithClusterDNSApplyDryRun skips provider mutations.
func WithClusterDNSApplyDryRun() ClusterDNSApplyOption {
	return func(o *ClusterDNSApplyOptions) { o.DryRun = true }
}

// ClusterPort is an interface (domain port) for cluster operations.
type ClusterPort interface {
	Status(ctx context.Context, cluster *Cluster) (*ClusterStatus, error)
	Provision(ctx context.Context, cluster *Cluster, opts ...ClusterProvisionOption) error
	Deprovision(ctx context.Context, cluster *Cluster, opts ...ClusterDeprovisionOption) error
	Install(ctx context.Context, cluster *Cluster, opts ...ClusterInstallOption) error
	Uninstall(ctx context.Context, cluster *Cluster, opts ...ClusterUninstallOption) error
	DNSApply(ctx context.Context, cluster *Cluster, rset DNSRecordSet, opts ...ClusterDNSApplyOption) error
}

// ClusterStatus represents the status of a cluster.
type ClusterStatus struct {
	Existing        bool   `json:"existing"`                  // Value of cluster.existing configuration
	Provisioned     bool   `json:"provisioned"`               // True when the Kubernetes cluster exists
	Installed       bool   `json:"installed"`                 // True when in-cluster resources are installed
	IngressGlobalIP string `json:"ingressGlobalIP,omitempty"` // Ingress global IP address
	IngressFQDN     string `json:"ingressFQDN,omitempty"`     // Ingress FQDN (if available)
}
