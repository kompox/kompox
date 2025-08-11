package aks

// Provider implements the AKS provider.
type Provider struct{}

// ID returns the provider identifier.
func (p Provider) ID() string { return "aks" }

// New returns a new AKS provider instance.
// settings: provider-specific settings from cfg.Cluster.Settings.
// Return nil if settings are invalid for this provider.
func New(settings map[string]string) *Provider {
	// TODO: Validate settings if/when AKS-specific keys are introduced.
	// For now, accept any map (including nil) as valid.
	_ = settings
	return &Provider{}
}
