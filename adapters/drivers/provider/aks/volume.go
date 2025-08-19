package aks

import (
	"context"
	"crypto/rand"
	"crypto/sha1"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/oklog/ulid/v2"
	"github.com/yaegashi/kompoxops/domain/model"
)

// Volume instance tag keys
const (
	tagVolumeKey          = "kompox-volume"                   // logical volume key value: kompox-volName-idHASH
	tagVolumeInstanceName = "kompox-volume-instance-name"     // ulid
	tagVolumeAssigned     = "kompox-volume-instance-assigned" // true/false
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
func (d *driver) ensureResourceGroup(ctx context.Context, resourceGroupName string) error {
	groupsClient, err := armresources.NewResourceGroupsClient(d.AzureSubscriptionId, d.TokenCredential, nil)
	if err != nil {
		return err
	}
	_, err = groupsClient.CreateOrUpdate(ctx, resourceGroupName, armresources.ResourceGroup{Location: to.Ptr(d.AzureLocation)}, nil)
	return err
}

// VolumeInstanceCreate creates an Azure Disk for logical volume.
func (d *driver) azureVolumeInstanceCreate(ctx context.Context, resourceGroupName, volName, idHASH string, sizeGiB int32) (*volumeInstanceMeta, error) {
	if resourceGroupName == "" {
		return nil, errors.New("AZURE_RESOURCE_GROUP_NAME required")
	}
	if sizeGiB < 1 {
		return nil, fmt.Errorf("sizeGiB must be >=1")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	if err := d.ensureResourceGroup(ctx, resourceGroupName); err != nil {
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

	disk := armcompute.Disk{
		Location: to.Ptr(d.AzureLocation),
		Tags: map[string]*string{
			tagVolumeKey:          to.Ptr(lvKey),
			tagVolumeInstanceName: to.Ptr(ulidStr),
			tagVolumeAssigned:     to.Ptr("false"),
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
	meta := &volumeInstanceMeta{Name: diskName, VolInst: ulidStr, SizeGiB: sizeGiB, Assigned: false, TimeCreated: time.Now().UTC()}
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

// === Driver interface adapter methods ===

// AppVolumeInstanceList adapts VolumeInstanceList to Driver interface returning model.AppVolumeInstance slice.
// === Spec-compliant Driver interface methods ===

// deriveIDHash computes idHASH (cluster independent) same logic as compose_converter: sha1(service:provider:app) first 6 hex.
func deriveIDHash(serviceName, providerName, appName string) string {
	base := fmt.Sprintf("%s:%s:%s", serviceName, providerName, appName)
	sum := sha1.Sum([]byte(base))
	return fmt.Sprintf("%x", sum)[:6]
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
func (d *driver) VolumeInstanceList(ctx context.Context, cluster *model.Cluster, app *model.App, volName string) ([]*model.AppVolumeInstance, error) {
	if cluster == nil || app == nil {
		return nil, fmt.Errorf("cluster/app nil")
	}
	rg := ""
	if app.Settings != nil {
		rg = app.Settings["AZURE_RESOURCE_GROUP_NAME"]
	}
	if rg == "" {
		return nil, fmt.Errorf("app setting AZURE_RESOURCE_GROUP_NAME missing")
	}
	idHASH := deriveIDHash(d.ServiceName(), d.ProviderName(), app.Name)
	items, err := d.azureVolumeInstanceList(ctx, rg, volName, idHASH)
	if err != nil {
		return nil, err
	}
	out := make([]*model.AppVolumeInstance, 0, len(items))
	for _, it := range items {
		out = append(out, &model.AppVolumeInstance{
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
func (d *driver) VolumeInstanceCreate(ctx context.Context, cluster *model.Cluster, app *model.App, volName string) (*model.AppVolumeInstance, error) {
	if cluster == nil || app == nil {
		return nil, fmt.Errorf("cluster/app nil")
	}
	rg := ""
	if app.Settings != nil {
		rg = app.Settings["AZURE_RESOURCE_GROUP_NAME"]
	}
	if rg == "" {
		return nil, fmt.Errorf("app setting AZURE_RESOURCE_GROUP_NAME missing")
	}
	sizeBytes, ok := volumeDefSize(app, volName)
	if !ok {
		return nil, fmt.Errorf("volume definition not found: %s", volName)
	}
	if sizeBytes < 1 {
		sizeBytes = 1 << 30
	}
	sizeGiB := int32((sizeBytes + (1 << 30) - 1) >> 30)
	idHASH := deriveIDHash(d.ServiceName(), d.ProviderName(), app.Name)
	meta, err := d.azureVolumeInstanceCreate(ctx, rg, volName, idHASH, sizeGiB)
	if err != nil {
		return nil, err
	}
	return &model.AppVolumeInstance{
		Name:       meta.VolInst,
		VolumeName: volName,
		Assigned:   false,
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
	rg := ""
	if app.Settings != nil {
		rg = app.Settings["AZURE_RESOURCE_GROUP_NAME"]
	}
	if rg == "" {
		return fmt.Errorf("app setting AZURE_RESOURCE_GROUP_NAME missing")
	}
	idHASH := deriveIDHash(d.ServiceName(), d.ProviderName(), app.Name)
	return d.azureVolumeInstanceAssign(ctx, rg, volName, idHASH, volInstName)
}

// VolumeInstanceDelete implements spec method.
func (d *driver) VolumeInstanceDelete(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, volInstName string) error {
	if cluster == nil || app == nil {
		return fmt.Errorf("cluster/app nil")
	}
	rg := ""
	if app.Settings != nil {
		rg = app.Settings["AZURE_RESOURCE_GROUP_NAME"]
	}
	if rg == "" {
		return fmt.Errorf("app setting AZURE_RESOURCE_GROUP_NAME missing")
	}
	idHASH := deriveIDHash(d.ServiceName(), d.ProviderName(), app.Name)
	return d.azureVolumeInstanceDelete(ctx, rg, volName, idHASH, volInstName)
}
