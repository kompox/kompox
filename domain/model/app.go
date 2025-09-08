package model

import "time"

// App represents an application deployed to a cluster.
type App struct {
	ID         string
	Name       string
	ClusterID  string // references Cluster
	Compose    string
	Ingress    AppIngress
	Volumes    []AppVolume
	Deployment AppDeployment
	Resources  map[string]string
	Settings   map[string]string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// AppIngress defines ingress-wide settings and rules for an app.
type AppIngress struct {
	// CertResolver overrides cluster-level resolver when set.
	CertResolver string
	Rules        []AppIngressRule
}

// AppIngressRule defines external exposure of a host/port.
type AppIngressRule struct {
	Name  string
	Port  int
	Hosts []string
}

// AppVolume defines a persistent volume requested by the app.
type AppVolume struct {
	Name    string
	Size    int64          // in bytes (parsed from user configuration quantities like "32Gi").
	Options map[string]any // provider-specific options for volume configuration (e.g., SKU, IOPS, throughput).
}

// AppDeployment defines deployment configuration for the app.
type AppDeployment struct {
	// Pool specifies the node pool for deployment.
	// Defaults to "user" if unspecified.
	Pool string
	// Zone specifies the availability zone for deployment.
	// Only sets nodeSelector when specified.
	Zone string
}
