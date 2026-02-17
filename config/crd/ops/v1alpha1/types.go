package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// Group is the API group for Kompox CRDs.
	Group = "ops.kompox.dev"
	// Version is the API version for Kompox CRDs.
	Version = "v1alpha1"

	// AnnotationID is the annotation key for the canonical Resource ID.
	// Format: / <kind> / <name> ( / <kind> / <name> )*
	// Example: /ws/ws1/prv/prv1/cls/cls1/app/app1/box/api
	// This is required for all Kinds including Workspace.
	AnnotationID = Group + "/id"
	// AnnotationDocPath is the annotation key for storing the source document file path.
	// This is automatically set by the Loader to enable file-relative path resolution.
	AnnotationDocPath = Group + "/doc-path"
	// AnnotationDocIndex is the annotation key for storing the document index within the source file.
	// This is automatically set by the Loader (1-based position in multi-document YAML files).
	AnnotationDocIndex = Group + "/doc-index"
)

// Workspace represents a logical workspace.
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced
type Workspace struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`

	Spec   WorkspaceSpec   `json:"spec,omitzero"`
	Status WorkspaceStatus `json:"status,omitzero"`
}

// WorkspaceSpec defines the desired state of Workspace.
type WorkspaceSpec struct {
	// Settings stores workspace-level configuration.
	Settings map[string]string `json:"settings,omitzero"`
}

// WorkspaceStatus defines the observed state of Workspace.
type WorkspaceStatus struct {
	// OpsNamespace is the Kubernetes namespace for this workspace's operational resources.
	OpsNamespace string `json:"opsNamespace,omitzero"`
}

// Defaults declares default configuration for loading KOM and resolving the default App.
// This is a non-persistent, loader-only resource type.
// At most one Defaults document is allowed per kompoxapp.yml.
// +kubebuilder:object:root=true
type Defaults struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`

	Spec DefaultsSpec `json:"spec,omitzero"`
}

// DefaultsSpec defines the desired state of Defaults.
type DefaultsSpec struct {
	// KOMPath specifies local file/directory paths to load KOM documents from.
	// Paths must be local only (no URLs). Wildcards are not supported.
	// Directories are recursively searched for YAML files.
	// Relative paths are resolved relative to the directory containing kompoxapp.yml.
	KOMPath []string `json:"komPath,omitzero"`

	// AppID specifies the default App resource ID (FQN) to use when --app-id is not provided.
	AppID string `json:"appId,omitzero"`
}

// Provider represents an infrastructure provider (e.g., AKS, k3s).
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced
type Provider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`

	Spec   ProviderSpec   `json:"spec,omitzero"`
	Status ProviderStatus `json:"status,omitzero"`
}

// ProviderSpec defines the desired state of Provider.
type ProviderSpec struct {
	// Driver specifies the provider implementation (e.g., "aks", "k3s").
	Driver string `json:"driver,omitzero"`
	// Settings stores provider-level configuration.
	Settings map[string]string `json:"settings,omitzero"`
}

// ProviderStatus defines the observed state of Provider.
type ProviderStatus struct {
	// OpsNamespace is the Kubernetes namespace for this provider's operational resources.
	OpsNamespace string `json:"opsNamespace,omitzero"`
}

// Cluster represents a Kubernetes cluster resource.
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`

	Spec   ClusterSpec   `json:"spec,omitzero"`
	Status ClusterStatus `json:"status,omitzero"`
}

// ClusterSpec defines the desired state of Cluster.
type ClusterSpec struct {
	// Existing indicates if this is an existing cluster (not managed by Kompox).
	Existing bool `json:"existing,omitzero"`
	// Ingress defines cluster-level ingress configuration.
	Ingress *ClusterIngressSpec `json:"ingress,omitzero"`
	// Protection defines lifecycle operation guards.
	Protection *ClusterProtectionSpec `json:"protection,omitzero"`
	// Settings stores cluster-level configuration.
	Settings map[string]string `json:"settings,omitzero"`
}

// ClusterProtectionSpec defines resource protection policies for cluster operations.
type ClusterProtectionSpec struct {
	// Provisioning controls cloud/infrastructure lifecycle operations (provision/deprovision).
	// Allowed values: "none" (default), "cannotDelete", "readOnly".
	// +kubebuilder:validation:Enum=none;cannotDelete;readOnly
	// +kubebuilder:default=none
	Provisioning string `json:"provisioning,omitzero"`
	// Installation controls in-cluster lifecycle operations (install/uninstall/updates).
	// Allowed values: "none" (default), "cannotDelete", "readOnly".
	// +kubebuilder:validation:Enum=none;cannotDelete;readOnly
	// +kubebuilder:default=none
	Installation string `json:"installation,omitzero"`
}

// ClusterIngressSpec defines cluster-level ingress configuration.
type ClusterIngressSpec struct {
	// Namespace is the namespace where the ingress controller runs.
	Namespace string `json:"namespace,omitzero"`
	// Controller specifies the ingress controller type.
	Controller string `json:"controller,omitzero"`
	// ServiceAccount is the service account for the ingress controller.
	ServiceAccount string `json:"serviceAccount,omitzero"`
	// Domain is the default DNS domain for generating app ingress hosts.
	Domain string `json:"domain,omitzero"`
	// CertResolver selects the ACME resolver (e.g., "staging", "production").
	CertResolver string `json:"certResolver,omitzero"`
	// CertEmail is the email address for ACME account registration.
	CertEmail string `json:"certEmail,omitzero"`
	// Certificates are static TLS certificates for the ingress controller.
	Certificates []ClusterIngressCertificate `json:"certificates,omitzero"`
}

// ClusterIngressCertificate represents a static certificate reference.
type ClusterIngressCertificate struct {
	// Name is an arbitrary identifier; determines the Kubernetes TLS Secret name.
	Name string `json:"name"`
	// Source is a provider-specific locator (e.g., Key Vault secret URL for AKS).
	Source string `json:"source"`
}

// ClusterStatus defines the observed state of Cluster.
type ClusterStatus struct {
	// OpsNamespace is the Kubernetes namespace for this cluster's operational resources.
	OpsNamespace string `json:"opsNamespace,omitzero"`
}

// App represents an application deployed to a cluster.
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced
type App struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`

	Spec   AppSpec   `json:"spec,omitzero"`
	Status AppStatus `json:"status,omitzero"`
}

// AppSpec defines the desired state of App.
type AppSpec struct {
	// Compose is the Docker Compose content for the application.
	Compose string `json:"compose,omitzero"`
	// Ingress defines ingress-wide settings and rules for the app.
	Ingress *AppIngressSpec `json:"ingress,omitzero"`
	// Volumes are persistent volumes requested by the app.
	Volumes []AppVolumeSpec `json:"volumes,omitzero"`
	// Deployment defines deployment configuration for the app.
	Deployment *AppDeploymentSpec `json:"deployment,omitzero"`
	// NetworkPolicy defines network policy configuration for the app.
	NetworkPolicy *AppNetworkPolicySpec `json:"networkPolicy,omitzero"`
	// Resources stores resource-related configuration.
	Resources map[string]string `json:"resources,omitzero"`
	// Settings stores app-level configuration.
	Settings map[string]string `json:"settings,omitzero"`
}

// AppIngressSpec defines ingress-wide settings and rules for an app.
type AppIngressSpec struct {
	// CertResolver overrides cluster-level resolver when set.
	CertResolver string `json:"certResolver,omitzero"`
	// Rules define external exposure of host/port.
	Rules []AppIngressRule `json:"rules,omitzero"`
}

// AppIngressRule defines external exposure of a host/port.
type AppIngressRule struct {
	// Name is an identifier for this rule.
	Name string `json:"name"`
	// Port is the service port to expose.
	Port int `json:"port"`
	// Hosts are the hostnames for this rule.
	Hosts []string `json:"hosts,omitzero"`
}

// AppVolumeSpec defines a persistent volume requested by the app.
type AppVolumeSpec struct {
	// Name is the volume identifier.
	Name string `json:"name"`
	// Size is the volume size (int64 for bytes, or string like "10Gi").
	Size any `json:"size,omitzero"`
	// Type is the volume type: "disk" (default, RWO) or "files" (RWX).
	// Empty means "disk".
	// +kubebuilder:validation:Enum=disk;files;""
	// +kubebuilder:default=""
	Type string `json:"type,omitzero"`
	// Options are provider-specific volume options (e.g., SKU, IOPS).
	Options map[string]any `json:"options,omitzero"`
}

// AppDeploymentSpec defines deployment configuration for the app.
type AppDeploymentSpec struct {
	// Pool specifies the node pool for deployment (defaults to "user").
	Pool string `json:"pool,omitzero"`
	// Pools specifies multiple node pools for deployment.
	Pools []string `json:"pools,omitzero"`
	// Zone specifies the availability zone for deployment.
	Zone string `json:"zone,omitzero"`
	// Zones specifies multiple availability zones for deployment.
	Zones []string `json:"zones,omitzero"`
	// Selectors is reserved for future scheduling extensions.
	Selectors map[string]string `json:"selectors,omitzero"`
}

// AppNetworkPolicySpec defines network policy configuration for the app.
type AppNetworkPolicySpec struct {
	// IngressRules defines additional ingress rules to allow.
	IngressRules []AppNetworkPolicyIngressRule `json:"ingressRules,omitzero"`
}

// AppNetworkPolicyIngressRule defines an ingress rule.
type AppNetworkPolicyIngressRule struct {
	// From defines the sources allowed by this rule.
	From []AppNetworkPolicyPeer `json:"from,omitzero"`
	// Ports defines the ports allowed by this rule.
	Ports []AppNetworkPolicyPort `json:"ports,omitzero"`
}

// AppNetworkPolicyPeer defines a network peer selector.
type AppNetworkPolicyPeer struct {
	// NamespaceSelector selects namespaces by label.
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitzero"`
}

// AppNetworkPolicyPort defines a port and protocol.
type AppNetworkPolicyPort struct {
	// Protocol is the network protocol (TCP, UDP, SCTP). Defaults to TCP.
	// +kubebuilder:validation:Enum=TCP;UDP;SCTP;""
	// +kubebuilder:default=TCP
	Protocol string `json:"protocol,omitzero"`
	// Port is the port number.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int `json:"port"`
}

// AppStatus defines the observed state of App.
type AppStatus struct {
	// OpsNamespace is the Kubernetes namespace for this app's operational resources.
	OpsNamespace string `json:"opsNamespace,omitzero"`
}

// Box represents a deployment unit within an App.
// This is a placeholder structure; detailed implementation follows in ADR-008.
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced
type Box struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitzero"`

	Spec   BoxSpec   `json:"spec,omitzero"`
	Status BoxStatus `json:"status,omitzero"`
}

// BoxSpec defines the desired state of Box.
// Box represents either a Compose Box (services from App.spec.compose)
// or a Standalone Box (independent image-based workload).
// The type is determined by the presence of spec.image:
//   - If spec.image is present: Standalone Box
//   - If spec.image is absent: Compose Box
type BoxSpec struct {
	// Component name (componentName) for this box.
	// If specified, must match metadata.name.
	// If empty, componentName is derived from metadata.name.
	Component string `json:"component,omitzero"`

	// Image specifies the container image for Standalone Box.
	// If present, this Box is a Standalone Box.
	// If absent, this Box is a Compose Box.
	Image string `json:"image,omitzero"`

	// Command overrides the default entrypoint for Standalone Box.
	// Only valid for Standalone Box (when spec.image is present).
	Command []string `json:"command,omitzero"`

	// Args provides additional arguments for Standalone Box.
	// Only valid for Standalone Box (when spec.image is present).
	Args []string `json:"args,omitzero"`

	// Ingress is reserved for future use and must not be specified.
	// External exposure is configured via App.spec.ingress.
	Ingress *BoxIngressSpec `json:"ingress,omitzero"`

	// NetworkPolicy defines network policy configuration for this Box.
	NetworkPolicy *AppNetworkPolicySpec `json:"networkPolicy,omitzero"`
}

// BoxIngressSpec is reserved for future use.
type BoxIngressSpec struct {
}

// BoxStatus defines the observed state of Box.
type BoxStatus struct {
}
