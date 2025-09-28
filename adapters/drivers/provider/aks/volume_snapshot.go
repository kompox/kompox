package aks

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
	"github.com/kompox/kompox/domain/model"
	"github.com/kompox/kompox/internal/naming"
)

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

func (d *driver) VolumeSnapshotCreate(ctx context.Context, cluster *model.Cluster, app *model.App, volName string, snapName string, source string, _ ...model.VolumeSnapshotCreateOption) (*model.VolumeSnapshot, error) {
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
	snapName = strings.TrimSpace(snapName)
	if snapName == "" {
		if snapName, err = naming.NewCompactID(); err != nil {
			return nil, fmt.Errorf("compact id: %w", err)
		}
	}
	source = strings.TrimSpace(source)
	var sourceResourceID string
	if source == "" {
		disks, lerr := d.VolumeDiskList(ctx, cluster, app, volName)
		if lerr != nil {
			return nil, fmt.Errorf("list disks: %w", lerr)
		}
		var assigned []*model.VolumeDisk
		for _, disk := range disks {
			if disk.Assigned {
				assigned = append(assigned, disk)
			}
		}
		switch len(assigned) {
		case 0:
			return nil, fmt.Errorf("no assigned disk available for volume %s", volName)
		case 1:
			sourceResourceID, err = d.resolveKompoxDiskResourceID(ctx, app, volName, assigned[0].Name)
			if err != nil {
				return nil, fmt.Errorf("resolve assigned disk %q: %w", assigned[0].Name, err)
			}
		default:
			return nil, fmt.Errorf("multiple assigned disks found for volume %s", volName)
		}
	} else {
		sourceResourceID, err = d.resolveSourceDiskResourceID(ctx, app, volName, source)
		if err != nil {
			return nil, fmt.Errorf("resolve source %q: %w", source, err)
		}
	}

	// Determine if the source Resource ID is a snapshot to choose proper createOption.
	// Use Azure SDK's Resource ID parser and fail fast if it cannot be parsed.
	rid, err := arm.ParseResourceID(sourceResourceID)
	if err != nil {
		return nil, fmt.Errorf("parse source resource ID: %w", err)
	}
	createOption := armcompute.DiskCreateOptionCopy
	if strings.EqualFold(rid.ResourceType.Namespace, "Microsoft.Compute") && strings.EqualFold(rid.ResourceType.Type, "snapshots") {
		createOption = armcompute.DiskCreateOptionCopyStart
	}

	// New snapshot resource name
	snapResourceName, err := d.appSnapshotName(app, volName, snapName)
	if err != nil {
		return nil, fmt.Errorf("generate snapshot resource name: %w", err)
	}

	// New snapshot tags
	tags := d.appResourceTags(app.Name)
	tags[tagVolumeName] = to.Ptr(volName)
	tags[tagSnapshotName] = to.Ptr(snapName)

	snap := armcompute.Snapshot{
		Location: to.Ptr(d.AzureLocation),
		Tags:     tags,
		Properties: &armcompute.SnapshotProperties{
			CreationData: &armcompute.CreationData{
				CreateOption:     to.Ptr(createOption),
				SourceResourceID: to.Ptr(sourceResourceID),
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
