package provider

import "fmt"

// Provider is a configured provider instance composed of a selected driver
// and its provider-specific settings.
type Provider struct {
	Name     string
	Driver   Driver
	Settings map[string]string
}

// New constructs a Provider by looking up a registered driver by name and
// instantiating it with the provided settings.
func New(name string, settings map[string]string) (*Provider, error) {
	if name == "" {
		return &Provider{Name: "", Driver: nil, Settings: settings}, nil
	}
	factory, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", name)
	}
	drv, err := factory(settings)
	if err != nil {
		return nil, err
	}
	return &Provider{Name: name, Driver: drv, Settings: settings}, nil
}
