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

// Volume instance tag keys
const (
	tagVolumeKey          = "kompox-volume"                   // logical volume key value: kompox-volName-idHASH
	tagVolumeInstanceName = "kompox-volume-instance-name"     // ulid
	tagVolumeAssigned     = "kompox-volume-instance-assigned" // true/false
)

// Built-in role definition IDs used by this driver
const (
	// Contributor role definition GUID (tenant-agnostic)
	contributorRoleDefinitionID = "b24988ac-6180-42a0-ab88-20f7382dd24c"
)

// volumeInstanceMeta represents returned metadata for a volume insta
// (Internal structure; adapt to domain model when wiring into usecases.)
type volumeInstanceMeta struct {
	Name        string // Azure Disk name
	VolInst     string // ULID
	SizeGiB     int32
	Assigned    bool
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

// logicalVolumeKey returns kompox-volName-idHASH
func logicalVolumeKey(volName, idHASH string) string {
	return fmt.Sprintf("kompox-%s-%s", volName, idHASH)
}

// VolumeInstanceList lists Azure Managed Disks for a logical volume.
func (d *driver) azureVolumeInstanceList(ctx context.Context, resourceGroupName, volName, idHASH string) ([]volumeInstanceMeta, error) {
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
	out := []volumeInstanceMeta{}
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "could not be found") { // RG not found
				return []volumeInstanceMeta{}, nil
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
			volInst := strings.TrimPrefix(name, prefix)
			size := int32(0)
			if disk.Properties.DiskSizeGB != nil {
				size = *disk.Properties.DiskSizeGB
			}
			assigned := false
			if v, ok := tags[tagVolumeAssigned]; ok && v != nil && strings.EqualFold(*v, "true") {
				assigned = true
			}
			created := time.Time{}
			if disk.Properties.TimeCreated != nil {
				created = *disk.Properties.TimeCreated
			}
			out = append(out, volumeInstanceMeta{Name: name, VolInst: volInst, SizeGiB: size, Assigned: assigned, TimeCreated: created})
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

// VolumeInstanceCreate creates an Azure Disk for logical volume.
func (d *driver) azureVolumeInstanceCreate(ctx context.Context, resourceGroupName, volName, idHASH string, sizeGiB int32, principalID string) (*volumeInstanceMeta, error) {
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

	// If this is the first volume instance for the logical volume, mark it assigned=true.
	initialAssigned := false
	if list, err := d.azureVolumeInstanceList(ctx, resourceGroupName, volName, idHASH); err == nil {
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
			tagVolumeKey:          to.Ptr(lvKey),
			tagVolumeInstanceName: to.Ptr(ulidStr),
			tagVolumeAssigned:     to.Ptr(assignedVal),
			"managed-by":          to.Ptr("kompox"),
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
	meta := &volumeInstanceMeta{Name: diskName, VolInst: ulidStr, SizeGiB: sizeGiB, Assigned: assignedVal == "true", TimeCreated: time.Now().UTC()}
	if res.Properties != nil && res.Properties.TimeCreated != nil {
		meta.TimeCreated = *res.Properties.TimeCreated
	}
	return meta, nil
}

// VolumeInstanceAssign marks one disk assigned=true and others false using tag update with concurrency control.
func (d *driver) azureVolumeInstanceAssign(ctx context.Context, resourceGroupName, volName, idHASH, volInstName string) error {
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
	list, err := d.azureVolumeInstanceList(ctx, resourceGroupName, volName, idHASH)
	if err != nil {
		return err
	}
	var target *volumeInstanceMeta
	for i := range list {
		if list[i].VolInst == volInstName {
			target = &list[i]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("volume instance not found: %s", volInstName)
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
		tags[tagVolumeInstanceName] = to.Ptr(item.VolInst)
		assigned := strings.EqualFold(item.VolInst, volInstName)
		// Idempotency check
		prevAssigned := false
		if v, ok := getRes.Tags[tagVolumeAssigned]; ok && v != nil && strings.EqualFold(*v, "true") {
			prevAssigned = true
		}
		if assigned && prevAssigned {
			continue
		}
		val := "false"
		if assigned {
			val = "true"
		}
		tags[tagVolumeAssigned] = to.Ptr(val)
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

// VolumeInstanceDelete deletes a disk (idempotent).
func (d *driver) azureVolumeInstanceDelete(ctx context.Context, resourceGroupName, volName, idHASH, volInstName string) error {
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
	diskName := buildDiskName(lvKey, volInstName)
	poller, err := disksClient.BeginDelete(ctx, resourceGroupName, diskName, nil)
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

// volumeDefSize returns defined size for volume name from app.Volumes.
func volumeDefSize(app *model.App, volName string) (int64, bool) {
	for _, v := range app.Volumes {
		if v.Name == volName {
			return v.Size, true
		}
	}
	return 0, false
}

// VolumeInstanceList implements spec method.
func (d *driver) VolumeInstanceList(ctx context.Context, cluster *model.Cluster, app *model.App, volName string) ([]*model.VolumeInstance, error) {
	if cluster == nil || app == nil {
		return nil, fmt.Errorf("cluster/app nil")
	}
	rg, err := d.volumeResourceGroupName(app)
	if err != nil {
		return nil, err
	}
	hashes := naming.NewHashes(d.ServiceName(), d.ProviderName(), cluster.Name, app.Name)
	idHASH := hashes.AppID
	items, err := d.azureVolumeInstanceList(ctx, rg, volName, idHASH)
	if err != nil {
		return nil, err
	}
	out := make([]*model.VolumeInstance, 0, len(items))
	for _, it := range items {
		out = append(out, &model.VolumeInstance{
			Name:       it.VolInst,
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

// VolumeInstanceCreate implements spec method.
func (d *driver) VolumeInstanceCreate(ctx context.Context, cluster *model.Cluster, app *model.App, volName string) (*model.VolumeInstance, error) {
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
	meta, err := d.azureVolumeInstanceCreate(ctx, rg, volName, idHASH, sizeGiB, principalID)
	if err != nil {
		return nil, err
	}
	return &model.VolumeInstance{
		Name:       meta.VolInst,
		VolumeName: volName,
		Assigned:   meta.Assigned,
		Size:       int64(meta.SizeGiB) << 30,
		Handle:     fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/disks/%s", d.AzureSubscriptionId, rg, meta.Name),
		CreatedAt:  meta.TimeCreated,
		UpdatedAt:  meta.TimeCreated,
	}, nil
}

// VolumeInstanceAssign implements spec method.
func (d *driver) VolumeInstanceAssign(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, volInstName string) error {
	if cluster == nil || app == nil {
		return fmt.Errorf("cluster/app nil")
	}
	rg, err := d.volumeResourceGroupName(app)
	if err != nil {
		return err
	}
	hashes := naming.NewHashes(d.ServiceName(), d.ProviderName(), cluster.Name, app.Name)
	idHASH := hashes.AppID
	return d.azureVolumeInstanceAssign(ctx, rg, volName, idHASH, volInstName)
}

// VolumeInstanceDelete implements spec method.
func (d *driver) VolumeInstanceDelete(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, volInstName string) error {
	if cluster == nil || app == nil {
		return fmt.Errorf("cluster/app nil")
	}
	rg, err := d.volumeResourceGroupName(app)
	if err != nil {
		return err
	}
	hashes := naming.NewHashes(d.ServiceName(), d.ProviderName(), cluster.Name, app.Name)
	idHASH := hashes.AppID
	return d.azureVolumeInstanceDelete(ctx, rg, volName, idHASH, volInstName)
}

// Snapshot operations for AKS (skeleton placeholders for now)
func (d *driver) VolumeSnapshotList(ctx context.Context, cluster *model.Cluster, app *model.App, volName string) ([]*model.VolumeSnapshot, error) {
	return nil, fmt.Errorf("VolumeSnapshotList not implemented for AKS driver")
}

func (d *driver) VolumeSnapshotCreate(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, volInstName string) (*model.VolumeSnapshot, error) {
	return nil, fmt.Errorf("VolumeSnapshotCreate not implemented for AKS driver")
}

func (d *driver) VolumeSnapshotDelete(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, snapName string) error {
	return fmt.Errorf("VolumeSnapshotDelete not implemented for AKS driver")
}

func (d *driver) VolumeSnapshotRestore(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, snapName string) (*model.VolumeInstance, error) {
	return nil, fmt.Errorf("VolumeSnapshotRestore not implemented for AKS driver")
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
