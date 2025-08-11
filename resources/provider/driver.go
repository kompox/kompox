package provider

// Driver abstracts provider-specific behavior (identifier, hooks, etc.).
// Implementations live under provider/drivers/<name> and should return a
// provider identifier such as "aks" via ID().
type Driver interface {
	// ID returns the provider identifier (e.g., "aks").
	ID() string
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
