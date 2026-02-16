package aks

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v2"
	"github.com/kompox/kompox/domain/model"
	"github.com/kompox/kompox/internal/logging"
)

// Kompox label keys for node pools.
const (
	labelNodePool = "kompox.dev/node-pool"
	labelNodeZone = "kompox.dev/node-zone"
)

// NodePoolList returns a list of node pools for the specified cluster.
func (d *driver) NodePoolList(ctx context.Context, cluster *model.Cluster, opts ...model.NodePoolListOption) (pools []*model.NodePool, err error) {
	ctx, cleanup := d.withMethodLogger(ctx, "NodePoolList")
	defer func() { cleanup(err) }()

	log := logging.FromContext(ctx)
	o := model.ApplyNodePoolListOptions(opts...)

	// Get resource group and cluster name
	rgName, err := d.clusterResourceGroupName(cluster)
	if err != nil || rgName == "" {
		return nil, fmt.Errorf("derive cluster resource group: %w", err)
	}
	clusterName, err := d.azureAksClusterName(cluster)
	if err != nil {
		return nil, fmt.Errorf("derive cluster name: %w", err)
	}

	// Create agent pools client
	client, err := armcontainerservice.NewAgentPoolsClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return nil, fmt.Errorf("create agent pools client: %w", err)
	}

	// List all agent pools
	pager := client.NewListPager(rgName, clusterName, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			// Resource group or cluster not found -> return empty list
			if isNotFoundError(err) {
				log.Info(ctx, "cluster not found, returning empty pool list",
					"resourceGroup", rgName,
					"cluster", clusterName)
				return []*model.NodePool{}, nil
			}
			return nil, fmt.Errorf("list agent pools: %w", err)
		}

		for _, agentPool := range page.Value {
			if agentPool == nil || agentPool.Name == nil {
				continue
			}

			// Apply name filter if specified
			if o.Name != "" && *agentPool.Name != o.Name {
				continue
			}

			pool := d.agentPoolToNodePool(agentPool)
			pools = append(pools, pool)
		}
	}

	return pools, nil
}

// NodePoolCreate creates a new node pool in the cluster.
func (d *driver) NodePoolCreate(ctx context.Context, cluster *model.Cluster, pool model.NodePool, opts ...model.NodePoolCreateOption) (result *model.NodePool, err error) {
	ctx, cleanup := d.withMethodLogger(ctx, "NodePoolCreate")
	defer func() { cleanup(err) }()

	log := logging.FromContext(ctx)
	_ = model.ApplyNodePoolCreateOptions(opts...)

	// Validate required fields
	if pool.Name == nil || *pool.Name == "" {
		return nil, fmt.Errorf("validation error: pool.Name is required")
	}

	// Get resource group and cluster name
	rgName, err := d.clusterResourceGroupName(cluster)
	if err != nil || rgName == "" {
		return nil, fmt.Errorf("derive cluster resource group: %w", err)
	}
	clusterName, err := d.azureAksClusterName(cluster)
	if err != nil {
		return nil, fmt.Errorf("derive cluster name: %w", err)
	}

	// Build agent pool profile
	agentPool, err := d.nodePoolToAgentPoolProfile(pool)
	if err != nil {
		return nil, fmt.Errorf("build agent pool profile: %w", err)
	}

	// Create agent pools client
	client, err := armcontainerservice.NewAgentPoolsClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return nil, fmt.Errorf("create agent pools client: %w", err)
	}

	log.Info(ctx, "creating agent pool",
		"resourceGroup", rgName,
		"cluster", clusterName,
		"poolName", *pool.Name)

	// Begin create or update operation
	poller, err := client.BeginCreateOrUpdate(ctx, rgName, clusterName, *pool.Name, agentPool, nil)
	if err != nil {
		return nil, fmt.Errorf("begin create agent pool: %w", err)
	}

	// Wait for completion
	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("wait for agent pool creation: %w", err)
	}

	return d.agentPoolToNodePool(&resp.AgentPool), nil
}

// NodePoolUpdate updates mutable fields of an existing node pool.
func (d *driver) NodePoolUpdate(ctx context.Context, cluster *model.Cluster, pool model.NodePool, opts ...model.NodePoolUpdateOption) (result *model.NodePool, err error) {
	ctx, cleanup := d.withMethodLogger(ctx, "NodePoolUpdate")
	defer func() { cleanup(err) }()

	log := logging.FromContext(ctx)
	_ = model.ApplyNodePoolUpdateOptions(opts...)

	// Validate required fields
	if pool.Name == nil || *pool.Name == "" {
		return nil, fmt.Errorf("validation error: pool.Name is required")
	}

	// Get resource group and cluster name
	rgName, err := d.clusterResourceGroupName(cluster)
	if err != nil || rgName == "" {
		return nil, fmt.Errorf("derive cluster resource group: %w", err)
	}
	clusterName, err := d.azureAksClusterName(cluster)
	if err != nil {
		return nil, fmt.Errorf("derive cluster name: %w", err)
	}

	// Create agent pools client
	client, err := armcontainerservice.NewAgentPoolsClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return nil, fmt.Errorf("create agent pools client: %w", err)
	}

	// Get existing agent pool
	existing, err := client.Get(ctx, rgName, clusterName, *pool.Name, nil)
	if err != nil {
		return nil, fmt.Errorf("get existing agent pool: %w", err)
	}

	// Check for immutable field changes
	if err := d.validateImmutableFields(pool, &existing.AgentPool); err != nil {
		return nil, err
	}

	// Merge mutable fields into existing profile
	updated := d.mergeMutableFields(pool, &existing.AgentPool)

	log.Info(ctx, "updating agent pool",
		"resourceGroup", rgName,
		"cluster", clusterName,
		"poolName", *pool.Name)

	// Begin update operation
	poller, err := client.BeginCreateOrUpdate(ctx, rgName, clusterName, *pool.Name, updated, nil)
	if err != nil {
		return nil, fmt.Errorf("begin update agent pool: %w", err)
	}

	// Wait for completion
	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("wait for agent pool update: %w", err)
	}

	return d.agentPoolToNodePool(&resp.AgentPool), nil
}

// NodePoolDelete deletes the specified node pool from the cluster.
func (d *driver) NodePoolDelete(ctx context.Context, cluster *model.Cluster, poolName string, opts ...model.NodePoolDeleteOption) (err error) {
	ctx, cleanup := d.withMethodLogger(ctx, "NodePoolDelete")
	defer func() { cleanup(err) }()

	log := logging.FromContext(ctx)
	_ = model.ApplyNodePoolDeleteOptions(opts...)

	// Get resource group and cluster name
	rgName, err := d.clusterResourceGroupName(cluster)
	if err != nil || rgName == "" {
		return fmt.Errorf("derive cluster resource group: %w", err)
	}
	clusterName, err := d.azureAksClusterName(cluster)
	if err != nil {
		return fmt.Errorf("derive cluster name: %w", err)
	}

	// Create agent pools client
	client, err := armcontainerservice.NewAgentPoolsClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return fmt.Errorf("create agent pools client: %w", err)
	}

	log.Info(ctx, "deleting agent pool",
		"resourceGroup", rgName,
		"cluster", clusterName,
		"poolName", poolName)

	// Begin delete operation
	poller, err := client.BeginDelete(ctx, rgName, clusterName, poolName, nil)
	if err != nil {
		// NotFound is acceptable for idempotency
		if isNotFoundError(err) {
			log.Info(ctx, "agent pool not found, considering delete successful",
				"poolName", poolName)
			return nil
		}
		return fmt.Errorf("begin delete agent pool: %w", err)
	}

	// Wait for completion
	if _, err := poller.PollUntilDone(ctx, nil); err != nil {
		// NotFound during polling is also acceptable
		if isNotFoundError(err) {
			log.Info(ctx, "agent pool deleted during polling",
				"poolName", poolName)
			return nil
		}
		return fmt.Errorf("wait for agent pool deletion: %w", err)
	}

	return nil
}

// agentPoolToNodePool converts an AKS agent pool to Kompox NodePool.
func (d *driver) agentPoolToNodePool(agentPool *armcontainerservice.AgentPool) *model.NodePool {
	if agentPool == nil {
		return nil
	}

	pool := &model.NodePool{
		Extensions: make(map[string]any),
	}

	if agentPool.Name != nil {
		pool.Name = to.Ptr(*agentPool.Name)
		pool.ProviderName = to.Ptr(*agentPool.Name)
	}

	if agentPool.Properties != nil {
		props := agentPool.Properties

		// Mode: System or User -> system or user
		if props.Mode != nil {
			mode := strings.ToLower(string(*props.Mode))
			pool.Mode = &mode
		}

		// Instance type
		if props.VMSize != nil {
			pool.InstanceType = to.Ptr(*props.VMSize)
		}

		// OS Disk
		if props.OSDiskType != nil {
			diskType := string(*props.OSDiskType)
			pool.OSDiskType = &diskType
		}
		if props.OSDiskSizeGB != nil {
			size := int(*props.OSDiskSizeGB)
			pool.OSDiskSizeGiB = &size
		}

		// Priority
		if props.ScaleSetPriority != nil {
			priority := strings.ToLower(string(*props.ScaleSetPriority))
			pool.Priority = &priority
		}

		// Zones
		if props.AvailabilityZones != nil && len(props.AvailabilityZones) > 0 {
			zones := make([]string, len(props.AvailabilityZones))
			for i, z := range props.AvailabilityZones {
				if z != nil {
					zones[i] = d.normalizeAksZoneToKompox(*z)
				}
			}
			pool.Zones = &zones
		}

		// Labels
		if props.NodeLabels != nil {
			labels := make(map[string]string)
			for k, v := range props.NodeLabels {
				if v != nil {
					labels[k] = *v
				}
			}
			if len(labels) > 0 {
				pool.Labels = &labels
			}
		}

		// Autoscaling
		autoscaling := &model.NodePoolAutoscaling{}
		if props.EnableAutoScaling != nil {
			autoscaling.Enabled = *props.EnableAutoScaling
		}
		if props.MinCount != nil {
			autoscaling.Min = int(*props.MinCount)
		}
		if props.MaxCount != nil {
			autoscaling.Max = int(*props.MaxCount)
		}
		if props.Count != nil && !autoscaling.Enabled {
			count := int(*props.Count)
			autoscaling.Desired = &count
		}
		pool.Autoscaling = autoscaling

		// Status
		status := &model.NodePoolStatus{
			Extensions: make(map[string]any),
		}
		if props.ProvisioningState != nil {
			status.ProvisioningState = to.Ptr(*props.ProvisioningState)
		}
		if props.Count != nil {
			count := int(*props.Count)
			status.CurrentNodeCount = &count
		}
		pool.Status = status
	}

	return pool
}

// nodePoolToAgentPoolProfile converts a Kompox NodePool to AKS agent pool profile.
func (d *driver) nodePoolToAgentPoolProfile(pool model.NodePool) (armcontainerservice.AgentPool, error) {
	agentPool := armcontainerservice.AgentPool{
		Properties: &armcontainerservice.ManagedClusterAgentPoolProfileProperties{},
	}

	props := agentPool.Properties

	// Mode: system or user -> System or User
	if pool.Mode != nil {
		mode := armcontainerservice.AgentPoolMode(strings.Title(strings.ToLower(*pool.Mode)))
		props.Mode = &mode
	}

	// Instance type
	if pool.InstanceType != nil {
		props.VMSize = to.Ptr(*pool.InstanceType)
	}

	// OS Disk
	if pool.OSDiskType != nil {
		diskType := armcontainerservice.OSDiskType(*pool.OSDiskType)
		props.OSDiskType = &diskType
	}
	if pool.OSDiskSizeGiB != nil {
		size := int32(*pool.OSDiskSizeGiB)
		props.OSDiskSizeGB = &size
	}

	// Priority
	if pool.Priority != nil {
		priority := armcontainerservice.ScaleSetPriority(strings.Title(strings.ToLower(*pool.Priority)))
		props.ScaleSetPriority = &priority
	}

	// Zones: Kompox format -> AKS format
	if pool.Zones != nil && len(*pool.Zones) > 0 {
		zones := make([]*string, len(*pool.Zones))
		for i, z := range *pool.Zones {
			zones[i] = to.Ptr(d.normalizeKompoxZoneToAks(z))
		}
		props.AvailabilityZones = zones
	}

	// Labels: merge with Kompox labels
	labels := make(map[string]*string)
	if pool.Labels != nil {
		for k, v := range *pool.Labels {
			labels[k] = to.Ptr(v)
		}
	}
	// Add Kompox labels
	if pool.Name != nil {
		labels[labelNodePool] = to.Ptr(*pool.Name)
	}
	if pool.Zones != nil && len(*pool.Zones) > 0 {
		// Add zone labels for each zone
		for _, z := range *pool.Zones {
			// For multi-zone pools, we add the first zone as primary
			labels[labelNodeZone] = to.Ptr(z)
			break
		}
	}
	if len(labels) > 0 {
		props.NodeLabels = labels
	}

	// Autoscaling
	if pool.Autoscaling != nil {
		props.EnableAutoScaling = to.Ptr(pool.Autoscaling.Enabled)
		if pool.Autoscaling.Enabled {
			props.MinCount = to.Ptr(int32(pool.Autoscaling.Min))
			props.MaxCount = to.Ptr(int32(pool.Autoscaling.Max))
		} else if pool.Autoscaling.Desired != nil {
			props.Count = to.Ptr(int32(*pool.Autoscaling.Desired))
		}
	}

	return agentPool, nil
}

// validateImmutableFields checks if any immutable fields are being changed.
func (d *driver) validateImmutableFields(update model.NodePool, existing *armcontainerservice.AgentPool) error {
	if existing == nil || existing.Properties == nil {
		return nil
	}

	props := existing.Properties
	var errs []string

	// Mode is immutable
	if update.Mode != nil {
		existingMode := strings.ToLower(string(*props.Mode))
		if *update.Mode != existingMode {
			errs = append(errs, "Mode is immutable")
		}
	}

	// InstanceType (VMSize) is immutable
	if update.InstanceType != nil && props.VMSize != nil {
		if *update.InstanceType != *props.VMSize {
			errs = append(errs, "InstanceType is immutable")
		}
	}

	// OSDiskType is immutable
	if update.OSDiskType != nil && props.OSDiskType != nil {
		if *update.OSDiskType != string(*props.OSDiskType) {
			errs = append(errs, "OSDiskType is immutable")
		}
	}

	// OSDiskSizeGiB is immutable
	if update.OSDiskSizeGiB != nil && props.OSDiskSizeGB != nil {
		if *update.OSDiskSizeGiB != int(*props.OSDiskSizeGB) {
			errs = append(errs, "OSDiskSizeGiB is immutable")
		}
	}

	// Priority is immutable
	if update.Priority != nil && props.ScaleSetPriority != nil {
		existingPriority := strings.ToLower(string(*props.ScaleSetPriority))
		if *update.Priority != existingPriority {
			errs = append(errs, "Priority is immutable")
		}
	}

	// Zones are immutable
	if update.Zones != nil && props.AvailabilityZones != nil {
		updateZones := *update.Zones
		existingZones := make([]string, len(props.AvailabilityZones))
		for i, z := range props.AvailabilityZones {
			if z != nil {
				existingZones[i] = d.normalizeAksZoneToKompox(*z)
			}
		}
		if !equalStringSlices(updateZones, existingZones) {
			errs = append(errs, "Zones are immutable")
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("validation error: cannot modify immutable fields: %s", strings.Join(errs, ", "))
	}

	return nil
}

// mergeMutableFields merges mutable fields from update into existing agent pool.
func (d *driver) mergeMutableFields(update model.NodePool, existing *armcontainerservice.AgentPool) armcontainerservice.AgentPool {
	// Start with existing as base
	merged := *existing
	if merged.Properties == nil {
		merged.Properties = &armcontainerservice.ManagedClusterAgentPoolProfileProperties{}
	}

	props := merged.Properties

	// Update labels (mutable)
	if update.Labels != nil {
		labels := make(map[string]*string)
		// Start with existing labels
		if props.NodeLabels != nil {
			for k, v := range props.NodeLabels {
				labels[k] = v
			}
		}
		// Merge with update
		for k, v := range *update.Labels {
			labels[k] = to.Ptr(v)
		}
		// Ensure Kompox labels are present
		if update.Name != nil {
			labels[labelNodePool] = to.Ptr(*update.Name)
		}
		if update.Zones != nil && len(*update.Zones) > 0 {
			for _, z := range *update.Zones {
				labels[labelNodeZone] = to.Ptr(z)
				break
			}
		}
		props.NodeLabels = labels
	}

	// Update autoscaling (mutable)
	if update.Autoscaling != nil {
		props.EnableAutoScaling = to.Ptr(update.Autoscaling.Enabled)
		if update.Autoscaling.Enabled {
			props.MinCount = to.Ptr(int32(update.Autoscaling.Min))
			props.MaxCount = to.Ptr(int32(update.Autoscaling.Max))
		} else if update.Autoscaling.Desired != nil {
			props.Count = to.Ptr(int32(*update.Autoscaling.Desired))
		}
	}

	return merged
}

// normalizeAksZoneToKompox converts AKS zone format to Kompox format.
// AKS uses "1", "2", "3" while Kompox prefers "region-1", "region-2", etc.
func (d *driver) normalizeAksZoneToKompox(aksZone string) string {
	// If it's already in Kompox format (contains dash), return as-is
	if strings.Contains(aksZone, "-") {
		return aksZone
	}
	// Convert "1" to "region-1"
	return fmt.Sprintf("%s-%s", d.AzureLocation, aksZone)
}

// normalizeKompoxZoneToAks converts Kompox zone format to AKS format.
// Kompox uses "region-1", "region-2" while AKS expects "1", "2", "3".
func (d *driver) normalizeKompoxZoneToAks(kompoxZone string) string {
	// If it contains region prefix, strip it
	parts := strings.Split(kompoxZone, "-")
	if len(parts) > 1 {
		// Return the last part (zone index)
		return parts[len(parts)-1]
	}
	// Already in AKS format
	return kompoxZone
}

// isNotFoundError checks if an error is a 404 Not Found error.
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	// Check for Azure SDK error response
	var respErr *azcore.ResponseError
	if errors.As(err, &respErr) {
		return respErr.StatusCode == 404
	}
	return false
}

// equalStringSlices compares two string slices for equality (order matters).
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
