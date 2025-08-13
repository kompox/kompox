// Package cfgops defines the configuration schema (structs) for kompoxops.yml.
// This package is intended for YAML -> struct deserialization.
// Loading helpers and validations will be implemented separately.
package cfgops

// Root is the root structure of kompoxops.yml.
// Example:
// version: v1
// service: { name: ops }
// provider: { name: aks1, driver: aks, settings: {...} }
// cluster: { name: my-aks, existing: false, domain: ops.kompox.dev, ... }
// app: { name: my-app, compose: compose.yml, ... }
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
	Name     string                 `yaml:"name"`
	Existing bool                   `yaml:"existing"` // whether to use existing cluster
	Domain   string                 `yaml:"domain"`   // e.g., "ops.kompox.dev"
	Ingress  map[string]interface{} `yaml:"ingress"`  // ingress configuration
	Settings map[string]string      `yaml:"settings"` // cluster-specific settings
}

// App represents the target application to deploy.
type App struct {
	Name      string            `yaml:"name"`
	Compose   string            `yaml:"compose"`             // path to Docker Compose file (relative/absolute)
	Ingress   map[string]string `yaml:"ingress,omitempty"`   // per-port custom DNS (e.g., http_80, http_8080)
	Resources map[string]string `yaml:"resources,omitempty"` // pod resources (e.g., cpu, memory)
	Settings  map[string]string `yaml:"settings,omitempty"`  // app-specific settings
}
