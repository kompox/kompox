package model

import (
	"fmt"
	"time"
)

// App represents an application deployed to a cluster.
type App struct {
	ID            string
	Name          string
	ClusterID     string // references Cluster
	Compose       string
	Ingress       AppIngress
	Volumes       []AppVolume
	Deployment    AppDeployment
	NetworkPolicy AppNetworkPolicy
	Resources     map[string]string
	Settings      map[string]string
	CreatedAt     time.Time
	UpdatedAt     time.Time

	// RefBase defines the base reference for resolving relative file/URL paths in Compose.
	// - "" (empty): external references are prohibited (external KOM origin)
	// - "file:///abs/dir/": local references allowed, relative paths resolved from this directory (Kompox app file origin)
	// - "http(s)://.../": relative URLs allowed, local references prohibited (URL origin)
	// Validation occurs at kompoxops app command execution time, not at KOM load time.
	// See: K4x-ADR-012
	RefBase string
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
	Type    string         // volume type: "disk" (default, RWO block storage) or "files" (RWX network file shares). Empty means "disk".
	Options map[string]any // provider-specific options for volume configuration (e.g., SKU, IOPS, throughput).
}

// AppDeployment defines deployment configuration for the app.
type AppDeployment struct {
	// Pool specifies the node pool for deployment.
	// Defaults to "user" if unspecified.
	Pool string
	// Pools specifies multiple node pools for deployment.
	Pools []string
	// Zone specifies the availability zone for deployment.
	// Only sets nodeSelector when specified.
	Zone string
	// Zones specifies multiple availability zones for deployment.
	Zones []string
	// Selectors is reserved for future scheduling extensions.
	Selectors map[string]string
}

// AppNetworkPolicy defines network policy configuration for the app.
type AppNetworkPolicy struct {
	IngressRules []AppNetworkPolicyIngressRule
}

// AppNetworkPolicyIngressRule defines an ingress rule.
type AppNetworkPolicyIngressRule struct {
	From  []AppNetworkPolicyPeer
	Ports []AppNetworkPolicyPort
}

// AppNetworkPolicyPeer defines a network peer selector.
type AppNetworkPolicyPeer struct {
	NamespaceSelector *LabelSelector
}

// LabelSelector represents a label selector for matching resources.
type LabelSelector struct {
	MatchLabels      map[string]string
	MatchExpressions []LabelSelectorRequirement
}

// LabelSelectorRequirement represents a label selector requirement.
type LabelSelectorRequirement struct {
	Key      string
	Operator string // In, NotIn, Exists, DoesNotExist
	Values   []string
}

// AppNetworkPolicyPort defines a port and protocol.
type AppNetworkPolicyPort struct {
	Protocol string // TCP, UDP, or SCTP (defaults to TCP if empty)
	Port     int
}

// FindVolume returns the AppVolume for the given volume name.
// Returns an error if the volume is not found.
func (app *App) FindVolume(volName string) (*AppVolume, error) {
	for i, v := range app.Volumes {
		if v.Name == volName {
			return &app.Volumes[i], nil
		}
	}
	return nil, fmt.Errorf("volume not found: %s", volName)
}
