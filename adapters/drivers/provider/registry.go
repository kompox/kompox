package providerdrv

import (
	"context"

	"github.com/kompox/kompox/domain/model"
)

// Driver abstracts provider-specific behavior (identifier, hooks, etc.).
// Implementations live under adapters/drivers/provider/<name> and should return a
// provider driver identifier such as "aks" via ID().
type Driver interface {
	// ID returns the provider driver identifier (e.g., "aks").
	ID() string

	// WorkspaceName returns the workspace name associated with this driver instance.
	// May return "(nil)" if no workspace is associated (e.g., for testing).
	WorkspaceName() string

	// ProviderName returns the provider name associated with this driver instance.
	ProviderName() string

	// ClusterProvision provisions a Kubernetes cluster according to the cluster specification.
	ClusterProvision(ctx context.Context, cluster *model.Cluster, opts ...model.ClusterProvisionOption) error

	// ClusterDeprovision deprovisions a Kubernetes cluster according to the cluster specification.
	ClusterDeprovision(ctx context.Context, cluster *model.Cluster, opts ...model.ClusterDeprovisionOption) error

	// ClusterStatus returns the status of a Kubernetes cluster.
	ClusterStatus(ctx context.Context, cluster *model.Cluster) (*model.ClusterStatus, error)

	// ClusterInstall installs in-cluster resources (Ingress Controller, etc.).
	ClusterInstall(ctx context.Context, cluster *model.Cluster, opts ...model.ClusterInstallOption) error

	// ClusterUninstall uninstalls in-cluster resources (Ingress Controller, etc.).
	ClusterUninstall(ctx context.Context, cluster *model.Cluster, opts ...model.ClusterUninstallOption) error

	// ClusterKubeconfig returns kubeconfig bytes for connecting to the target cluster.
	// Implementations may fetch admin/user credentials depending on provider capability.
	ClusterKubeconfig(ctx context.Context, cluster *model.Cluster) ([]byte, error)

	// ClusterDNSApply applies a DNS record set in the provider-managed DNS zones.
	// The method must be idempotent and best-effort: providers should suppress recoverable
	// write failures unless opts request strict handling. Invalid input or context
	// cancellation should still return an error.
	ClusterDNSApply(ctx context.Context, cluster *model.Cluster, rset model.DNSRecordSet, opts ...model.ClusterDNSApplyOption) error

	// VolumeDiskList returns a list of disks of the specified logical volume.
	VolumeDiskList(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, opts ...model.VolumeDiskListOption) ([]*model.VolumeDisk, error)

	// VolumeDiskCreate creates a disk of the specified logical volume. diskName and source
	// are forwarded from CLI/usecase as opaque strings. Empty values indicate provider defaults.
	VolumeDiskCreate(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, diskName string, source string, opts ...model.VolumeDiskCreateOption) (*model.VolumeDisk, error)

	// VolumeDiskDelete deletes a disk of the specified logical volume.
	VolumeDiskDelete(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, diskName string, opts ...model.VolumeDiskDeleteOption) error

	// VolumeDiskAssign assigns a disk to the specified logical volume.
	VolumeDiskAssign(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, diskName string, opts ...model.VolumeDiskAssignOption) error

	// VolumeSnapshotList returns a list of snapshots of the specified volume.
	VolumeSnapshotList(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, opts ...model.VolumeSnapshotListOption) ([]*model.VolumeSnapshot, error)

	// VolumeSnapshotCreate creates a snapshot. snapName and source follow the same semantics as
	// disk creation and must be interpreted by the driver.
	VolumeSnapshotCreate(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, snapName string, source string, opts ...model.VolumeSnapshotCreateOption) (*model.VolumeSnapshot, error)

	// VolumeSnapshotDelete deletes the specified snapshot.
	VolumeSnapshotDelete(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, snapName string, opts ...model.VolumeSnapshotDeleteOption) error

	// VolumeClass returns provider specific volume provisioning parameters for the given logical volume.
	// Empty fields mean "no opinion" and the caller should omit them from generated manifests rather than
	// substituting provider-specific defaults. This keeps kube layer free from provider assumptions.
	VolumeClass(ctx context.Context, cluster *model.Cluster, app *model.App, vol model.AppVolume) (model.VolumeClass, error)
}

// driverFactory is a constructor function for a provider driver.
type driverFactory func(workspace *model.Workspace, provider *model.Provider) (Driver, error)

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
