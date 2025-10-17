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
	// Settings stores cluster-level configuration.
	Settings map[string]string `json:"settings,omitzero"`
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
	// Size is the volume size in bytes.
	Size int64 `json:"size,omitzero"`
	// Options are provider-specific volume options (e.g., SKU, IOPS).
	Options map[string]any `json:"options,omitzero"`
}

// AppDeploymentSpec defines deployment configuration for the app.
type AppDeploymentSpec struct {
	// Pool specifies the node pool for deployment (defaults to "user").
	Pool string `json:"pool,omitzero"`
	// Zone specifies the availability zone for deployment.
	Zone string `json:"zone,omitzero"`
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
// Currently a placeholder; to be expanded in ADR-008.
type BoxSpec struct {
	// Component name (componentName) for this box.
	// If empty, defaults to the app name.
	Component string `json:"component,omitzero"`
}

// BoxStatus defines the observed state of Box.
type BoxStatus struct {
}
