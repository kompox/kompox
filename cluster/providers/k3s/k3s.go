package k3s

// Provider implements the K3s provider.
type Provider struct{}

// ID returns the provider identifier.
func (p Provider) ID() string { return "k3s" }

// New returns a new K3s provider instance.
// settings: provider-specific settings from cfg.Cluster.Settings.
// Return nil if settings are invalid for this provider.
func New(settings map[string]string) *Provider {
	// Example validation: reject an explicitly invalid flag
	if settings != nil {
		if settings["disabled"] == "true" {
			return nil
		}
	}
	return &Provider{}
}
