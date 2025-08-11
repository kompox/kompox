package aks

import "github.com/yaegashi/kompoxops/provider"

// driver implements the AKS provider driver.
type driver struct{}

// ID returns the provider identifier.
func (d driver) ID() string { return "aks" }

// init registers the AKS driver.
func init() {
	provider.Register("aks", func(settings map[string]string) (provider.Driver, error) {
		// TODO: Validate settings when AKS-specific options are added.
		return driver{}, nil
	})
}
