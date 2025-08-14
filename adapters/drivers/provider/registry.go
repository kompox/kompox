package providerdrv

import "github.com/yaegashi/kompoxops/domain/model"

// ClusterStatus represents the status of a cluster.
type ClusterStatus struct {
	Existing    bool `json:"existing"`    // cluster.existing の設定値
	Provisioned bool `json:"provisioned"` // K8s クラスタが存在するとき true
	Installed   bool `json:"installed"`   // K8s クラスタ内のリソースが存在するとき true
}

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

	// ClusterStatus returns the status of a Kubernetes cluster.
	ClusterStatus(cluster *model.Cluster) (*ClusterStatus, error)
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
