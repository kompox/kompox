package model

import "context"

// NodePool represents a provider-agnostic node pool abstraction.
// It supports both creation and listing operations, using pointer fields
// to distinguish between "not set" and "set to zero value" for updates.
type NodePool struct {
	// Name is the logical node pool name (e.g., "system", "user", or custom names).
	Name *string

	// ProviderName is the provider-specific name or identifier (may differ from Name).
	ProviderName *string

	// Mode indicates the pool purpose: "system" or "user".
	Mode *string

	// Labels are Kubernetes node labels to apply to nodes in this pool.
	Labels *map[string]string

	// Zones are availability zones for node placement (e.g., ["japaneast-1", "japaneast-2"]).
	Zones *[]string

	// InstanceType is the VM/machine type (e.g., "Standard_D2s_v3").
	InstanceType *string

	// OSDiskType is the OS disk type (e.g., "Managed", "Ephemeral").
	OSDiskType *string

	// OSDiskSizeGiB is the OS disk size in GiB.
	OSDiskSizeGiB *int

	// Priority indicates VM priority: "regular" or "spot".
	Priority *string

	// Autoscaling contains autoscaling configuration if enabled.
	Autoscaling *NodePoolAutoscaling

	// Status contains runtime status information (read-only, populated by List).
	Status *NodePoolStatus

	// Extensions holds provider-specific fields not covered by the common model.
	Extensions map[string]any
}

// NodePoolAutoscaling defines autoscaling parameters for a node pool.
type NodePoolAutoscaling struct {
	// Enabled indicates whether autoscaling is active.
	Enabled bool

	// Min is the minimum node count (required if Enabled is true).
	Min int

	// Max is the maximum node count (required if Enabled is true).
	Max int

	// Desired is the target node count when autoscaling is disabled.
	// Ignored if Enabled is true.
	Desired *int
}

// NodePoolStatus represents runtime status of a node pool (read-only).
type NodePoolStatus struct {
	// ProvisioningState is the provider-specific provisioning state (e.g., "Succeeded", "Creating").
	ProvisioningState *string

	// CurrentNodeCount is the current number of nodes in the pool.
	CurrentNodeCount *int

	// Extensions holds provider-specific status fields.
	Extensions map[string]any
}

// NodePoolListOptions holds options for listing node pools.
type NodePoolListOptions struct {
	// Name filters by node pool name if non-empty.
	Name string
}

// NodePoolCreateOptions holds options for creating a node pool.
type NodePoolCreateOptions struct {
	Force bool
}

// NodePoolUpdateOptions holds options for updating a node pool.
type NodePoolUpdateOptions struct {
	Force bool
}

// NodePoolDeleteOptions holds options for deleting a node pool.
type NodePoolDeleteOptions struct {
	Force bool
}

// Functional option types for node pool operations.
type NodePoolListOption func(*NodePoolListOptions)
type NodePoolCreateOption func(*NodePoolCreateOptions)
type NodePoolUpdateOption func(*NodePoolUpdateOptions)
type NodePoolDeleteOption func(*NodePoolDeleteOptions)

// Option constructors for node pool operations.
func WithNodePoolListName(name string) NodePoolListOption {
	return func(o *NodePoolListOptions) { o.Name = name }
}

func WithNodePoolCreateForce() NodePoolCreateOption {
	return func(o *NodePoolCreateOptions) { o.Force = true }
}

func WithNodePoolUpdateForce() NodePoolUpdateOption {
	return func(o *NodePoolUpdateOptions) { o.Force = true }
}

func WithNodePoolDeleteForce() NodePoolDeleteOption {
	return func(o *NodePoolDeleteOptions) { o.Force = true }
}

// ApplyNodePoolListOptions applies functional options to NodePoolListOptions.
func ApplyNodePoolListOptions(opts ...NodePoolListOption) NodePoolListOptions {
	var o NodePoolListOptions
	for _, fn := range opts {
		fn(&o)
	}
	return o
}

// ApplyNodePoolCreateOptions applies functional options to NodePoolCreateOptions.
func ApplyNodePoolCreateOptions(opts ...NodePoolCreateOption) NodePoolCreateOptions {
	var o NodePoolCreateOptions
	for _, fn := range opts {
		fn(&o)
	}
	return o
}

// ApplyNodePoolUpdateOptions applies functional options to NodePoolUpdateOptions.
func ApplyNodePoolUpdateOptions(opts ...NodePoolUpdateOption) NodePoolUpdateOptions {
	var o NodePoolUpdateOptions
	for _, fn := range opts {
		fn(&o)
	}
	return o
}

// ApplyNodePoolDeleteOptions applies functional options to NodePoolDeleteOptions.
func ApplyNodePoolDeleteOptions(opts ...NodePoolDeleteOption) NodePoolDeleteOptions {
	var o NodePoolDeleteOptions
	for _, fn := range opts {
		fn(&o)
	}
	return o
}

// NodePoolPort defines node pool management operations.
// Implementations are provided by provider drivers.
type NodePoolPort interface {
	// NodePoolList returns a list of node pools for the specified cluster.
	NodePoolList(ctx context.Context, cluster *Cluster, opts ...NodePoolListOption) ([]*NodePool, error)

	// NodePoolCreate creates a new node pool in the cluster.
	NodePoolCreate(ctx context.Context, cluster *Cluster, pool NodePool, opts ...NodePoolCreateOption) (*NodePool, error)

	// NodePoolUpdate updates mutable fields of an existing node pool.
	NodePoolUpdate(ctx context.Context, cluster *Cluster, pool NodePool, opts ...NodePoolUpdateOption) (*NodePool, error)

	// NodePoolDelete deletes the specified node pool from the cluster.
	NodePoolDelete(ctx context.Context, cluster *Cluster, poolName string, opts ...NodePoolDeleteOption) error
}
