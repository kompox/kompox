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

// driverVolumeDisk implements driverVolume interface for Azure Managed Disks (Type="disk").
type driverVolumeDisk struct {
	driver *driver
}

func newDriverVolumeDisk(d *driver) driverVolume {
	return &driverVolumeDisk{driver: d}
}

// DiskList lists Azure Managed Disks for a volume (Type="disk").
func (vd *driverVolumeDisk) DiskList(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, opts ...model.VolumeDiskListOption) ([]*model.VolumeDisk, error) {
	rg, err := vd.driver.appResourceGroupName(app)
	if err != nil {
		return nil, fmt.Errorf("app RG: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()

	disksClient, err := armcompute.NewDisksClient(vd.driver.AzureSubscriptionId, vd.driver.TokenCredential, nil)
	if err != nil {
		return nil, fmt.Errorf("new disks client: %w", err)
	}

	pager := disksClient.NewListByResourceGroupPager(rg, nil)
	var out []*model.VolumeDisk
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list disks: %w", err)
		}
		for _, disk := range page.Value {
			volumeDisk, err := vd.newDisk(disk, volName)
			if err != nil {
				continue
			}
			if volumeDisk == nil {
				continue
			}

			out = append(out, volumeDisk)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

// DiskCreate creates a new Azure Managed Disk for a volume (Type="disk").
func (vd *driverVolumeDisk) DiskCreate(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, diskName string, source string, opts ...model.VolumeDiskCreateOption) (*model.VolumeDisk, error) {
	rg, err := vd.driver.appResourceGroupName(app)
	if err != nil {
		return nil, fmt.Errorf("app RG: %w", err)
	}

	// Build options from functional options
	var optionsStruct model.VolumeDiskCreateOptions
	for _, opt := range opts {
		opt(&optionsStruct)
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	disksClient, err := armcompute.NewDisksClient(vd.driver.AzureSubscriptionId, vd.driver.TokenCredential, nil)
	if err != nil {
		return nil, fmt.Errorf("new disks client: %w", err)
	}

	// List existing disks to determine name if needed
	items, err := vd.DiskList(ctx, cluster, app, volName)
	if err != nil {
		return nil, fmt.Errorf("list disks: %w", err)
	}

	diskName = strings.TrimSpace(diskName)
	if diskName == "" {
		diskName, err = naming.NewCompactID()
		if err != nil {
			return nil, fmt.Errorf("compact id: %w", err)
		}
	} else {
		for _, item := range items {
			if item.Name == diskName {
				return nil, fmt.Errorf("disk %q already exists", diskName)
			}
		}
	}

	diskResourceName, err := vd.driver.appDiskName(app, volName, diskName)
	if err != nil {
		return nil, fmt.Errorf("generate disk resource name: %w", err)
	}

	// Find the volume to determine size and deployment zone
	vol, err := app.FindVolume(volName)
	if err != nil {
		return nil, fmt.Errorf("find volume %q: %w", volName, err)
	}

	// Get size from volume configuration
	size := vol.Size
	sizeGB := int32(size >> 30) // Convert bytes to GB
	if sizeGB < 1 {
		sizeGB = 1
	}

	// Determine zone (options override app config)
	zone := app.Deployment.Zone
	if optionsStruct.Zone != "" {
		zone = optionsStruct.Zone
	}

	// Merge volume options with functional options
	volOptions := maps.Clone(vol.Options)
	if optionsStruct.Options != nil {
		maps.Copy(volOptions, optionsStruct.Options)
	}

	tags := vd.driver.appResourceTags(app.Name)
	tags[tagVolumeName] = to.Ptr(volName)
	tags[tagDiskName] = to.Ptr(diskName)
	tags[tagDiskAssigned] = to.Ptr("false")

	// Determine creation data based on source
	source = strings.TrimSpace(source)
	if source == "" {
		// Create empty disk
		disk := armcompute.Disk{
			Location: to.Ptr(vd.driver.AzureLocation),
			Zones:    vd.zones(zone),
			Tags:     tags,
			SKU:      &armcompute.DiskSKU{},
			Properties: &armcompute.DiskProperties{
				DiskSizeGB: to.Ptr(sizeGB),
				CreationData: &armcompute.CreationData{
					CreateOption: to.Ptr(armcompute.DiskCreateOptionEmpty),
				},
			},
		}
		vd.setDiskOptions(&disk, vol.Options)

		poller, err := disksClient.BeginCreateOrUpdate(ctx, rg, diskResourceName, disk, nil)
		if err != nil {
			return nil, fmt.Errorf("create disk: %w", err)
		}
		getResp, err := poller.PollUntilDone(ctx, nil)
		if err != nil {
			return nil, fmt.Errorf("poll disk: %w", err)
		}

		volumeDisk, err := vd.newDisk(&getResp.Disk, volName)
		if err != nil {
			return nil, fmt.Errorf("create VolumeDisk from disk: %w", err)
		}
		return volumeDisk, nil
	}

	// Resolve source (snapshot or disk)
	snapsClient, err := armcompute.NewSnapshotsClient(vd.driver.AzureSubscriptionId, vd.driver.TokenCredential, nil)
	if err != nil {
		return nil, fmt.Errorf("new snapshots client: %w", err)
	}

	// Try snapshot first
	snapResourceName, err := vd.driver.appSnapshotName(app, volName, source)
	if err == nil {
		snapResp, err := snapsClient.Get(ctx, rg, snapResourceName, nil)
		if err == nil {
			// Create disk from snapshot
			disk := armcompute.Disk{
				Location: to.Ptr(vd.driver.AzureLocation),
				Zones:    vd.zones(zone),
				Tags:     tags,
				SKU:      &armcompute.DiskSKU{},
				Properties: &armcompute.DiskProperties{
					DiskSizeGB: to.Ptr(sizeGB),
					CreationData: &armcompute.CreationData{
						CreateOption:     to.Ptr(armcompute.DiskCreateOptionCopy),
						SourceResourceID: snapResp.Snapshot.ID,
					},
				},
			}
			vd.setDiskOptions(&disk, vol.Options)

			poller, err := disksClient.BeginCreateOrUpdate(ctx, rg, diskResourceName, disk, nil)
			if err != nil {
				return nil, fmt.Errorf("create disk from snapshot: %w", err)
			}
			getResp, err := poller.PollUntilDone(ctx, nil)
			if err != nil {
				return nil, fmt.Errorf("poll disk: %w", err)
			}

			volumeDisk, err := vd.newDisk(&getResp.Disk, volName)
			if err != nil {
				return nil, fmt.Errorf("create VolumeDisk from disk: %w", err)
			}
			return volumeDisk, nil
		}
	}

	// Try source as disk name
	sourceDiskResourceName, err := vd.driver.appDiskName(app, volName, source)
	if err == nil {
		sourceDiskResp, err := disksClient.Get(ctx, rg, sourceDiskResourceName, nil)
		if err == nil {
			// Create disk from disk (copy)
			disk := armcompute.Disk{
				Location: to.Ptr(vd.driver.AzureLocation),
				Zones:    vd.zones(zone),
				Tags:     tags,
				SKU:      &armcompute.DiskSKU{},
				Properties: &armcompute.DiskProperties{
					DiskSizeGB: to.Ptr(sizeGB),
					CreationData: &armcompute.CreationData{
						CreateOption:     to.Ptr(armcompute.DiskCreateOptionCopy),
						SourceResourceID: sourceDiskResp.Disk.ID,
					},
				},
			}
			vd.setDiskOptions(&disk, vol.Options)

			poller, err := disksClient.BeginCreateOrUpdate(ctx, rg, diskResourceName, disk, nil)
			if err != nil {
				return nil, fmt.Errorf("create disk from disk: %w", err)
			}
			getResp, err := poller.PollUntilDone(ctx, nil)
			if err != nil {
				return nil, fmt.Errorf("poll disk: %w", err)
			}

			volumeDisk, err := vd.newDisk(&getResp.Disk, volName)
			if err != nil {
				return nil, fmt.Errorf("create VolumeDisk from disk: %w", err)
			}
			return volumeDisk, nil
		}
	}

	return nil, fmt.Errorf("source %q not found (neither snapshot nor disk)", source)
}

// DiskDelete deletes an Azure Managed Disk (Type="disk").
func (vd *driverVolumeDisk) DiskDelete(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, diskName string, opts ...model.VolumeDiskDeleteOption) error {
	rg, err := vd.driver.appResourceGroupName(app)
	if err != nil {
		return fmt.Errorf("app RG: %w", err)
	}

	diskResourceName, err := vd.driver.appDiskName(app, volName, diskName)
	if err != nil {
		return fmt.Errorf("generate disk resource name: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	disksClient, err := armcompute.NewDisksClient(vd.driver.AzureSubscriptionId, vd.driver.TokenCredential, nil)
	if err != nil {
		return fmt.Errorf("new disks client: %w", err)
	}

	poller, err := disksClient.BeginDelete(ctx, rg, diskResourceName, nil)
	if err != nil {
		return fmt.Errorf("delete disk: %w", err)
	}

	_, err = poller.PollUntilDone(ctx, nil)
	return err
}

// DiskAssign assigns or unassigns Azure Managed Disks (Type="disk") by updating tags.
func (vd *driverVolumeDisk) DiskAssign(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, diskName string, opts ...model.VolumeDiskAssignOption) error {
	rg, err := vd.driver.appResourceGroupName(app)
	if err != nil {
		return fmt.Errorf("app RG: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	disksClient, err := armcompute.NewDisksClient(vd.driver.AzureSubscriptionId, vd.driver.TokenCredential, nil)
	if err != nil {
		return fmt.Errorf("new disks client: %w", err)
	}

	disks, err := vd.DiskList(ctx, cluster, app, volName)
	if err != nil {
		return fmt.Errorf("list disks: %w", err)
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
		diskResourceName, err := vd.driver.appDiskName(app, volName, disk.Name)
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

// SnapshotList lists snapshots for an Azure Managed Disk (Type="disk").
func (vd *driverVolumeDisk) SnapshotList(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, opts ...model.VolumeSnapshotListOption) ([]*model.VolumeSnapshot, error) {
	rg, err := vd.driver.appResourceGroupName(app)
	if err != nil {
		return nil, fmt.Errorf("app RG: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()

	snapsClient, err := armcompute.NewSnapshotsClient(vd.driver.AzureSubscriptionId, vd.driver.TokenCredential, nil)
	if err != nil {
		return nil, fmt.Errorf("new snapshots client: %w", err)
	}
	pager := snapsClient.NewListByResourceGroupPager(rg, nil)

	var out []*model.VolumeSnapshot
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list snapshots: %w", err)
		}
		for _, snap := range page.Value {
			volumeSnapshot, err := vd.newSnapshot(snap, volName)
			if err != nil {
				continue
			}
			if volumeSnapshot == nil {
				continue
			}

			out = append(out, volumeSnapshot)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

// SnapshotCreate creates a snapshot of an Azure Managed Disk (Type="disk").
func (vd *driverVolumeDisk) SnapshotCreate(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, snapName string, source string, opts ...model.VolumeSnapshotCreateOption) (*model.VolumeSnapshot, error) {
	rg, err := vd.driver.appResourceGroupName(app)
	if err != nil {
		return nil, fmt.Errorf("app RG: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	snapsClient, err := armcompute.NewSnapshotsClient(vd.driver.AzureSubscriptionId, vd.driver.TokenCredential, nil)
	if err != nil {
		return nil, fmt.Errorf("new snapshots client: %w", err)
	}

	// List existing snapshots to determine name if needed
	items, err := vd.SnapshotList(ctx, cluster, app, volName)
	if err != nil {
		return nil, fmt.Errorf("list snapshots: %w", err)
	}

	snapName = strings.TrimSpace(snapName)
	if snapName == "" {
		snapName, err = naming.NewCompactID()
		if err != nil {
			return nil, fmt.Errorf("compact id: %w", err)
		}
	} else {
		for _, item := range items {
			if item.Name == snapName {
				return nil, fmt.Errorf("snapshot %q already exists", snapName)
			}
		}
	}

	snapResourceName, err := vd.driver.appSnapshotName(app, volName, snapName)
	if err != nil {
		return nil, fmt.Errorf("generate snapshot resource name: %w", err)
	}

	tags := vd.driver.appResourceTags(app.Name)
	tags[tagVolumeName] = to.Ptr(volName)
	tags[tagSnapshotName] = to.Ptr(snapName)

	// Determine source disk resource ID
	sourceID := ""
	source = strings.TrimSpace(source)
	if source == "" {
		// Use assigned disk
		disks, err := vd.DiskList(ctx, cluster, app, volName)
		if err != nil {
			return nil, fmt.Errorf("list disks: %w", err)
		}
		for _, d := range disks {
			if d.Assigned && d.VolumeName == volName {
				sourceID = d.Handle
				break
			}
		}
		if sourceID == "" {
			return nil, fmt.Errorf("no assigned disk found for volume %q", volName)
		}
	} else {
		sourceID, err = vd.driver.resolveSourceDiskResourceID(ctx, app, volName, source)
		if err != nil {
			return nil, fmt.Errorf("resolve source %q: %w", source, err)
		}
	}

	snapshot := armcompute.Snapshot{
		Location: to.Ptr(vd.driver.AzureLocation),
		Tags:     tags,
		Properties: &armcompute.SnapshotProperties{
			CreationData: &armcompute.CreationData{
				CreateOption:     to.Ptr(armcompute.DiskCreateOptionCopy),
				SourceResourceID: to.Ptr(sourceID),
			},
		},
	}

	poller, err := snapsClient.BeginCreateOrUpdate(ctx, rg, snapResourceName, snapshot, nil)
	if err != nil {
		return nil, fmt.Errorf("create snapshot: %w", err)
	}

	getResp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("poll snapshot: %w", err)
	}

	volumeSnapshot, err := vd.newSnapshot(&getResp.Snapshot, volName)
	if err != nil {
		return nil, fmt.Errorf("create VolumeSnapshot from snapshot: %w", err)
	}

	return volumeSnapshot, nil
}

// SnapshotDelete deletes a snapshot of an Azure Managed Disk (Type="disk").
func (vd *driverVolumeDisk) SnapshotDelete(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, snapName string, opts ...model.VolumeSnapshotDeleteOption) error {
	rg, err := vd.driver.appResourceGroupName(app)
	if err != nil {
		return fmt.Errorf("app RG: %w", err)
	}

	snapResourceName, err := vd.driver.appSnapshotName(app, volName, snapName)
	if err != nil {
		return fmt.Errorf("generate snapshot resource name: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	snapsClient, err := armcompute.NewSnapshotsClient(vd.driver.AzureSubscriptionId, vd.driver.TokenCredential, nil)
	if err != nil {
		return fmt.Errorf("new snapshots client: %w", err)
	}

	poller, err := snapsClient.BeginDelete(ctx, rg, snapResourceName, nil)
	if err != nil {
		return fmt.Errorf("delete snapshot: %w", err)
	}

	_, err = poller.PollUntilDone(ctx, nil)
	return err
}

// Class returns Azure Managed Disk provisioning parameters (Type="disk").
func (vd *driverVolumeDisk) Class(ctx context.Context, cluster *model.Cluster, app *model.App, vol model.AppVolume) (model.VolumeClass, error) {
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

// newDisk creates a model.VolumeDisk from an Azure Disk resource.
// Returns an error if the disk lacks required tags or metadata.
func (vd *driverVolumeDisk) newDisk(disk *armcompute.Disk, volName string) (*model.VolumeDisk, error) {
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
		Options:    vd.diskOptions(disk),
		Handle:     *disk.ID,
		CreatedAt:  created,
		UpdatedAt:  created,
	}, nil
}

// zones creates zones array from app deployment zone setting.
// Returns nil if zone is empty (regional disk), or []*string with zone value if zone is specified.
func (vd *driverVolumeDisk) zones(zone string) []*string {
	zone = strings.TrimSpace(zone)
	if zone == "" {
		return nil // Regional disk
	}
	return []*string{to.Ptr(zone)}
}

// setDiskOptions applies Azure Disk SKU and performance options from volume options.
// Default SKU is Premium_LRS if not specified.
func (vd *driverVolumeDisk) setDiskOptions(disk *armcompute.Disk, options map[string]any) {
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

// diskOptions extracts Azure Disk SKU and performance options into an options map.
func (vd *driverVolumeDisk) diskOptions(disk *armcompute.Disk) map[string]any {
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

// newSnapshot creates a model.VolumeSnapshot from an Azure Snapshot resource.
// Returns an error if the snapshot lacks required tags or metadata.
func (vd *driverVolumeDisk) newSnapshot(snap *armcompute.Snapshot, volName string) (*model.VolumeSnapshot, error) {
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
