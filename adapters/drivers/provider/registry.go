package providerdrv

import (
	"context"

	"github.com/yaegashi/kompoxops/domain/model"
)

// Driver abstracts provider-specific behavior (identifier, hooks, etc.).
// Implementations live under adapters/drivers/provider/<name> and should return a
// provider driver identifier such as "aks" via ID().
type Driver interface {
	// ID returns the provider driver identifier (e.g., "aks").
	ID() string

	// ServiceName returns the service name associated with this driver instance.
	// May return "(nil)" if no service is associated (e.g., for testing).
	ServiceName() string

	// ProviderName returns the provider name associated with this driver instance.
	ProviderName() string

	// ClusterProvision provisions a Kubernetes cluster according to the cluster specification.
	ClusterProvision(ctx context.Context, cluster *model.Cluster) error

	// ClusterDeprovision deprovisions a Kubernetes cluster according to the cluster specification.
	ClusterDeprovision(ctx context.Context, cluster *model.Cluster) error

	// ClusterStatus returns the status of a Kubernetes cluster.
	ClusterStatus(ctx context.Context, cluster *model.Cluster) (*model.ClusterStatus, error)

	// ClusterInstall installs in-cluster resources (Ingress Controller, etc.).
	ClusterInstall(ctx context.Context, cluster *model.Cluster) error

	// ClusterUninstall uninstalls in-cluster resources (Ingress Controller, etc.).
	ClusterUninstall(ctx context.Context, cluster *model.Cluster) error

	// ClusterKubeconfig returns kubeconfig bytes for connecting to the target cluster.
	// Implementations may fetch admin/user credentials depending on provider capability.
	ClusterKubeconfig(ctx context.Context, cluster *model.Cluster) ([]byte, error)

	// VolumeInstanceList returns a list of volume instances of the specified volume.
	VolumeInstanceList(ctx context.Context, cluster *model.Cluster, app *model.App, volName string) ([]*model.AppVolumeInstance, error)

	// VolumeInstanceCreate creates a volume instance of the specified volume.
	VolumeInstanceCreate(ctx context.Context, cluster *model.Cluster, app *model.App, volName string) (*model.AppVolumeInstance, error)

	// VolumeInstanceAssign assigns a volume instance to the specified volume.
	VolumeInstanceAssign(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, volInstName string) error

	// VolumeInstanceDelete deletes a volume instance of the specified volume.
	VolumeInstanceDelete(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, volInstName string) error

	// VolumeClass returns provider specific volume provisioning parameters for the given logical volume.
	// Empty fields mean "no opinion" and the caller should omit them from generated manifests rather than
	// substituting provider-specific defaults. This keeps kube layer free from provider assumptions.
	VolumeClass(ctx context.Context, cluster *model.Cluster, app *model.App, vol model.AppVolume) (VolumeClass, error)
}

// VolumeClass represents provider specific volume provisioning parameters.
// All fields are optional. The kube adapter must not inject provider defaults; if a field is empty it should
// be simply omitted from the PV/PVC spec letting the cluster defaulting logic apply (e.g. default storage class).
type VolumeClass struct {
	StorageClassName string            // e.g. "managed-csi" (omit if empty)
	CSIDriver        string            // CSI driver name (omit if empty)
	FSType           string            // Filesystem type, e.g. ext4 (omit if empty)
	Attributes       map[string]string // CSI volumeAttributes (nil/empty => none)
	AccessModes      []string          // e.g. ["ReadWriteOnce"]. Empty => use caller default mapping.
	ReclaimPolicy    string            // "Retain" | "Delete" (empty => caller default)
	VolumeMode       string            // "Filesystem" | "Block" (empty => caller default)
	_                struct{}          // future-proofing
}

// driverFactory is a constructor function for a provider driver.
type driverFactory func(service *model.Service, provider *model.Provider) (Driver, error)

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
