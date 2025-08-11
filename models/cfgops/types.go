// Package cfgops defines the configuration schema (structs) for kompoxops.yml.
// This package is intended for YAML -> struct deserialization.
// Loading helpers and validations will be implemented separately.
package cfgops

// Root is the root structure of kompoxops.yml.
// Example:
// version: 1
// service: { name: ops, domain: ops.kompox.dev }
// cluster: { ... }
// app: { ... }
type Root struct {
	Version int     `yaml:"version"`
	Service Service `yaml:"service"`
	Cluster Cluster `yaml:"cluster"`
	App     App     `yaml:"app"`
}

// Service represents global service settings including default DNS domain.
type Service struct {
	Name   string `yaml:"name"`   // RFC1123-compliant DNS label
	Domain string `yaml:"domain"` // e.g., "ops.kompox.dev"
}

// Cluster represents target Kubernetes cluster and provider-specific settings.
type Cluster struct {
	Name     string            `yaml:"name"`
	Auth     ClusterAuth       `yaml:"auth"`
	Ingress  ClusterIngress    `yaml:"ingress"`
	Provider string            `yaml:"provider"`           // e.g., aks, k3s, eks
	Settings map[string]string `yaml:"settings,omitempty"` // provider-specific settings
}

// ClusterAuth describes how to connect to the cluster.
type ClusterAuth struct {
	Type       string `yaml:"type"`                 // e.g., kubectl
	Kubeconfig string `yaml:"kubeconfig,omitempty"` // e.g., ~/.kube/config
	Context    string `yaml:"context,omitempty"`    // e.g., my-aks
}

// ClusterIngress specifies the ingress controller deployment settings.
type ClusterIngress struct {
	Controller string `yaml:"controller"` // e.g., traefik
	Namespace  string `yaml:"namespace"`  // e.g., traefik
}

// App represents the target application to deploy.
type App struct {
	Name      string            `yaml:"name"`
	Compose   string            `yaml:"compose"`             // path to Docker Compose file (relative/absolute)
	Ingress   map[string]string `yaml:"ingress,omitempty"`   // per-port custom DNS (e.g., http_80, http_8080)
	Resources map[string]string `yaml:"resources,omitempty"` // pod resources (e.g., cpu, memory)
	Settings  map[string]string `yaml:"settings,omitempty"`  // provider-specific settings (e.g., AZURE_DISK_SIZE)
}
