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
	"github.com/kompox/kompox/internal/logging"
	"github.com/kompox/kompox/internal/naming"
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
func (d *driver) VolumeDiskCreate(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, diskName string, source string, opts ...model.VolumeDiskCreateOption) (*model.VolumeDisk, error) {
	if cluster == nil || app == nil {
		return nil, fmt.Errorf("cluster/app nil")
	}

	log := logging.FromContext(ctx)

	// Process options
	options := &model.VolumeDiskCreateOptions{}
	for _, opt := range opts {
		opt(options)
	}
	diskName = strings.TrimSpace(diskName)
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
	var principalID string
	outputs, err := d.azureDeploymentOutputs(ctx, cluster)
	if err == nil {
		principalID = outputs[outputAksPrincipalID].(string)
	} else {
		log.Warn(ctx, "Failed to get deployment outputs", "error", azureShorterErrorString(err))
	}
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
	// Role assignment is skipped if principalID is empty
	err = d.ensureAzureResourceGroupCreated(ctx, rg, d.appResourceTags(app.Name), principalID)
	if err != nil {
		log.Warn(ctx, "Failed to ensure RG", "resource_group", rg, "principal_id", principalID, "error", azureShorterErrorString(err))
	}
	disksClient, err := armcompute.NewDisksClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return nil, fmt.Errorf("new disks client: %w", err)
	}
	items, err := d.VolumeDiskList(ctx, cluster, app, volName)
	if err != nil {
		return nil, fmt.Errorf("list disks: %w", err)
	}
	// Derive diskName if not provided and ensure uniqueness when specified.
	if diskName == "" {
		if diskName, err = naming.NewCompactID(); err != nil {
			return nil, fmt.Errorf("compact id: %w", err)
		}
	} else {
		for _, item := range items {
			if item.Name == diskName {
				return nil, fmt.Errorf("disk %q already exists", diskName)
			}
		}
	}
	diskResourceName, err := d.appDiskName(app, volName, diskName)
	if err != nil {
		return nil, fmt.Errorf("generate disk resource name: %w", err)
	}
	// If this is the first volume disk for the logical volume, mark it assignedValue=true.
	assignedValue := "false"
	if len(items) == 0 {
		assignedValue = "true"
	}
	// Base tags from app identity
	tags := d.appResourceTags(app.Name)
	// Add volume/disk specific tags
	tags[tagVolumeName] = to.Ptr(volName)
	tags[tagDiskName] = to.Ptr(diskName)
	tags[tagDiskAssigned] = to.Ptr(assignedValue)

	// Determine the creation option and source based on the Source field
	var creationData *armcompute.CreationData
	source = strings.TrimSpace(source)
	if source == "" {
		// Empty disk (default behavior)
		creationData = &armcompute.CreationData{CreateOption: to.Ptr(armcompute.DiskCreateOptionEmpty)}
	} else {
		// Interpret the source string to determine creation method
		sourceID, err := d.resolveSourceSnapshotResourceID(ctx, app, volName, source)
		if err != nil {
			return nil, fmt.Errorf("resolve source %q: %w", source, err)
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
