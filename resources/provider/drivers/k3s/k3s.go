package k3s

import (
	"fmt"

	"github.com/yaegashi/kompoxops/resources/provider"
)

// driver implements the K3s provider driver.
type driver struct{}

// ID returns the provider identifier.
func (d *driver) ID() string { return "k3s" }

// init registers the K3s driver.
func init() {
	provider.Register("k3s", func(settings map[string]string) (provider.Driver, error) {
		if settings != nil && settings["disabled"] == "true" {
			return nil, fmt.Errorf("k3s provider disabled by settings")
		}
		return &driver{}, nil
	})
}
