package aks

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
	"github.com/yaegashi/kompoxops/domain/model"
	"github.com/yaegashi/kompoxops/internal/logging"
	"github.com/yaegashi/kompoxops/internal/naming"
)

// Disk tag keys
const (
	tagVolumeKey    = "kompox-volume"        // logical volume key value: kompox-volName-idHASH
	tagDiskName     = "kompox-disk-name"     // ulid
	tagDiskAssigned = "kompox-disk-assigned" // true/false
	tagSnapshotName = "kompox-snapshot-name" // ulid
)

// Built-in role definition IDs used by this driver
const (
	// Contributor role definition GUID (tenant-agnostic)
	contributorRoleDefinitionID = "b24988ac-6180-42a0-ab88-20f7382dd24c"
)

// volumeDiskMeta represents returned metadata for a volume disk.
// Internal structure; adapted to domain model when wiring into usecases.
type volumeDiskMeta struct {
	Name        string // Azure Disk name
	DiskID      string // ULID
	SizeGiB     int32
	Assigned    bool
	TimeCreated time.Time
}

// snapshotMeta represents returned metadata for a snapshot.
// Internal structure; adapted to domain model when wiring into usecases.
type snapshotMeta struct {
	Name        string // Azure Snapshot name
	SnapID      string // ULID
	SourceDisk  string // ULID (kompox disk id)
	SizeGiB     int32
	TimeCreated time.Time
}

// generateULID returns a monotonic ULID (time based) for disk naming.
func generateULID() (string, error) {
	entropy := ulid.Monotonic(rand.Reader, 0)
	id, err := ulid.New(ulid.Timestamp(time.Now().UTC()), entropy)
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

// helper to build disk name: kompox-volName-idHASH-volInstName
func buildDiskName(lvKey, ulid string) string { return fmt.Sprintf("%s-%s", lvKey, ulid) }

// helper to build snapshot name: kompox-volName-idHASH-ULID
func buildSnapshotName(lvKey, ulid string) string { return fmt.Sprintf("%s-%s", lvKey, ulid) }

// logicalVolumeKey returns kompox-volName-idHASH
func logicalVolumeKey(volName, idHASH string) string {
	return fmt.Sprintf("kompox-%s-%s", volName, idHASH)
}

// VolumeDiskList lists Azure Managed Disks for a logical volume (internal helper).
func (d *driver) azureVolumeDiskList(ctx context.Context, resourceGroupName, volName, idHASH string) ([]volumeDiskMeta, error) {
	if resourceGroupName == "" {
		return nil, errors.New("AZURE_RESOURCE_GROUP_NAME required")
	}
	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()

	disksClient, err := armcompute.NewDisksClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return nil, fmt.Errorf("new disks client: %w", err)
	}

	lvKey := logicalVolumeKey(volName, idHASH)
	prefix := lvKey + "-"
	pager := disksClient.NewListByResourceGroupPager(resourceGroupName, nil)
	out := []volumeDiskMeta{}
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "could not be found") { // RG not found
				return []volumeDiskMeta{}, nil
			}
			return nil, fmt.Errorf("list disks page: %w", err)
		}
		for _, disk := range page.Value {
			if disk == nil || disk.Name == nil || disk.Properties == nil {
				continue
			}
			name := *disk.Name
			if !strings.HasPrefix(name, prefix) {
				continue
			}
			// filter tag
			tags := disk.Tags
			if tags == nil {
				continue
			}
			if v, ok := tags[tagVolumeKey]; !ok || v == nil || *v != lvKey {
				continue
			}
			diskID := strings.TrimPrefix(name, prefix)
			size := int32(0)
			if disk.Properties.DiskSizeGB != nil {
				size = *disk.Properties.DiskSizeGB
			}
			assigned := false
			if v, ok := tags[tagDiskAssigned]; ok && v != nil && strings.EqualFold(*v, "true") {
				assigned = true
			}
			created := time.Time{}
			if disk.Properties.TimeCreated != nil {
				created = *disk.Properties.TimeCreated
			}
			out = append(out, volumeDiskMeta{Name: name, DiskID: diskID, SizeGiB: size, Assigned: assigned, TimeCreated: created})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TimeCreated.After(out[j].TimeCreated) })
	return out, nil
}

// ensureResourceGroup ensures RG exists for Create path only.
func (d *driver) ensureResourceGroup(ctx context.Context, resourceGroupName string, principalID string) error {
	log := logging.FromContext(ctx)

	// Ensure RG exists
	groupsClient, err := armresources.NewResourceGroupsClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return err
	}

	log.Info(ctx, "ensuring resource group", "resource_group", resourceGroupName, "subscription", d.AzureSubscriptionId, "location", d.AzureLocation)
	if _, err = groupsClient.CreateOrUpdate(ctx, resourceGroupName, armresources.ResourceGroup{Location: to.Ptr(d.AzureLocation)}, nil); err != nil {
		return err
	}

	// Ensure AKS principal has Contributor on this RG (idempotent)
	principalID = strings.TrimSpace(principalID)
	if principalID == "" {
		// Unknown principal; skip assignment silently (caller should have provided from deployment outputs).
		return nil
	}

	assignmentsClient, err := armauthorization.NewRoleAssignmentsClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return err
	}

	// Create assignment with deterministic GUID name derived from (scope, principalID, roleDefinitionID)
	scope := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s", d.AzureSubscriptionId, resourceGroupName)
	roleDefinitionID := fmt.Sprintf("/subscriptions/%s/providers/Microsoft.Authorization/roleDefinitions/%s", d.AzureSubscriptionId, contributorRoleDefinitionID)
	hashInput := scope + "|" + principalID + "|" + roleDefinitionID
	name := uuid.NewSHA1(uuid.NameSpaceURL, []byte(hashInput)).String()
	params := armauthorization.RoleAssignmentCreateParameters{
		Properties: &armauthorization.RoleAssignmentProperties{
			PrincipalID:      to.Ptr(principalID),
			RoleDefinitionID: to.Ptr(roleDefinitionID),
		},
	}

	log.Info(ctx, "ensuring role assignment", "scope", scope, "principal_id", principalID, "role_definition_id", contributorRoleDefinitionID, "assignment_name", name)
	if _, err := assignmentsClient.Create(ctx, scope, name, params, nil); err != nil {
		// If conflict due to existing assignment (race), treat as success
		if strings.Contains(strings.ToLower(err.Error()), "existing assignment") || strings.Contains(strings.ToLower(err.Error()), "conflict") {
			return nil
		}
		return err
	}

	return nil
}

// VolumeDiskCreate creates an Azure Disk for logical volume (internal helper).
func (d *driver) azureVolumeDiskCreate(ctx context.Context, resourceGroupName, volName, idHASH string, sizeGiB int32, principalID string) (*volumeDiskMeta, error) {
	if resourceGroupName == "" {
		return nil, errors.New("AZURE_RESOURCE_GROUP_NAME required")
	}
	if sizeGiB < 1 {
		return nil, fmt.Errorf("sizeGiB must be >=1")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	if err := d.ensureResourceGroup(ctx, resourceGroupName, principalID); err != nil {
		return nil, fmt.Errorf("ensure RG: %w", err)
	}

	disksClient, err := armcompute.NewDisksClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return nil, fmt.Errorf("new disks client: %w", err)
	}
	ulidStr, err := generateULID()
	if err != nil {
		return nil, fmt.Errorf("ulid: %w", err)
	}
	lvKey := logicalVolumeKey(volName, idHASH)
	diskName := buildDiskName(lvKey, ulidStr)

	// If this is the first volume disk for the logical volume, mark it assigned=true.
	initialAssigned := false
	if list, err := d.azureVolumeDiskList(ctx, resourceGroupName, volName, idHASH); err == nil {
		if len(list) == 0 {
			initialAssigned = true
		}
	} else {
		// If list fails, continue with default (not assigned) but log via error wrapping when creation fails later.
	}

	assignedVal := "false"
	if initialAssigned {
		assignedVal = "true"
	}

	disk := armcompute.Disk{
		Location: to.Ptr(d.AzureLocation),
		Tags: map[string]*string{
			tagVolumeKey:    to.Ptr(lvKey),
			tagDiskName:     to.Ptr(ulidStr),
			tagDiskAssigned: to.Ptr(assignedVal),
			"managed-by":    to.Ptr("kompox"),
		},
		Properties: &armcompute.DiskProperties{
			CreationData: &armcompute.CreationData{CreateOption: to.Ptr(armcompute.DiskCreateOptionEmpty)},
			DiskSizeGB:   to.Ptr(sizeGiB),
		},
		SKU: &armcompute.DiskSKU{Name: to.Ptr(armcompute.DiskStorageAccountTypesPremiumLRS)},
	}

	poller, err := disksClient.BeginCreateOrUpdate(ctx, resourceGroupName, diskName, disk, nil)
	if err != nil {
		return nil, fmt.Errorf("begin create disk: %w", err)
	}
	res, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("create disk: %w", err)
	}
	meta := &volumeDiskMeta{Name: diskName, DiskID: ulidStr, SizeGiB: sizeGiB, Assigned: assignedVal == "true", TimeCreated: time.Now().UTC()}
	if res.Properties != nil && res.Properties.TimeCreated != nil {
		meta.TimeCreated = *res.Properties.TimeCreated
	}
	return meta, nil
}

// VolumeDiskAssign marks one disk assigned=true and others false using tag update with concurrency control.
func (d *driver) azureVolumeDiskAssign(ctx context.Context, resourceGroupName, volName, idHASH, diskName string) error {
	if resourceGroupName == "" {
		return errors.New("AZURE_RESOURCE_GROUP_NAME required")
	}
	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()
	disksClient, err := armcompute.NewDisksClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return fmt.Errorf("new disks client: %w", err)
	}
	lvKey := logicalVolumeKey(volName, idHASH)
	list, err := d.azureVolumeDiskList(ctx, resourceGroupName, volName, idHASH)
	if err != nil {
		return err
	}
	var target *volumeDiskMeta
	for i := range list {
		if list[i].DiskID == diskName {
			target = &list[i]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("volume disk not found: %s", diskName)
	}

	for _, item := range list {
		getRes, err := disksClient.Get(ctx, resourceGroupName, item.Name, nil)
		if err != nil {
			return fmt.Errorf("get disk %s: %w", item.Name, err)
		}
		tags := map[string]*string{}
		for k, v := range getRes.Tags {
			tags[k] = v
		}
		tags[tagVolumeKey] = to.Ptr(lvKey)
		tags[tagDiskName] = to.Ptr(item.DiskID)
		assigned := strings.EqualFold(item.DiskID, diskName)
		// Idempotency check
		prevAssigned := false
		if v, ok := getRes.Tags[tagDiskAssigned]; ok && v != nil && strings.EqualFold(*v, "true") {
			prevAssigned = true
		}
		if assigned && prevAssigned {
			continue
		}
		val := "false"
		if assigned {
			val = "true"
		}
		tags[tagDiskAssigned] = to.Ptr(val)
		update := armcompute.DiskUpdate{Tags: tags}
		poller, err := disksClient.BeginUpdate(ctx, resourceGroupName, item.Name, update, nil)
		if err != nil {
			return fmt.Errorf("update disk %s: %w", item.Name, err)
		}
		if _, err = poller.PollUntilDone(ctx, nil); err != nil {
			return fmt.Errorf("poll update disk %s: %w", item.Name, err)
		}
	}
	return nil
}

// VolumeDiskDelete deletes a disk (idempotent).
func (d *driver) azureVolumeDiskDelete(ctx context.Context, resourceGroupName, volName, idHASH, diskID string) error {
	if resourceGroupName == "" {
		return errors.New("AZURE_RESOURCE_GROUP_NAME required")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	disksClient, err := armcompute.NewDisksClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return fmt.Errorf("new disks client: %w", err)
	}
	lvKey := logicalVolumeKey(volName, idHASH)
	fullDiskName := buildDiskName(lvKey, diskID)
	poller, err := disksClient.BeginDelete(ctx, resourceGroupName, fullDiskName, nil)
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

// azureVolumeSnapshotList lists snapshots belonging to a logical volume (by tag and name prefix).
func (d *driver) azureVolumeSnapshotList(ctx context.Context, resourceGroupName, volName, idHASH string) ([]snapshotMeta, error) {
	if resourceGroupName == "" {
		return nil, errors.New("AZURE_RESOURCE_GROUP_NAME required")
	}
	ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
	defer cancel()

	snapsClient, err := armcompute.NewSnapshotsClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return nil, fmt.Errorf("new snapshots client: %w", err)
	}
	lvKey := logicalVolumeKey(volName, idHASH)
	prefix := lvKey + "-"
	pager := snapsClient.NewListByResourceGroupPager(resourceGroupName, nil)
	out := []snapshotMeta{}
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "could not be found") { // RG not found
				return []snapshotMeta{}, nil
			}
			return nil, fmt.Errorf("list snapshots page: %w", err)
		}
		for _, snap := range page.Value {
			if snap == nil || snap.Name == nil || snap.Properties == nil {
				continue
			}
			name := *snap.Name
			if !strings.HasPrefix(name, prefix) {
				continue
			}
			tags := snap.Tags
			if tags == nil {
				continue
			}
			if v, ok := tags[tagVolumeKey]; !ok || v == nil || *v != lvKey {
				continue
			}
			snapID := strings.TrimPrefix(name, prefix)
			size := int32(0)
			if snap.Properties.DiskSizeGB != nil {
				size = *snap.Properties.DiskSizeGB
			}
			created := time.Time{}
			if snap.Properties.TimeCreated != nil {
				created = *snap.Properties.TimeCreated
			}
			// Source disk ULID is recorded in tagDiskName on the snapshot
			src := ""
			if v, ok := tags[tagDiskName]; ok && v != nil && *v != "" {
				src = *v
			}
			out = append(out, snapshotMeta{Name: name, SnapID: snapID, SourceDisk: src, SizeGiB: size, TimeCreated: created})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TimeCreated.After(out[j].TimeCreated) })
	return out, nil
}

// azureVolumeSnapshotCreate creates an Azure Managed Snapshot from a given disk in the same RG.
func (d *driver) azureVolumeSnapshotCreate(ctx context.Context, resourceGroupName, volName, idHASH, diskID string) (*snapshotMeta, error) {
	if resourceGroupName == "" {
		return nil, errors.New("AZURE_RESOURCE_GROUP_NAME required")
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	snapsClient, err := armcompute.NewSnapshotsClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return nil, fmt.Errorf("new snapshots client: %w", err)
	}
	lvKey := logicalVolumeKey(volName, idHASH)
	// Build disk and snapshot names
	fullDiskName := buildDiskName(lvKey, diskID)

	// Resolve source disk resource ID
	disksClient, err := armcompute.NewDisksClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return nil, fmt.Errorf("new disks client: %w", err)
	}
	diskRes, err := disksClient.Get(ctx, resourceGroupName, fullDiskName, nil)
	if err != nil {
		return nil, fmt.Errorf("get disk: %w", err)
	}
	sourceResID := ""
	if diskRes.ID != nil {
		sourceResID = *diskRes.ID
	} else {
		// Construct fallback ID
		sourceResID = fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/disks/%s", d.AzureSubscriptionId, resourceGroupName, fullDiskName)
	}

	ulidStr, err := generateULID()
	if err != nil {
		return nil, fmt.Errorf("ulid: %w", err)
	}
	snapName := buildSnapshotName(lvKey, ulidStr)

	snap := armcompute.Snapshot{
		Location: to.Ptr(d.AzureLocation),
		Tags: map[string]*string{
			tagVolumeKey:    to.Ptr(lvKey),
			tagSnapshotName: to.Ptr(ulidStr),
			// Use disk name tag to record the source disk ID (ULID)
			tagDiskName:  to.Ptr(diskID),
			"managed-by": to.Ptr("kompox"),
		},
		Properties: &armcompute.SnapshotProperties{
			CreationData: &armcompute.CreationData{
				CreateOption:     to.Ptr(armcompute.DiskCreateOptionCopy),
				SourceResourceID: to.Ptr(sourceResID),
			},
			Incremental: to.Ptr(true),
		},
		SKU: &armcompute.SnapshotSKU{Name: to.Ptr(armcompute.SnapshotStorageAccountTypesStandardLRS)},
	}

	poller, err := snapsClient.BeginCreateOrUpdate(ctx, resourceGroupName, snapName, snap, nil)
	if err != nil {
		return nil, fmt.Errorf("begin create snapshot: %w", err)
	}
	res, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("create snapshot: %w", err)
	}
	// derive size/time
	sizeGiB := int32(0)
	created := time.Now().UTC()
	if res.Properties != nil {
		if res.Properties.DiskSizeGB != nil {
			sizeGiB = *res.Properties.DiskSizeGB
		}
		if res.Properties.TimeCreated != nil {
			created = *res.Properties.TimeCreated
		}
	}
	return &snapshotMeta{Name: snapName, SnapID: ulidStr, SourceDisk: diskID, SizeGiB: sizeGiB, TimeCreated: created}, nil
}

// azureVolumeSnapshotDelete deletes a snapshot (idempotent).
func (d *driver) azureVolumeSnapshotDelete(ctx context.Context, resourceGroupName, volName, idHASH, snapID string) error {
	if resourceGroupName == "" {
		return errors.New("AZURE_RESOURCE_GROUP_NAME required")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	snapsClient, err := armcompute.NewSnapshotsClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return fmt.Errorf("new snapshots client: %w", err)
	}
	lvKey := logicalVolumeKey(volName, idHASH)
	fullSnapName := buildSnapshotName(lvKey, snapID)
	poller, err := snapsClient.BeginDelete(ctx, resourceGroupName, fullSnapName, nil)
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

// azureVolumeDiskCreateFromSnapshot creates a new disk from an existing snapshot.
func (d *driver) azureVolumeDiskCreateFromSnapshot(ctx context.Context, resourceGroupName, volName, idHASH, snapID string, principalID string) (*volumeDiskMeta, error) {
	if resourceGroupName == "" {
		return nil, errors.New("AZURE_RESOURCE_GROUP_NAME required")
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	if err := d.ensureResourceGroup(ctx, resourceGroupName, principalID); err != nil {
		return nil, fmt.Errorf("ensure RG: %w", err)
	}

	snapsClient, err := armcompute.NewSnapshotsClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return nil, fmt.Errorf("new snapshots client: %w", err)
	}
	lvKey := logicalVolumeKey(volName, idHASH)
	fullSnapName := buildSnapshotName(lvKey, snapID)
	snapRes, err := snapsClient.Get(ctx, resourceGroupName, fullSnapName, nil)
	if err != nil {
		return nil, fmt.Errorf("get snapshot: %w", err)
	}
	sourceResID := ""
	if snapRes.ID != nil {
		sourceResID = *snapRes.ID
	} else {
		sourceResID = fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/snapshots/%s", d.AzureSubscriptionId, resourceGroupName, fullSnapName)
	}

	disksClient, err := armcompute.NewDisksClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return nil, fmt.Errorf("new disks client: %w", err)
	}
	// Per contract, restored disks should always start as Assigned=false.
	assignedVal := "false"

	ulidStr, err := generateULID()
	if err != nil {
		return nil, fmt.Errorf("ulid: %w", err)
	}
	diskName := buildDiskName(lvKey, ulidStr)

	disk := armcompute.Disk{
		Location: to.Ptr(d.AzureLocation),
		Tags: map[string]*string{
			tagVolumeKey:    to.Ptr(lvKey),
			tagDiskName:     to.Ptr(ulidStr),
			tagDiskAssigned: to.Ptr(assignedVal),
			"managed-by":    to.Ptr("kompox"),
		},
		Properties: &armcompute.DiskProperties{
			CreationData: &armcompute.CreationData{
				CreateOption:     to.Ptr(armcompute.DiskCreateOptionCopy),
				SourceResourceID: to.Ptr(sourceResID),
			},
		},
		SKU: &armcompute.DiskSKU{Name: to.Ptr(armcompute.DiskStorageAccountTypesPremiumLRS)},
	}

	poller, err := disksClient.BeginCreateOrUpdate(ctx, resourceGroupName, diskName, disk, nil)
	if err != nil {
		return nil, fmt.Errorf("begin create disk from snapshot: %w", err)
	}
	res, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("create disk from snapshot: %w", err)
	}
	sizeGiB := int32(0)
	created := time.Now().UTC()
	if res.Properties != nil {
		if res.Properties.DiskSizeGB != nil {
			sizeGiB = *res.Properties.DiskSizeGB
		}
		if res.Properties.TimeCreated != nil {
			created = *res.Properties.TimeCreated
		}
	}
	meta := &volumeDiskMeta{Name: diskName, DiskID: ulidStr, SizeGiB: sizeGiB, Assigned: false, TimeCreated: created}
	return meta, nil
}

// volumeDefSize returns defined size for volume name from app.Volumes.
func volumeDefSize(app *model.App, volName string) (int64, bool) {
	for _, v := range app.Volumes {
		if v.Name == volName {
			return v.Size, true
		}
	}
	return 0, false
}

// VolumeDiskList implements spec method.
func (d *driver) VolumeDiskList(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, _ ...model.VolumeDiskListOption) ([]*model.VolumeDisk, error) {
	if cluster == nil || app == nil {
		return nil, fmt.Errorf("cluster/app nil")
	}
	rg, err := d.volumeResourceGroupName(app)
	if err != nil {
		return nil, err
	}
	hashes := naming.NewHashes(d.ServiceName(), d.ProviderName(), cluster.Name, app.Name)
	idHASH := hashes.AppID
	items, err := d.azureVolumeDiskList(ctx, rg, volName, idHASH)
	if err != nil {
		return nil, err
	}
	out := make([]*model.VolumeDisk, 0, len(items))
	for _, it := range items {
		out = append(out, &model.VolumeDisk{
			Name:       it.DiskID,
			VolumeName: volName,
			Assigned:   it.Assigned,
			Size:       int64(it.SizeGiB) << 30,
			Handle:     fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/disks/%s", d.AzureSubscriptionId, rg, it.Name),
			CreatedAt:  it.TimeCreated,
			UpdatedAt:  it.TimeCreated,
		})
	}
	return out, nil
}

// VolumeDiskCreate implements spec method.
func (d *driver) VolumeDiskCreate(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, _ ...model.VolumeDiskCreateOption) (*model.VolumeDisk, error) {
	if cluster == nil || app == nil {
		return nil, fmt.Errorf("cluster/app nil")
	}
	// Retrieve AKS principal ID from deployment outputs (per-cluster)
	outputs, err := d.getDeploymentOutputs(ctx, cluster)
	if err != nil {
		return nil, fmt.Errorf("get deployment outputs: %w", err)
	}
	principalID, _ := outputs[OutputAksPrincipalID].(string)
	rg, err := d.volumeResourceGroupName(app)
	if err != nil {
		return nil, err
	}
	sizeBytes, ok := volumeDefSize(app, volName)
	if !ok {
		return nil, fmt.Errorf("volume definition not found: %s", volName)
	}
	if sizeBytes < 1 {
		sizeBytes = 1 << 30
	}
	sizeGiB := int32((sizeBytes + (1 << 30) - 1) >> 30)
	hashes := naming.NewHashes(d.ServiceName(), d.ProviderName(), cluster.Name, app.Name)
	idHASH := hashes.AppID
	meta, err := d.azureVolumeDiskCreate(ctx, rg, volName, idHASH, sizeGiB, principalID)
	if err != nil {
		return nil, err
	}
	return &model.VolumeDisk{
		Name:       meta.DiskID,
		VolumeName: volName,
		Assigned:   meta.Assigned,
		Size:       int64(meta.SizeGiB) << 30,
		Handle:     fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/disks/%s", d.AzureSubscriptionId, rg, meta.Name),
		CreatedAt:  meta.TimeCreated,
		UpdatedAt:  meta.TimeCreated,
	}, nil
}

// VolumeDiskAssign implements spec method.
func (d *driver) VolumeDiskAssign(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, diskName string, _ ...model.VolumeDiskAssignOption) error {
	if cluster == nil || app == nil {
		return fmt.Errorf("cluster/app nil")
	}
	rg, err := d.volumeResourceGroupName(app)
	if err != nil {
		return err
	}
	hashes := naming.NewHashes(d.ServiceName(), d.ProviderName(), cluster.Name, app.Name)
	idHASH := hashes.AppID
	return d.azureVolumeDiskAssign(ctx, rg, volName, idHASH, diskName)
}

// VolumeDiskDelete implements spec method.
func (d *driver) VolumeDiskDelete(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, diskName string, _ ...model.VolumeDiskDeleteOption) error {
	if cluster == nil || app == nil {
		return fmt.Errorf("cluster/app nil")
	}
	rg, err := d.volumeResourceGroupName(app)
	if err != nil {
		return err
	}
	hashes := naming.NewHashes(d.ServiceName(), d.ProviderName(), cluster.Name, app.Name)
	idHASH := hashes.AppID
	return d.azureVolumeDiskDelete(ctx, rg, volName, idHASH, diskName)
}

// Snapshot operations for AKS
func (d *driver) VolumeSnapshotList(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, _ ...model.VolumeSnapshotListOption) ([]*model.VolumeSnapshot, error) {
	if cluster == nil || app == nil {
		return nil, fmt.Errorf("cluster/app nil")
	}
	rg, err := d.volumeResourceGroupName(app)
	if err != nil {
		return nil, err
	}
	hashes := naming.NewHashes(d.ServiceName(), d.ProviderName(), cluster.Name, app.Name)
	idHASH := hashes.AppID
	items, err := d.azureVolumeSnapshotList(ctx, rg, volName, idHASH)
	if err != nil {
		return nil, err
	}
	out := make([]*model.VolumeSnapshot, 0, len(items))
	for _, it := range items {
		out = append(out, &model.VolumeSnapshot{
			Name:       it.SnapID,
			VolumeName: volName,
			Size:       int64(it.SizeGiB) << 30,
			Handle:     fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/snapshots/%s", d.AzureSubscriptionId, rg, it.Name),
			CreatedAt:  it.TimeCreated,
			UpdatedAt:  it.TimeCreated,
		})
	}
	return out, nil
}

func (d *driver) VolumeSnapshotCreate(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, diskName string, _ ...model.VolumeSnapshotCreateOption) (*model.VolumeSnapshot, error) {
	if cluster == nil || app == nil {
		return nil, fmt.Errorf("cluster/app nil")
	}
	rg, err := d.volumeResourceGroupName(app)
	if err != nil {
		return nil, err
	}
	hashes := naming.NewHashes(d.ServiceName(), d.ProviderName(), cluster.Name, app.Name)
	idHASH := hashes.AppID
	meta, err := d.azureVolumeSnapshotCreate(ctx, rg, volName, idHASH, diskName)
	if err != nil {
		return nil, err
	}
	return &model.VolumeSnapshot{
		Name:       meta.SnapID,
		VolumeName: volName,
		Size:       int64(meta.SizeGiB) << 30,
		Handle:     fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/snapshots/%s", d.AzureSubscriptionId, rg, meta.Name),
		CreatedAt:  meta.TimeCreated,
		UpdatedAt:  meta.TimeCreated,
	}, nil
}

func (d *driver) VolumeSnapshotDelete(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, snapName string, _ ...model.VolumeSnapshotDeleteOption) error {
	if cluster == nil || app == nil {
		return fmt.Errorf("cluster/app nil")
	}
	rg, err := d.volumeResourceGroupName(app)
	if err != nil {
		return err
	}
	hashes := naming.NewHashes(d.ServiceName(), d.ProviderName(), cluster.Name, app.Name)
	idHASH := hashes.AppID
	return d.azureVolumeSnapshotDelete(ctx, rg, volName, idHASH, snapName)
}

func (d *driver) VolumeSnapshotRestore(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, snapName string, _ ...model.VolumeSnapshotRestoreOption) (*model.VolumeDisk, error) {
	if cluster == nil || app == nil {
		return nil, fmt.Errorf("cluster/app nil")
	}
	// Retrieve AKS principal ID from deployment outputs (per-cluster)
	outputs, err := d.getDeploymentOutputs(ctx, cluster)
	if err != nil {
		return nil, fmt.Errorf("get deployment outputs: %w", err)
	}
	principalID, _ := outputs[OutputAksPrincipalID].(string)

	rg, err := d.volumeResourceGroupName(app)
	if err != nil {
		return nil, err
	}
	hashes := naming.NewHashes(d.ServiceName(), d.ProviderName(), cluster.Name, app.Name)
	idHASH := hashes.AppID
	meta, err := d.azureVolumeDiskCreateFromSnapshot(ctx, rg, volName, idHASH, snapName, principalID)
	if err != nil {
		return nil, err
	}
	return &model.VolumeDisk{
		Name:       meta.DiskID,
		VolumeName: volName,
		Assigned:   meta.Assigned,
		Size:       int64(meta.SizeGiB) << 30,
		Handle:     fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/disks/%s", d.AzureSubscriptionId, rg, meta.Name),
		CreatedAt:  meta.TimeCreated,
		UpdatedAt:  meta.TimeCreated,
	}, nil
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
