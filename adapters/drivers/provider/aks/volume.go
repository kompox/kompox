package aks

import (
	"context"
	"fmt"
	"maps"
	"sort"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
	"github.com/kompox/kompox/domain/model"
	"github.com/kompox/kompox/internal/naming"
)

// Built-in role definition IDs used by this driver
const (
	// Contributor role definition GUID (tenant-agnostic)
	contributorRoleDefinitionID = "b24988ac-6180-42a0-ab88-20f7382dd24c"
)

// newVolumeDisk creates a model.VolumeDisk from an Azure Disk resource.
// Returns an error if the disk lacks required tags or metadata.
func newVolumeDisk(disk *armcompute.Disk, volName string) (*model.VolumeDisk, error) {
	if disk == nil || disk.Name == nil || disk.Properties == nil {
		return nil, fmt.Errorf("disk is nil or missing required fields")
	}

	tags := disk.Tags
	if tags == nil {
		return nil, fmt.Errorf("disk missing tags")
	}

	// Extract disk name from tags
	diskName := ""
	if v, ok := tags[tagDiskName]; !ok || v == nil || *v == "" {
		return nil, fmt.Errorf("disk missing required tag: %s", tagDiskName)
	} else {
		diskName = *v
	}

	// Extract size
	var size int32
	if disk.Properties.DiskSizeGB != nil {
		size = *disk.Properties.DiskSizeGB
	}

	// Extract assigned status
	assigned := false
	if v, ok := tags[tagDiskAssigned]; ok && v != nil && strings.EqualFold(*v, "true") {
		assigned = true
	}

	// Extract creation time
	var created time.Time
	if disk.Properties.TimeCreated != nil {
		created = *disk.Properties.TimeCreated
	}

	// Extract zone
	zone := ""
	if len(disk.Zones) > 0 && disk.Zones[0] != nil {
		zone = *disk.Zones[0]
	}

	return &model.VolumeDisk{
		Name:       diskName,
		VolumeName: volName,
		Assigned:   assigned,
		Size:       int64(size) << 30,
		Zone:       zone,
		Options:    azureDiskOptions(disk),
		Handle:     *disk.ID,
		CreatedAt:  created,
		UpdatedAt:  created,
	}, nil
}

// newVolumeSnapshot creates a model.VolumeSnapshot from an Azure Snapshot resource.
// Returns an error if the snapshot lacks required tags or metadata.
func newVolumeSnapshot(snap *armcompute.Snapshot, volName string) (*model.VolumeSnapshot, error) {
	if snap == nil || snap.Name == nil || snap.Properties == nil {
		return nil, fmt.Errorf("snapshot is nil or missing required fields")
	}

	tags := snap.Tags
	if tags == nil {
		return nil, fmt.Errorf("snapshot missing tags")
	}

	// Extract snapshot name from tags
	snapName := ""
	if v, ok := tags[tagSnapshotName]; !ok || v == nil || *v == "" {
		return nil, fmt.Errorf("snapshot missing required tag: %s", tagSnapshotName)
	} else {
		snapName = *v
	}

	// Extract size and creation time
	var size int32
	var created time.Time
	if snap.Properties.DiskSizeGB != nil {
		size = *snap.Properties.DiskSizeGB
	}
	if snap.Properties.TimeCreated != nil {
		created = *snap.Properties.TimeCreated
	}

	return &model.VolumeSnapshot{
		Name:       snapName,
		VolumeName: volName,
		Size:       int64(size) << 30,
		Handle:     *snap.ID,
		CreatedAt:  created,
		UpdatedAt:  created,
	}, nil
}

// azureZones creates zones array from app deployment zone setting.
// Returns nil if zone is empty (regional disk), or []*string with zone value if zone is specified.
func azureZones(zone string) []*string {
	zone = strings.TrimSpace(zone)
	if zone == "" {
		return nil // Regional disk
	}
	return []*string{to.Ptr(zone)}
}

// setAzureDiskOptions applies Azure Disk SKU and performance options from volume options.
// Default SKU is Premium_LRS if not specified.
func setAzureDiskOptions(disk *armcompute.Disk, options map[string]any) {
	// Initialize SKU with default
	if disk.SKU == nil {
		disk.SKU = &armcompute.DiskSKU{}
	}
	disk.SKU.Name = to.Ptr(armcompute.DiskStorageAccountTypesPremiumLRS) // default

	if options == nil {
		return
	}

	// Apply SKU if specified
	if skuValue, ok := options["sku"].(string); ok && skuValue != "" {
		switch skuValue {
		case "Standard_LRS":
			disk.SKU.Name = to.Ptr(armcompute.DiskStorageAccountTypesStandardLRS)
		case "Premium_LRS":
			disk.SKU.Name = to.Ptr(armcompute.DiskStorageAccountTypesPremiumLRS)
		case "StandardSSD_LRS":
			disk.SKU.Name = to.Ptr(armcompute.DiskStorageAccountTypesStandardSSDLRS)
		case "UltraSSD_LRS":
			disk.SKU.Name = to.Ptr(armcompute.DiskStorageAccountTypesUltraSSDLRS)
		case "Premium_ZRS":
			disk.SKU.Name = to.Ptr(armcompute.DiskStorageAccountTypesPremiumZRS)
		case "StandardSSD_ZRS":
			disk.SKU.Name = to.Ptr(armcompute.DiskStorageAccountTypesStandardSSDZRS)
		case "PremiumV2_LRS":
			disk.SKU.Name = to.Ptr(armcompute.DiskStorageAccountTypesPremiumV2LRS)
		}
	}

	// Apply performance options if specified
	if disk.Properties == nil {
		return
	}

	// Set IOPS if specified
	if iopsValue, ok := options["iops"]; ok {
		switch v := iopsValue.(type) {
		case int:
			disk.Properties.DiskIOPSReadWrite = to.Ptr(int64(v))
		case int64:
			disk.Properties.DiskIOPSReadWrite = to.Ptr(v)
		case float64:
			disk.Properties.DiskIOPSReadWrite = to.Ptr(int64(v))
		}
	}

	// Set MBps if specified
	if mbpsValue, ok := options["mbps"]; ok {
		switch v := mbpsValue.(type) {
		case int:
			disk.Properties.DiskMBpsReadWrite = to.Ptr(int64(v))
		case int64:
			disk.Properties.DiskMBpsReadWrite = to.Ptr(v)
		case float64:
			disk.Properties.DiskMBpsReadWrite = to.Ptr(int64(v))
		}
	}
}

// azureDiskOptions extracts Azure Disk SKU and performance options into an options map.
func azureDiskOptions(disk *armcompute.Disk) map[string]any {
	options := make(map[string]any)
	// Extract SKU
	if disk.SKU != nil && disk.SKU.Name != nil {
		skuName := string(*disk.SKU.Name)
		options["sku"] = skuName
	}
	// Extract performance options
	if disk.Properties != nil {
		if disk.Properties.DiskIOPSReadWrite != nil {
			options["iops"] = *disk.Properties.DiskIOPSReadWrite
		}
		if disk.Properties.DiskMBpsReadWrite != nil {
			options["mbps"] = *disk.Properties.DiskMBpsReadWrite
		}
	}
	return options
}

// VolumeDiskList implements spec method.
func (d *driver) VolumeDiskList(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, _ ...model.VolumeDiskListOption) ([]*model.VolumeDisk, error) {
	if cluster == nil || app == nil {
		return nil, fmt.Errorf("cluster/app nil")
	}
	rg, err := d.appResourceGroupName(app)
	if err != nil {
		return nil, fmt.Errorf("app RG: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()
	disksClient, err := armcompute.NewDisksClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return nil, fmt.Errorf("new disks client: %w", err)
	}
	pager := disksClient.NewListByResourceGroupPager(rg, nil)
	out := []*model.VolumeDisk{}
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "could not be found") { // RG not found
				return []*model.VolumeDisk{}, nil
			}
			return nil, fmt.Errorf("list disks page: %w", err)
		}
		for _, disk := range page.Value {
			if disk == nil || disk.Name == nil || disk.Properties == nil {
				continue
			}
			tags := disk.Tags
			if tags == nil {
				continue
			}
			if v, ok := tags[tagVolumeName]; !ok || v == nil || *v != volName {
				continue
			}

			volumeDisk, err := newVolumeDisk(disk, volName)
			if err != nil {
				continue
			}

			out = append(out, volumeDisk)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

// VolumeDiskCreate implements spec method.
func (d *driver) VolumeDiskCreate(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, opts ...model.VolumeDiskCreateOption) (*model.VolumeDisk, error) {
	if cluster == nil || app == nil {
		return nil, fmt.Errorf("cluster/app nil")
	}
	// Process options
	options := &model.VolumeDiskCreateOptions{}
	for _, opt := range opts {
		opt(options)
	}
	// Find volume in app configuration
	vol, err := app.FindVolume(volName)
	if err != nil {
		return nil, fmt.Errorf("find volume: %w", err)
	}
	// Validate size and round up to GB
	if vol.Size < 1 {
		return nil, fmt.Errorf("volume size must be >0")
	}
	sizeGB := int32((vol.Size + (1 << 30) - 1) >> 30)
	// Get volume options from app configuration and merge with passed options
	volOptions := maps.Clone(vol.Options)
	maps.Copy(volOptions, options.Options)
	// Determine zone (options override app config)
	zone := app.Deployment.Zone
	if options.Zone != "" {
		zone = options.Zone
	}
	// Retrieve AKS principal ID from deployment outputs (per-cluster)
	outputs, err := d.azureDeploymentOutputs(ctx, cluster)
	if err != nil {
		return nil, fmt.Errorf("get deployment outputs: %w", err)
	}
	principalID, _ := outputs[outputAksPrincipalID].(string)

	// Inline azureVolumeDiskCreate logic
	rg, err := d.appResourceGroupName(app)
	if err != nil {
		return nil, fmt.Errorf("app RG: %w", err)
	}
	if sizeGB < 1 {
		return nil, fmt.Errorf("sizeGB must be >=1")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if err := d.ensureAzureResourceGroupCreated(ctx, rg, d.appResourceTags(app.Name), principalID); err != nil {
		return nil, fmt.Errorf("ensure RG: %w", err)
	}
	disksClient, err := armcompute.NewDisksClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return nil, fmt.Errorf("new disks client: %w", err)
	}
	diskName, err := naming.NewCompactID()
	if err != nil {
		return nil, fmt.Errorf("compact id: %w", err)
	}
	diskResourceName, err := d.appDiskName(app, volName, diskName)
	if err != nil {
		return nil, fmt.Errorf("generate disk resource name: %w", err)
	}
	// If this is the first volume disk for the logical volume, mark it assignedValue=true.
	assignedValue := "false"
	if items, err := d.VolumeDiskList(ctx, cluster, app, volName); err == nil {
		if len(items) == 0 {
			assignedValue = "true"
		}
	}
	// Base tags from app identity
	tags := d.appResourceTags(app.Name)
	// Add volume/disk specific tags
	tags[tagVolumeName] = to.Ptr(volName)
	tags[tagDiskName] = to.Ptr(diskName)
	tags[tagDiskAssigned] = to.Ptr(assignedValue)

	// Determine the creation option and source based on the Source field
	var creationData *armcompute.CreationData
	if options.Source == "" {
		// Empty disk (default behavior)
		creationData = &armcompute.CreationData{CreateOption: to.Ptr(armcompute.DiskCreateOptionEmpty)}
	} else {
		// Interpret the source string to determine creation method
		sourceID, err := d.resolveSourceID(ctx, cluster, app, volName, options.Source)
		if err != nil {
			return nil, fmt.Errorf("resolve source %q: %w", options.Source, err)
		}
		creationData = &armcompute.CreationData{
			CreateOption:     to.Ptr(armcompute.DiskCreateOptionCopy),
			SourceResourceID: to.Ptr(sourceID),
		}
	}

	disk := armcompute.Disk{
		Location: to.Ptr(d.AzureLocation),
		Zones:    azureZones(zone),
		Tags:     tags,
		Properties: &armcompute.DiskProperties{
			CreationData: creationData,
			DiskSizeGB:   to.Ptr(sizeGB),
		},
	}
	setAzureDiskOptions(&disk, volOptions)
	poller, err := disksClient.BeginCreateOrUpdate(ctx, rg, diskResourceName, disk, nil)
	if err != nil {
		return nil, fmt.Errorf("begin create disk: %w", err)
	}
	res, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("create disk: %w", err)
	}

	volumeDisk, err := newVolumeDisk(&res.Disk, volName)
	if err != nil {
		return nil, fmt.Errorf("failed to create VolumeDisk from created disk: %w", err)
	}

	return volumeDisk, nil
}

// VolumeDiskAssign implements spec method.
func (d *driver) VolumeDiskAssign(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, diskName string, _ ...model.VolumeDiskAssignOption) error {
	if cluster == nil || app == nil {
		return fmt.Errorf("cluster/app nil")
	}
	rg, err := d.appResourceGroupName(app)
	if err != nil {
		return fmt.Errorf("app RG: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()
	disksClient, err := armcompute.NewDisksClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return fmt.Errorf("new disks client: %w", err)
	}
	disks, err := d.VolumeDiskList(ctx, cluster, app, volName)
	if err != nil {
		return fmt.Errorf("list disks: %w", err)
	}
	var found bool
	for _, item := range disks {
		if item.Name == diskName {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("volume disk not found: %s", diskName)
	}
	for _, disk := range disks {
		assigned := false
		if disk.Name == diskName {
			assigned = true
		}
		if assigned == disk.Assigned {
			continue
		}

		// Get Azure disk resource name from the disk name
		diskResourceName, err := d.appDiskName(app, volName, disk.Name)
		if err != nil {
			return fmt.Errorf("generate disk resource name: %w", err)
		}

		// Get current disk tags
		diskRes, err := disksClient.Get(ctx, rg, diskResourceName, nil)
		if err != nil {
			return fmt.Errorf("get disk %s: %w", diskResourceName, err)
		}

		tags := diskRes.Disk.Tags
		if tags == nil {
			tags = map[string]*string{}
		}
		if assigned {
			tags[tagDiskAssigned] = to.Ptr("true")
		} else {
			tags[tagDiskAssigned] = to.Ptr("false")
		}
		update := armcompute.DiskUpdate{Tags: tags}
		poller, err := disksClient.BeginUpdate(ctx, rg, diskResourceName, update, nil)
		if err != nil {
			return fmt.Errorf("update disk %s: %w", diskResourceName, err)
		}
		if _, err = poller.PollUntilDone(ctx, nil); err != nil {
			return fmt.Errorf("poll update disk %s: %w", diskResourceName, err)
		}
	}
	return nil
}

// VolumeDiskDelete implements spec method.
func (d *driver) VolumeDiskDelete(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, diskName string, _ ...model.VolumeDiskDeleteOption) error {
	if cluster == nil || app == nil {
		return fmt.Errorf("cluster/app nil")
	}
	rg, err := d.appResourceGroupName(app)
	if err != nil {
		return fmt.Errorf("app RG: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	disksClient, err := armcompute.NewDisksClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return fmt.Errorf("new disks client: %w", err)
	}
	diskResourceName, err := d.appDiskName(app, volName, diskName)
	if err != nil {
		return fmt.Errorf("generate disk resource name: %w", err)
	}
	poller, err := disksClient.BeginDelete(ctx, rg, diskResourceName, nil)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "notfound") || strings.Contains(strings.ToLower(err.Error()), "could not be found") {
			return nil
		}
		return fmt.Errorf("begin delete disk: %w", err)
	}
	if _, err = poller.PollUntilDone(ctx, nil); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "notfound") {
			return nil
		}
		return fmt.Errorf("delete disk: %w", err)
	}
	return nil
}

func (d *driver) VolumeSnapshotList(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, _ ...model.VolumeSnapshotListOption) ([]*model.VolumeSnapshot, error) {
	if cluster == nil || app == nil {
		return nil, fmt.Errorf("cluster/app nil")
	}
	rg, err := d.appResourceGroupName(app)
	if err != nil {
		return nil, fmt.Errorf("app RG: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()

	snapsClient, err := armcompute.NewSnapshotsClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return nil, fmt.Errorf("new snapshots client: %w", err)
	}
	pager := snapsClient.NewListByResourceGroupPager(rg, nil)
	out := []*model.VolumeSnapshot{}
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "could not be found") { // RG not found
				return []*model.VolumeSnapshot{}, nil
			}
			return nil, fmt.Errorf("list snapshots page: %w", err)
		}
		for _, snap := range page.Value {
			if snap == nil || snap.Name == nil || snap.Properties == nil {
				continue
			}
			tags := snap.Tags
			if tags == nil {
				continue
			}
			if v, ok := tags[tagVolumeName]; !ok || v == nil || *v != volName {
				continue
			}

			volumeSnapshot, err := newVolumeSnapshot(snap, volName)
			if err != nil {
				continue
			}

			out = append(out, volumeSnapshot)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (d *driver) VolumeSnapshotCreate(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, diskName string, _ ...model.VolumeSnapshotCreateOption) (*model.VolumeSnapshot, error) {
	if cluster == nil || app == nil {
		return nil, fmt.Errorf("cluster/app nil")
	}
	rg, err := d.appResourceGroupName(app)
	if err != nil {
		return nil, fmt.Errorf("app RG: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	snapsClient, err := armcompute.NewSnapshotsClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return nil, fmt.Errorf("new snapshots client: %w", err)
	}
	diskResourceName, err := d.appDiskName(app, volName, diskName)
	if err != nil {
		return nil, fmt.Errorf("generate disk resource name: %w", err)
	}
	// Resolve source disk resource ID
	disksClient, err := armcompute.NewDisksClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return nil, fmt.Errorf("new disks client: %w", err)
	}
	diskRes, err := disksClient.Get(ctx, rg, diskResourceName, nil)
	if err != nil {
		return nil, fmt.Errorf("get disk: %w", err)
	}
	snapName, err := naming.NewCompactID()
	if err != nil {
		return nil, fmt.Errorf("compact id: %w", err)
	}
	snapResourceName, err := d.appSnapshotName(app, volName, snapName)
	if err != nil {
		return nil, fmt.Errorf("generate snapshot resource name: %w", err)
	}
	// Base tags from app identity
	tags := d.appResourceTags(app.Name)
	// Add volume/snapshot specific tags
	tags[tagVolumeName] = to.Ptr(volName)
	tags[tagDiskName] = to.Ptr(diskName)
	tags[tagSnapshotName] = to.Ptr(snapName)

	snap := armcompute.Snapshot{
		Location: to.Ptr(d.AzureLocation),
		Tags:     tags,
		Properties: &armcompute.SnapshotProperties{
			CreationData: &armcompute.CreationData{
				CreateOption:     to.Ptr(armcompute.DiskCreateOptionCopy),
				SourceResourceID: diskRes.ID,
			},
			Incremental: to.Ptr(true),
		},
		SKU: &armcompute.SnapshotSKU{Name: to.Ptr(armcompute.SnapshotStorageAccountTypesStandardZRS)},
	}

	poller, err := snapsClient.BeginCreateOrUpdate(ctx, rg, snapResourceName, snap, nil)
	if err != nil {
		return nil, fmt.Errorf("begin create snapshot: %w", err)
	}
	res, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("create snapshot: %w", err)
	}

	volumeSnapshot, err := newVolumeSnapshot(&res.Snapshot, volName)
	if err != nil {
		return nil, fmt.Errorf("failed to create VolumeSnapshot from created snapshot: %w", err)
	}

	return volumeSnapshot, nil
}

func (d *driver) VolumeSnapshotDelete(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, snapName string, _ ...model.VolumeSnapshotDeleteOption) error {
	if cluster == nil || app == nil {
		return fmt.Errorf("cluster/app nil")
	}
	rg, err := d.appResourceGroupName(app)
	if err != nil {
		return fmt.Errorf("app RG: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	snapsClient, err := armcompute.NewSnapshotsClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return fmt.Errorf("new snapshots client: %w", err)
	}
	snapResourceName, err := d.appSnapshotName(app, volName, snapName)
	if err != nil {
		return fmt.Errorf("generate snapshot resource name: %w", err)
	}
	poller, err := snapsClient.BeginDelete(ctx, rg, snapResourceName, nil)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "notfound") || strings.Contains(strings.ToLower(err.Error()), "could not be found") {
			return nil
		}
		return fmt.Errorf("begin delete snapshot: %w", err)
	}
	if _, err = poller.PollUntilDone(ctx, nil); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "notfound") {
			return nil
		}
		return fmt.Errorf("delete snapshot: %w", err)
	}
	return nil
}

func (d *driver) VolumeSnapshotRestore(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, snapName string, opts ...model.VolumeSnapshotRestoreOption) (*model.VolumeDisk, error) {
	if cluster == nil || app == nil {
		return nil, fmt.Errorf("cluster/app nil")
	}
	// Process options
	options := &model.VolumeSnapshotRestoreOptions{}
	for _, opt := range opts {
		opt(options)
	}
	// Find volume in app configuration
	vol, err := app.FindVolume(volName)
	if err != nil {
		return nil, fmt.Errorf("find volume: %w", err)
	}
	// Get volume options from app configuration and merge with passed options
	volOptions := maps.Clone(vol.Options)
	maps.Copy(volOptions, options.Options)
	// Determine zone (options override app config)
	zone := app.Deployment.Zone
	if options.Zone != "" {
		zone = options.Zone
	}
	// Retrieve AKS principal ID from deployment outputs (per-cluster)
	outputs, err := d.azureDeploymentOutputs(ctx, cluster)
	if err != nil {
		return nil, fmt.Errorf("get deployment outputs: %w", err)
	}
	principalID, _ := outputs[outputAksPrincipalID].(string)

	// Inline azureVolumeSnapshotRestore logic
	rg, err := d.appResourceGroupName(app)
	if err != nil {
		return nil, fmt.Errorf("app RG: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	if err := d.ensureAzureResourceGroupCreated(ctx, rg, d.appResourceTags(app.Name), principalID); err != nil {
		return nil, fmt.Errorf("ensure RG: %w", err)
	}
	snapsClient, err := armcompute.NewSnapshotsClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return nil, fmt.Errorf("new snapshots client: %w", err)
	}
	snapResourceName, err := d.appSnapshotName(app, volName, snapName)
	if err != nil {
		return nil, fmt.Errorf("generate snapshot resource name: %w", err)
	}
	snapRes, err := snapsClient.Get(ctx, rg, snapResourceName, nil)
	if err != nil {
		return nil, fmt.Errorf("get snapshot: %w", err)
	}
	disksClient, err := armcompute.NewDisksClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return nil, fmt.Errorf("new disks client: %w", err)
	}
	diskName, err := naming.NewCompactID()
	if err != nil {
		return nil, fmt.Errorf("compact id: %w", err)
	}
	diskResourceName, err := d.appDiskName(app, volName, diskName)
	if err != nil {
		return nil, fmt.Errorf("generate disk resource name: %w", err)
	}
	// Base tags from app identity
	tags := d.appResourceTags(app.Name)
	// Add volume/disk specific tags
	tags[tagVolumeName] = to.Ptr(volName)
	tags[tagDiskName] = to.Ptr(diskName)
	tags[tagDiskAssigned] = to.Ptr("false")

	disk := armcompute.Disk{
		Location: to.Ptr(d.AzureLocation),
		Zones:    azureZones(zone),
		Tags:     tags,
		Properties: &armcompute.DiskProperties{
			CreationData: &armcompute.CreationData{
				CreateOption:     to.Ptr(armcompute.DiskCreateOptionCopy),
				SourceResourceID: snapRes.ID,
			},
		},
	}
	setAzureDiskOptions(&disk, volOptions)
	poller, err := disksClient.BeginCreateOrUpdate(ctx, rg, diskResourceName, disk, nil)
	if err != nil {
		return nil, fmt.Errorf("begin create disk from snapshot: %w", err)
	}
	res, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("create disk from snapshot: %w", err)
	}

	volumeDisk, err := newVolumeDisk(&res.Disk, volName)
	if err != nil {
		return nil, fmt.Errorf("failed to create VolumeDisk from restored disk: %w", err)
	}

	return volumeDisk, nil
}

// resolveSourceID resolves a source string to an Azure resource ID.
// Supports:
// - Empty string -> empty disk creation (handled by caller)
// - "snapshot:name" or "name" (without prefix) -> Kompox managed snapshot
// - "disk:name" -> Kompox managed disk
// - "/subscriptions/..." -> Azure resource ID (snapshot or disk)
// - "arm:/subscriptions/..." -> Azure resource ID with arm: prefix
// - "resourceId:/subscriptions/..." -> Azure resource ID with resourceId: prefix
func (d *driver) resolveSourceID(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, source string) (string, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return "", fmt.Errorf("source cannot be empty")
	}

	// Convert to lowercase once for case-insensitive comparisons
	lowerSource := strings.ToLower(source)

	// Handle explicit Azure resource IDs (with various prefixes)
	if strings.HasPrefix(lowerSource, "/subscriptions/") {
		// Direct Azure resource ID
		return source, nil
	}
	if strings.HasPrefix(lowerSource, "arm:/subscriptions/") {
		// Strip "arm:" prefix (case-insensitive)
		return source[4:], nil
	}
	if strings.HasPrefix(lowerSource, "resourceid:/subscriptions/") {
		// Strip "resourceId:" prefix (case-insensitive)
		return source[11:], nil
	}

	// Handle Kompox managed resources
	if strings.HasPrefix(lowerSource, "snapshot:") {
		// Explicit snapshot reference (case-insensitive)
		snapName := source[9:]
		return d.resolveKompoxSnapshotID(ctx, app, volName, snapName)
	}
	if strings.HasPrefix(lowerSource, "disk:") {
		// Explicit disk reference (case-insensitive)
		diskName := source[5:]
		return d.resolveKompoxDiskID(ctx, app, volName, diskName)
	}

	// Default: treat as snapshot name (snapshot: prefix is optional)
	return d.resolveKompoxSnapshotID(ctx, app, volName, source)
}

// resolveKompoxSnapshotID resolves a Kompox managed snapshot name to its Azure resource ID.
func (d *driver) resolveKompoxSnapshotID(ctx context.Context, app *model.App, volName string, snapName string) (string, error) {
	if snapName == "" {
		return "", fmt.Errorf("snapshot name cannot be empty")
	}

	rg, err := d.appResourceGroupName(app)
	if err != nil {
		return "", fmt.Errorf("app RG: %w", err)
	}

	snapResourceName, err := d.appSnapshotName(app, volName, snapName)
	if err != nil {
		return "", fmt.Errorf("generate snapshot resource name: %w", err)
	}

	snapsClient, err := armcompute.NewSnapshotsClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return "", fmt.Errorf("new snapshots client: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	snapRes, err := snapsClient.Get(ctx, rg, snapResourceName, nil)
	if err != nil {
		return "", fmt.Errorf("get snapshot %q: %w", snapName, err)
	}

	if snapRes.ID == nil {
		return "", fmt.Errorf("snapshot %q has no resource ID", snapName)
	}

	return *snapRes.ID, nil
}

// resolveKompoxDiskID resolves a Kompox managed disk name to its Azure resource ID.
func (d *driver) resolveKompoxDiskID(ctx context.Context, app *model.App, volName string, diskName string) (string, error) {
	if diskName == "" {
		return "", fmt.Errorf("disk name cannot be empty")
	}

	rg, err := d.appResourceGroupName(app)
	if err != nil {
		return "", fmt.Errorf("app RG: %w", err)
	}

	diskResourceName, err := d.appDiskName(app, volName, diskName)
	if err != nil {
		return "", fmt.Errorf("generate disk resource name: %w", err)
	}

	disksClient, err := armcompute.NewDisksClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return "", fmt.Errorf("new disks client: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	diskRes, err := disksClient.Get(ctx, rg, diskResourceName, nil)
	if err != nil {
		return "", fmt.Errorf("get disk %q: %w", diskName, err)
	}

	if diskRes.ID == nil {
		return "", fmt.Errorf("disk %q has no resource ID", diskName)
	}

	return *diskRes.ID, nil
}

// VolumeClass implements providerdrv.Driver VolumeClass method for AKS.
// Returns opinionated defaults suitable for Azure Disk CSI; callers must omit any empty field.
func (d *driver) VolumeClass(ctx context.Context, cluster *model.Cluster, app *model.App, vol model.AppVolume) (model.VolumeClass, error) {
	// For now we always return managed-csi / disk.csi.azure.com with ext4; future: derive from volume, size, perf tier.
	return model.VolumeClass{
		StorageClassName: "managed-csi",
		CSIDriver:        "disk.csi.azure.com",
		FSType:           "ext4",
		Attributes:       map[string]string{"fsType": "ext4"},
		AccessModes:      []string{"ReadWriteOnce"},
		ReclaimPolicy:    "Retain",
		VolumeMode:       "Filesystem",
	}, nil
}
