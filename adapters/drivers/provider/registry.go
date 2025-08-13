package providerdrv

import "github.com/yaegashi/kompoxops/domain/model"

// Driver abstracts provider-specific behavior (identifier, hooks, etc.).
// Implementations live under adapters/drivers/provider/<name> and should return a
// provider identifier such as "aks" via ID().
type Driver interface {
	// ID returns the provider identifier (e.g., "aks").
	ID() string

	// ClusterProvision provisions a Kubernetes cluster according to the cluster specification.
	ClusterProvision(cluster *model.Cluster) error

	// ClusterDeprovision deprovisions a Kubernetes cluster according to the cluster specification.
	ClusterDeprovision(cluster *model.Cluster) error
}

// driverFactory is a constructor function for a provider driver.
type driverFactory func(settings map[string]string) (Driver, error)

// registry holds registered drivers by name.
var registry = map[string]driverFactory{}

// Register makes a driver available by the given name. Drivers should call
// this from their init() function.
func Register(name string, factory driverFactory) {
	registry[name] = factory
}

// GetDriverFactory returns the driver factory function for the given name.
func GetDriverFactory(name string) (driverFactory, bool) {
	factory, exists := registry[name]
	return factory, exists
}
