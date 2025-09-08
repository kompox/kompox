// Package kompoxopscfg defines the configuration schema (structs) for kompoxops.yml.
// This package is intended for YAML -> struct deserialization.
// Loading helpers and validations will be implemented separately.
package kompoxopscfg

// Root is the root structure of kompoxops.yml.
type Root struct {
	Version  string   `yaml:"version"`
	Service  Service  `yaml:"service"`
	Provider Provider `yaml:"provider"`
	Cluster  Cluster  `yaml:"cluster"`
	App      App      `yaml:"app"`
}

// Service represents global service settings.
type Service struct {
	Name string `yaml:"name"` // RFC1123-compliant DNS label
}

// Provider represents infrastructure provider configuration.
type Provider struct {
	Name     string            `yaml:"name"`     // provider name
	Driver   string            `yaml:"driver"`   // e.g., "aks", "k3s"
	Settings map[string]string `yaml:"settings"` // provider-specific settings
}

// Cluster represents target Kubernetes cluster configuration.
type Cluster struct {
	Name     string            `yaml:"name"`
	Existing bool              `yaml:"existing"` // whether to use existing cluster
	Ingress  ClusterIngress    `yaml:"ingress"`  // ingress configuration
	Settings map[string]string `yaml:"settings"` // cluster-specific settings
}

// ClusterIngress represents cluster-level ingress settings.
// Namespace: Kubernetes namespace for ingress controller resources
// Controller: Ingress controller type (e.g., "traefik")
// ServiceAccount: ServiceAccount name used by ingress workloads
type ClusterIngress struct {
	Namespace      string `yaml:"namespace"`
	Controller     string `yaml:"controller"`
	ServiceAccount string `yaml:"serviceAccount,omitempty"`
	// Domain is the default DNS domain used to generate app ingress hosts.
	// This is accepted under cluster.ingress for convenience; it will be propagated
	// to model.Cluster.Domain by the loader.
	Domain string `yaml:"domain,omitempty"`
	// CertResolver selects the Traefik ACME resolver to use (e.g., "staging" or "production").
	CertResolver string `yaml:"certResolver,omitempty"`
	// CertEmail is the email address used for ACME registration.
	CertEmail string `yaml:"certEmail,omitempty"`
	// Certificates defines static TLS certificates sourced from Key Vault, etc.
	Certificates []ClusterIngressCertificate `yaml:"certificates,omitempty"`
}

// ClusterIngressCertificate defines a named static certificate source.
// Currently only Azure Key Vault secret URLs are supported, for example:
//
//	https://<vault-name>.vault.azure.net/secret/<object-name>
//
// The name will be used to derive the Kubernetes TLS secret name as:
//
//	tls-<name>
type ClusterIngressCertificate struct {
	Name   string `yaml:"name"`
	Source string `yaml:"source"`
}

// App represents the target application to deploy.
type App struct {
	Name       string            `yaml:"name"`
	Compose    any               `yaml:"compose"` // compose.yml content or URL to fetch
	Ingress    AppIngress        `yaml:"ingress,omitempty"`
	Volumes    []AppVolume       `yaml:"volumes,omitempty"`
	Deployment AppDeployment     `yaml:"deployment,omitempty"`
	Resources  map[string]string `yaml:"resources,omitempty"` // pod resources (e.g., cpu, memory)
	Settings   map[string]string `yaml:"settings,omitempty"`  // app-specific settings
}

// AppIngress groups ingress-wide settings and routing rules.
type AppIngress struct {
	// CertResolver overrides cluster.ingress.certResolver when set.
	CertResolver string `yaml:"certResolver,omitempty"`
	// Rules defines the set of exposed ports and hostnames.
	Rules []AppIngressRule `yaml:"rules,omitempty"`
}

// AppIngressRule matches docs/Kompox-Convert-Draft schema.
type AppIngressRule struct {
	Name  string   `yaml:"name"`
	Port  int      `yaml:"port"`
	Hosts []string `yaml:"hosts"`
}

// AppVolume matches docs/Kompox-Convert-Draft schema for persistent volumes.
type AppVolume struct {
	Name    string         `yaml:"name"`
	Size    string         `yaml:"size"`
	Options map[string]any `yaml:"options,omitempty"` // provider-specific options for volume configuration
}

// AppDeployment defines deployment configuration for the app.
type AppDeployment struct {
	// Pool specifies the node pool for deployment.
	// Defaults to "user" if unspecified.
	Pool string `yaml:"pool,omitempty"`
	// Zone specifies the availability zone for deployment.
	// Only sets nodeSelector when specified.
	Zone string `yaml:"zone,omitempty"`
}
