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
	Name     string                 `yaml:"name"`
	Existing bool                   `yaml:"existing"` // whether to use existing cluster
	Domain   string                 `yaml:"domain"`   // e.g., "ops.kompox.dev"
	Ingress  map[string]interface{} `yaml:"ingress"`  // ingress configuration
	Settings map[string]string      `yaml:"settings"` // cluster-specific settings
}

// App represents the target application to deploy.
type App struct {
	Name      string            `yaml:"name"`
	Compose   any               `yaml:"compose"` // compose.yml content or URL to fetch
	Ingress   []AppIngressRule  `yaml:"ingress,omitempty"`
	Volumes   []AppVolume       `yaml:"volumes,omitempty"`
	Resources map[string]string `yaml:"resources,omitempty"` // pod resources (e.g., cpu, memory)
	Settings  map[string]string `yaml:"settings,omitempty"`  // app-specific settings
}

// AppIngressRule matches docs/Kompox-Convert-Draft schema.
type AppIngressRule struct {
	Name  string   `yaml:"name"`
	Port  int      `yaml:"port"`
	Hosts []string `yaml:"hosts"`
}

// AppVolume matches docs/Kompox-Convert-Draft schema for persistent volumes.
type AppVolume struct {
	Name string `yaml:"name"`
	Size string `yaml:"size"`
}
