package aks

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"github.com/kompox/kompox/domain/model"
	"github.com/kompox/kompox/internal/logging"
)

// Azure Files specific constants
const (
	tagFilesShareName     = "kompox-files-share-name"
	tagFilesShareAssigned = "kompox-files-share-assigned" // true/false
	maxShareNameLength    = 41                            // {vol.name}-{disk.name} with max 16+1+24=41
	defaultFilesSKU       = "Standard_LRS"
	defaultFilesProtocol  = "smb"
)

// volumeBackendFiles implements volumeBackend for Azure Files (Type="files").
type volumeBackendFiles struct {
	driver *driver
}

// newVolumeBackendFiles creates a volumeBackend implementation for Azure Files.
func newVolumeBackendFiles(d *driver) volumeBackend {
	return &volumeBackendFiles{driver: d}
}

// DiskList lists Azure Files shares for a volume (Type="files").
func (vb *volumeBackendFiles) DiskList(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, opts ...model.VolumeDiskListOption) ([]*model.VolumeDisk, error) {
	rg, err := vb.driver.appResourceGroupName(app)
	if err != nil {
		return nil, fmt.Errorf("app RG: %w", err)
	}

	accountName, err := vb.driver.appStorageAccountName(app)
	if err != nil {
		return nil, fmt.Errorf("storage account name: %w", err)
	}

	// Create file shares client
	sharesClient, err := armstorage.NewFileSharesClient(vb.driver.AzureSubscriptionId, vb.driver.TokenCredential, nil)
	if err != nil {
		return nil, fmt.Errorf("new file shares client: %w", err)
	}

	// List shares
	pager := sharesClient.NewListPager(rg, accountName, &armstorage.FileSharesClientListOptions{
		Expand: to.Ptr("metadata"),
	})

	out := []*model.VolumeDisk{}
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			var respErr *azcore.ResponseError
			if errors.As(err, &respErr) && respErr.StatusCode == http.StatusNotFound {
				// Storage account doesn't exist yet
				return []*model.VolumeDisk{}, nil
			}
			return nil, fmt.Errorf("list shares page: %w", err)
		}

		for _, share := range page.Value {
			if share == nil {
				continue
			}

			// Get storage endpoint suffix from environment (default to Azure Public Cloud)
			storageEndpointSuffix := "core.windows.net"
			// TODO: Support other clouds if needed

			volumeDisk, err := vb.newDisk(share, volName, accountName, storageEndpointSuffix)
			if err != nil {
				continue
			}
			if volumeDisk == nil {
				continue // Not for this volume
			}
			out = append(out, volumeDisk)
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

// DiskCreate creates an Azure Files share (Type="files").
func (vb *volumeBackendFiles) DiskCreate(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, diskName string, source string, opts ...model.VolumeDiskCreateOption) (*model.VolumeDisk, error) {
	log := logging.FromContext(ctx)

	if source != "" {
		return nil, fmt.Errorf("source restoration is not supported for Type=files")
	}

	// Get volume from app configuration
	vol, err := app.FindVolume(volName)
	if err != nil {
		return nil, fmt.Errorf("find volume: %w", err)
	}

	options := vol.Options

	// Validate protocol
	protocol := defaultFilesProtocol
	if v, ok := options["protocol"].(string); ok {
		protocol = v
	}
	if protocol != "smb" {
		return nil, fmt.Errorf("only protocol=smb is supported for Type=files, got %q", protocol)
	}

	// Get SKU from options
	sku := defaultFilesSKU
	if v, ok := options["skuName"].(string); ok && v != "" {
		sku = v
	}

	// Ensure storage account exists
	err = vb.driver.ensureStorageAccountCreated(ctx, app, sku)
	if err != nil {
		return nil, fmt.Errorf("ensure storage account: %w", err)
	}

	rg, err := vb.driver.appResourceGroupName(app)
	if err != nil {
		return nil, fmt.Errorf("app RG: %w", err)
	}

	accountName, err := vb.driver.appStorageAccountName(app)
	if err != nil {
		return nil, fmt.Errorf("storage account name: %w", err)
	}

	// Generate share name: {vol.name}-{disk.name}
	shareName := fmt.Sprintf("%s-%s", volName, diskName)
	if len(shareName) > maxShareNameLength {
		return nil, fmt.Errorf("share name %q exceeds max length %d", shareName, maxShareNameLength)
	}

	// Get quota from volume size (in GiB)
	quotaGiB := int32((vol.Size + (1 << 30) - 1) >> 30) // Round up to GiB

	// Override quota if specified in options
	if v, ok := options["quotaGiB"]; ok {
		switch val := v.(type) {
		case int:
			quotaGiB = int32(val)
		case int32:
			quotaGiB = val
		case int64:
			quotaGiB = int32(val)
		case float64:
			quotaGiB = int32(val)
		}
	}

	// Check if this is the first share for this volume
	shares, err := vb.DiskList(ctx, cluster, app, volName)
	if err != nil {
		return nil, fmt.Errorf("list shares: %w", err)
	}

	assignedValue := "false"
	if len(shares) == 0 {
		assignedValue = "true"
	}

	// Check for duplicate disk name
	for _, share := range shares {
		if share.Name == diskName {
			return nil, fmt.Errorf("share %q already exists", diskName)
		}
	}

	// Create file shares client
	sharesClient, err := armstorage.NewFileSharesClient(vb.driver.AzureSubscriptionId, vb.driver.TokenCredential, nil)
	if err != nil {
		return nil, fmt.Errorf("new file shares client: %w", err)
	}

	// Build metadata
	metadata := map[string]*string{
		tagVolumeName:         to.Ptr(volName),
		tagFilesShareName:     to.Ptr(diskName),
		tagFilesShareAssigned: to.Ptr(assignedValue),
	}

	// Create share
	shareProps := armstorage.FileShare{
		FileShareProperties: &armstorage.FileShareProperties{
			ShareQuota: to.Ptr(quotaGiB),
			Metadata:   metadata,
		},
	}

	log.Info(ctx, "Creating Azure Files share", "share", shareName, "quota_gib", quotaGiB)

	_, err = sharesClient.Create(ctx, rg, accountName, shareName, shareProps, nil)
	if err != nil {
		return nil, fmt.Errorf("create share: %w", err)
	}

	// Retrieve the created share to get full details
	getResp, err := sharesClient.Get(ctx, rg, accountName, shareName, &armstorage.FileSharesClientGetOptions{
		Expand: to.Ptr("metadata"),
	})
	if err != nil {
		return nil, fmt.Errorf("get share after create: %w", err)
	}

	// Convert FileShare to FileShareItem for consistent handling
	share := getResp.FileShare
	shareItem := &armstorage.FileShareItem{
		ID:         share.ID,
		Name:       share.Name,
		Type:       share.Type,
		Etag:       share.Etag,
		Properties: share.FileShareProperties,
	}

	storageEndpointSuffix := "core.windows.net"
	volumeDisk, err := vb.newDisk(shareItem, volName, accountName, storageEndpointSuffix)
	if err != nil {
		return nil, fmt.Errorf("create VolumeDisk from share: %w", err)
	}

	return volumeDisk, nil
}

// DiskDelete deletes an Azure Files share (Type="files").
func (vb *volumeBackendFiles) DiskDelete(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, diskName string, opts ...model.VolumeDiskDeleteOption) error {
	rg, err := vb.driver.appResourceGroupName(app)
	if err != nil {
		return fmt.Errorf("app RG: %w", err)
	}

	accountName, err := vb.driver.appStorageAccountName(app)
	if err != nil {
		return fmt.Errorf("storage account name: %w", err)
	}

	shareName := fmt.Sprintf("%s-%s", volName, diskName)

	// Create file shares client
	sharesClient, err := armstorage.NewFileSharesClient(vb.driver.AzureSubscriptionId, vb.driver.TokenCredential, nil)
	if err != nil {
		return fmt.Errorf("new file shares client: %w", err)
	}

	// Delete share
	_, err = sharesClient.Delete(ctx, rg, accountName, shareName, nil)
	if err != nil {
		var respErr *azcore.ResponseError
		if errors.As(err, &respErr) && respErr.StatusCode == http.StatusNotFound {
			return nil // Already deleted
		}
		return fmt.Errorf("delete share: %w", err)
	}

	return nil
}

// DiskAssign assigns an Azure Files share (Type="files").
func (vb *volumeBackendFiles) DiskAssign(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, diskName string, opts ...model.VolumeDiskAssignOption) error {
	rg, err := vb.driver.appResourceGroupName(app)
	if err != nil {
		return fmt.Errorf("app RG: %w", err)
	}

	accountName, err := vb.driver.appStorageAccountName(app)
	if err != nil {
		return fmt.Errorf("storage account name: %w", err)
	}

	// List all shares for this volume
	shares, err := vb.DiskList(ctx, cluster, app, volName)
	if err != nil {
		return fmt.Errorf("list shares: %w", err)
	}

	// Find the target share
	var found bool
	for _, share := range shares {
		if share.Name == diskName {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("share not found: %s", diskName)
	}

	// Create file shares client
	sharesClient, err := armstorage.NewFileSharesClient(vb.driver.AzureSubscriptionId, vb.driver.TokenCredential, nil)
	if err != nil {
		return fmt.Errorf("new file shares client: %w", err)
	}

	// Update assignment for all shares
	for _, share := range shares {
		assigned := share.Name == diskName
		if assigned == share.Assigned {
			continue // No change needed
		}

		shareName := fmt.Sprintf("%s-%s", volName, share.Name)

		// Get current share
		getResp, err := sharesClient.Get(ctx, rg, accountName, shareName, &armstorage.FileSharesClientGetOptions{
			Expand: to.Ptr("metadata"),
		})
		if err != nil {
			return fmt.Errorf("get share %s: %w", shareName, err)
		}

		// Update metadata
		metadata := getResp.FileShare.FileShareProperties.Metadata
		if metadata == nil {
			metadata = map[string]*string{}
		}
		if assigned {
			metadata[tagFilesShareAssigned] = to.Ptr("true")
		} else {
			metadata[tagFilesShareAssigned] = to.Ptr("false")
		}

		// Update share
		updateProps := armstorage.FileShare{
			FileShareProperties: &armstorage.FileShareProperties{
				Metadata: metadata,
			},
		}

		_, err = sharesClient.Update(ctx, rg, accountName, shareName, updateProps, nil)
		if err != nil {
			return fmt.Errorf("update share %s: %w", shareName, err)
		}
	}

	return nil
}

// SnapshotList returns ErrNotSupported (snapshots not supported for Azure Files).
func (vb *volumeBackendFiles) SnapshotList(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, opts ...model.VolumeSnapshotListOption) ([]*model.VolumeSnapshot, error) {
	return nil, model.ErrNotSupported
}

// SnapshotCreate returns ErrNotSupported (snapshots not supported for Azure Files).
func (vb *volumeBackendFiles) SnapshotCreate(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, snapName string, source string, opts ...model.VolumeSnapshotCreateOption) (*model.VolumeSnapshot, error) {
	return nil, model.ErrNotSupported
}

// SnapshotDelete returns ErrNotSupported (snapshots not supported for Azure Files).
func (vb *volumeBackendFiles) SnapshotDelete(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, snapName string, opts ...model.VolumeSnapshotDeleteOption) error {
	return model.ErrNotSupported
}

// Class returns Azure Files provisioning parameters (Type="files").
func (vb *volumeBackendFiles) Class(ctx context.Context, cluster *model.Cluster, app *model.App, vol model.AppVolume) (model.VolumeClass, error) {
	attrs := map[string]string{
		"protocol": "smb", // Fixed for now, NFS in future
	}
	// Add skuName if specified in options
	if vol.Options != nil {
		if v, ok := vol.Options["skuName"].(string); ok && v != "" {
			attrs["skuName"] = v
		}
	}
	return model.VolumeClass{
		StorageClassName: "azurefile-csi",
		CSIDriver:        "file.csi.azure.com",
		FSType:           "",
		Attributes:       attrs,
		AccessModes:      []string{"ReadWriteMany"},
		ReclaimPolicy:    "Retain",
		VolumeMode:       "Filesystem",
	}, nil
}

// newDisk creates a model.VolumeDisk from an Azure Files share.
func (vb *volumeBackendFiles) newDisk(share *armstorage.FileShareItem, volName, accountName, storageEndpointSuffix string) (*model.VolumeDisk, error) {
	if share == nil || share.Name == nil || share.Properties == nil {
		return nil, fmt.Errorf("share is nil or missing required fields")
	}

	metadata := share.Properties.Metadata
	if metadata == nil {
		return nil, fmt.Errorf("share missing metadata")
	}

	// Extract disk name from metadata
	diskName := ""
	if v, ok := metadata[tagFilesShareName]; !ok || v == nil || *v == "" {
		return nil, fmt.Errorf("share missing required metadata: %s", tagFilesShareName)
	} else {
		diskName = *v
	}

	// Extract volume name from metadata
	shareVolName := ""
	if v, ok := metadata[tagVolumeName]; !ok || v == nil || *v == "" {
		return nil, fmt.Errorf("share missing required metadata: %s", tagVolumeName)
	} else {
		shareVolName = *v
	}

	// Verify volume name matches
	if shareVolName != volName {
		return nil, nil // Skip this share, it belongs to a different volume
	}

	// Extract assigned status
	assigned := false
	if v, ok := metadata[tagFilesShareAssigned]; ok && v != nil && strings.EqualFold(*v, "true") {
		assigned = true
	}

	// Extract quota (size in GiB, convert to bytes)
	var size int64
	if share.Properties.ShareQuota != nil {
		size = int64(*share.Properties.ShareQuota) << 30 // GiB to bytes
	}

	// Extract creation time
	var created time.Time
	if share.Properties.LastModifiedTime != nil {
		created = *share.Properties.LastModifiedTime
	}

	// Generate Handle URI: smb://{account}.file.{suffix}/{share}
	handle := fmt.Sprintf("smb://%s.file.%s/%s", accountName, storageEndpointSuffix, *share.Name)

	// Build options from share properties
	options := map[string]any{
		"protocol": defaultFilesProtocol,
	}
	if share.Properties.ShareQuota != nil {
		options["quotaGiB"] = *share.Properties.ShareQuota
	}

	return &model.VolumeDisk{
		Name:       diskName,
		VolumeName: volName,
		Assigned:   assigned,
		Size:       size,
		Zone:       "", // Azure Files is regional
		Options:    options,
		Handle:     handle,
		CreatedAt:  created,
		UpdatedAt:  created,
	}, nil
}
